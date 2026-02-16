package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"picture-this/internal/db"
	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

const (
	promptGenerateJobStateRunning   = "running"
	promptGenerateJobStateCompleted = "completed"
	promptGenerateJobStateFailed    = "failed"

	promptGenerateJobTimeout = 2 * time.Minute
	promptGenerateJobTTL     = 20 * time.Minute
)

type promptGenerateJob struct {
	ID          string
	SearchQuery string
	State       string
	Message     string
	Error       string
	Notice      string
	Current     int
	Total       int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Server) handleAdminPromptGenerateJobCreate(c *gin.Context) {
	searchQuery := normalizePromptLibraryQuery(c.PostForm("q"))
	instructions := strings.TrimSpace(c.PostForm("instructions"))
	if instructions == "" {
		c.Header("Cache-Control", "no-store")
		templ.Handler(web.AdminPromptGenerateJob(web.AdminPromptGenerateJobData{
			Error: "Please provide guidance for the prompt generation.",
		})).ServeHTTP(c.Writer, c.Request)
		return
	}

	if s.db == nil {
		c.Header("Cache-Control", "no-store")
		templ.Handler(web.AdminPromptGenerateJob(web.AdminPromptGenerateJobData{
			Error: "Database not configured.",
		})).ServeHTTP(c.Writer, c.Request)
		return
	}

	job := s.createPromptGenerateJob(searchQuery)
	go s.runPromptGenerateJob(job.ID, instructions)

	c.Header("Cache-Control", "no-store")
	templ.Handler(web.AdminPromptGenerateJob(job.toViewData())).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleAdminPromptGenerateJobPoll(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("jobID"))
	if jobID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	job, ok := s.getPromptGenerateJob(jobID)
	if !ok {
		c.Status(http.StatusNotFound)
		c.Header("Cache-Control", "no-store")
		templ.Handler(web.AdminPromptGenerateJob(web.AdminPromptGenerateJobData{
			JobID: jobID,
			State: promptGenerateJobStateFailed,
			Error: "Prompt generation job not found. Start a new generation run.",
		})).ServeHTTP(c.Writer, c.Request)
		return
	}

	if job.State == promptGenerateJobStateCompleted {
		c.Header("HX-Redirect", promptLibraryRedirectURL(job.SearchQuery, job.Notice))
		c.Status(http.StatusNoContent)
		return
	}

	c.Header("Cache-Control", "no-store")
	templ.Handler(web.AdminPromptGenerateJob(job.toViewData())).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) runPromptGenerateJob(jobID, instructions string) {
	ctx, cancel := context.WithTimeout(context.Background(), promptGenerateJobTimeout)
	defer cancel()

	s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
		job.State = promptGenerateJobStateRunning
		job.Total = 4
		job.Current = 0
		job.Message = "Requesting prompts from OpenAI..."
		job.Error = ""
		job.Notice = ""
	})

	prompts, err := s.generatePromptsFromOpenAI(ctx, instructions)
	if err != nil {
		s.failPromptGenerateJob(jobID, err.Error())
		return
	}

	s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
		job.Current = 1
		job.Message = "Normalizing generated prompts..."
	})

	entries := make([]db.PromptLibrary, 0, len(prompts))
	for _, prompt := range prompts {
		clean, err := validatePrompt(prompt.Text)
		if err != nil {
			continue
		}
		joke, err := validateJoke(prompt.Joke)
		if err != nil {
			joke = ""
		}
		entries = append(entries, db.PromptLibrary{Text: clean, Joke: joke})
	}
	if len(entries) == 0 {
		s.failPromptGenerateJob(jobID, "No valid prompts were generated. Try again.")
		return
	}

	s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
		job.Current = 2
		job.Message = "Checking for similar prompts..."
	})
	filteredEntries, embeddingByText, err := s.filterGeneratedPromptEntries(ctx, entries)
	if err != nil {
		s.failPromptGenerateJob(jobID, "Failed to compare generated prompts with existing prompts.")
		return
	}
	if len(filteredEntries) == 0 {
		s.failPromptGenerateJob(jobID, "All generated prompts were too similar to existing prompts. Try different guidance.")
		return
	}

	totalToPersist := len(filteredEntries)
	totalSteps := 4 + totalToPersist
	s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
		job.Total = totalSteps
		job.Current = 3
		job.Message = fmt.Sprintf("Saving prompts (0/%d)...", totalToPersist)
	})

	added, err := s.insertPromptLibraryEntriesWithProgress(ctx, filteredEntries, embeddingByText, func(processed, total int) {
		s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
			job.Total = 4 + total
			job.Current = 3 + processed
			job.Message = fmt.Sprintf("Saving prompts (%d/%d)...", processed, total)
		})
	})
	if err != nil {
		s.failPromptGenerateJob(jobID, "Failed to save generated prompts.")
		return
	}
	if added == 0 {
		s.failPromptGenerateJob(jobID, "Generated prompts already exist or were too similar.")
		return
	}

	s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
		job.State = promptGenerateJobStateCompleted
		if job.Total < 1 {
			job.Total = 1
		}
		job.Current = job.Total
		job.Notice = fmt.Sprintf("Added %d prompt(s) to the library.", added)
		job.Message = "Prompt generation complete."
		job.Error = ""
	})
}

