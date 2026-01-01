package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"picture-this/internal/db"
	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func (s *Server) handleAdminPromptsView(c *gin.Context) {
	page, perPage := parsePagination(c, promptLibraryDefaultPerPage, promptLibraryMaxPerPage)
	data := s.loadPromptLibraryData(page, perPage)
	if data.Error == "" {
		if msg := strings.TrimSpace(c.Query("error")); msg != "" {
			data.Error = msg
		}
	}
	data.Notice = strings.TrimSpace(c.Query("notice"))
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleAdminPromptCreate(c *gin.Context) {
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	text, err := validatePrompt(c.PostForm("text"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"))
		return
	}
	joke, err := validateJoke(c.PostForm("joke"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"))
		return
	}

	entry := db.PromptLibrary{Text: text, Joke: joke}
	if err := s.db.Create(&entry).Error; err != nil {
		s.renderPromptLibraryError(c, "Failed to save prompt (it may already exist).", text, joke)
		return
	}

	notice := url.QueryEscape("Prompt added.")
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) handleAdminPromptGenerate(c *gin.Context) {
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	instructions := strings.TrimSpace(c.PostForm("instructions"))
	if instructions == "" {
		s.renderPromptLibraryGenerateError(c, "Please provide guidance for the prompt generation.", instructions)
		return
	}
	prompts, err := s.generatePromptsFromOpenAI(c.Request.Context(), instructions)
	if err != nil {
		s.renderPromptLibraryGenerateError(c, err.Error(), instructions)
		return
	}

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
		s.renderPromptLibraryGenerateError(c, "No valid prompts were generated. Try again.", instructions)
		return
	}
	result := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entries)
	if result.Error != nil {
		s.renderPromptLibraryGenerateError(c, "Failed to save generated prompts.", instructions)
		return
	}

	added := result.RowsAffected
	notice := url.QueryEscape(fmt.Sprintf("Added %d prompt(s) to the library.", added))
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) handleAdminPromptUpdate(c *gin.Context) {
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		s.renderPromptLibraryError(c, "Invalid prompt id.", "", "")
		return
	}
	text, err := validatePrompt(c.PostForm("text"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"))
		return
	}
	joke, err := validateJoke(c.PostForm("joke"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"))
		return
	}

	var entry db.PromptLibrary
	if err := s.db.First(&entry, uint(id)).Error; err != nil {
		s.renderPromptLibraryError(c, "Prompt not found.", "", "")
		return
	}
	entry.Text = text
	if err := s.db.Model(&entry).Updates(map[string]any{
		"Text": text,
		"Joke": joke,
	}).Error; err != nil {
		s.renderPromptLibraryError(c, "Failed to update prompt (it may already exist).", text, joke)
		return
	}

	notice := url.QueryEscape("Prompt updated.")
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) handleAdminPromptDelete(c *gin.Context) {
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		s.renderPromptLibraryError(c, "Invalid prompt id.", "", "")
		return
	}
	result := s.db.Delete(&db.PromptLibrary{}, uint(id))
	if result.Error != nil {
		s.renderPromptLibraryError(c, "Failed to delete prompt.", "", "")
		return
	}
	if result.RowsAffected == 0 {
		s.renderPromptLibraryError(c, "Prompt not found.", "", "")
		return
	}

	notice := url.QueryEscape("Prompt deleted.")
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) loadPromptLibraryData(page, perPage int) web.AdminPromptLibraryData {
	data := web.AdminPromptLibraryData{}
	if s.db == nil {
		data.Error = "Database not configured."
		return data
	}
	var total int64
	if err := s.db.Model(&db.PromptLibrary{}).Count(&total).Error; err != nil {
		data.Error = "Failed to load prompt library."
		return data
	}
	pagination := buildPaginationData("/admin/prompts", page, perPage, total)
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := s.db.Order("text asc, id asc").Limit(pagination.PerPage).Offset(offset).Find(&data.Prompts).Error; err != nil {
		data.Error = "Failed to load prompt library."
	}
	data.Pagination = pagination
	return data
}

func (s *Server) renderPromptLibraryError(c *gin.Context, message, text, joke string) {
	page, perPage := parsePagination(c, promptLibraryDefaultPerPage, promptLibraryMaxPerPage)
	data := s.loadPromptLibraryData(page, perPage)
	data.Error = message
	data.DraftText = text
	data.DraftJoke = joke
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) renderPromptLibraryGenerateError(c *gin.Context, message, instructions string) {
	page, perPage := parsePagination(c, promptLibraryDefaultPerPage, promptLibraryMaxPerPage)
	data := s.loadPromptLibraryData(page, perPage)
	data.Error = message
	data.GenerateInstructions = instructions
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

const (
	promptLibraryDefaultPerPage = 20
	promptLibraryMaxPerPage     = 200
)