func (s *Server) createPromptGenerateJob(searchQuery string) promptGenerateJob {
	now := time.Now().UTC()
	job := &promptGenerateJob{
		ID:          strconv.FormatUint(atomic.AddUint64(&s.nextPromptJobID, 1), 10),
		SearchQuery: searchQuery,
		State:       promptGenerateJobStateRunning,
		Message:     "Queued...",
		Current:     0,
		Total:       4,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	s.prunePromptGenerateJobsLocked(now)
	s.promptJobs[job.ID] = job
	return *job
}

func (s *Server) getPromptGenerateJob(jobID string) (promptGenerateJob, bool) {
	now := time.Now().UTC()
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	s.prunePromptGenerateJobsLocked(now)
	job, ok := s.promptJobs[jobID]
	if !ok {
		return promptGenerateJob{}, false
	}
	return *job, true
}

func (s *Server) updatePromptGenerateJob(jobID string, mutate func(*promptGenerateJob)) {
	now := time.Now().UTC()
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	job, ok := s.promptJobs[jobID]
	if !ok {
		return
	}
	mutate(job)
	job.UpdatedAt = now
}

func (s *Server) failPromptGenerateJob(jobID, message string) {
	s.updatePromptGenerateJob(jobID, func(job *promptGenerateJob) {
		job.State = promptGenerateJobStateFailed
		job.Error = strings.TrimSpace(message)
		if job.Error == "" {
			job.Error = "Prompt generation failed."
		}
		job.Notice = ""
		job.Message = ""
		if job.Total < 1 {
			job.Total = 1
		}
		if job.Current < 0 {
			job.Current = 0
		}
		if job.Current > job.Total {
			job.Current = job.Total
		}
	})
}

func (s *Server) prunePromptGenerateJobsLocked(now time.Time) {
	for id, job := range s.promptJobs {
		if job.State == promptGenerateJobStateRunning {
			continue
		}
		if now.Sub(job.UpdatedAt) > promptGenerateJobTTL {
			delete(s.promptJobs, id)
		}
	}
}

func (job promptGenerateJob) toViewData() web.AdminPromptGenerateJobData {
	total := job.Total
	if total < 1 {
		total = 1
	}
	current := job.Current
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	percent := (current * 100) / total
	pollPath := "/admin/prompts/generate-jobs/" + url.PathEscape(job.ID)
	return web.AdminPromptGenerateJobData{
		JobID:       job.ID,
		SearchQuery: job.SearchQuery,
		State:       job.State,
		Message:     job.Message,
		Error:       job.Error,
		Notice:      job.Notice,
		Current:     current,
		Total:       total,
		Percent:     percent,
		PollPath:    pollPath,
	}
}
