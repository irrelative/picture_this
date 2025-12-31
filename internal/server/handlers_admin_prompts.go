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
	data := s.loadPromptLibraryData()
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
		data := s.loadPromptLibraryData()
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	text, err := validatePrompt(c.PostForm("text"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"))
		return
	}

	entry := db.PromptLibrary{Text: text}
	if err := s.db.Create(&entry).Error; err != nil {
		s.renderPromptLibraryError(c, "Failed to save prompt (it may already exist).", text)
		return
	}

	notice := url.QueryEscape("Prompt added.")
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) handleAdminPromptGenerate(c *gin.Context) {
	if s.db == nil {
		data := s.loadPromptLibraryData()
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
		clean, err := validatePrompt(prompt)
		if err != nil {
			continue
		}
		entries = append(entries, db.PromptLibrary{Text: clean})
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
		data := s.loadPromptLibraryData()
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		s.renderPromptLibraryError(c, "Invalid prompt id.", "")
		return
	}
	text, err := validatePrompt(c.PostForm("text"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"))
		return
	}

	var entry db.PromptLibrary
	if err := s.db.First(&entry, uint(id)).Error; err != nil {
		s.renderPromptLibraryError(c, "Prompt not found.", "")
		return
	}
	entry.Text = text
	if err := s.db.Model(&entry).Updates(map[string]any{
		"Text": text,
	}).Error; err != nil {
		s.renderPromptLibraryError(c, "Failed to update prompt (it may already exist).", text)
		return
	}

	notice := url.QueryEscape("Prompt updated.")
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) handleAdminPromptDelete(c *gin.Context) {
	if s.db == nil {
		data := s.loadPromptLibraryData()
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		s.renderPromptLibraryError(c, "Invalid prompt id.", "")
		return
	}
	result := s.db.Delete(&db.PromptLibrary{}, uint(id))
	if result.Error != nil {
		s.renderPromptLibraryError(c, "Failed to delete prompt.", "")
		return
	}
	if result.RowsAffected == 0 {
		s.renderPromptLibraryError(c, "Prompt not found.", "")
		return
	}

	notice := url.QueryEscape("Prompt deleted.")
	c.Redirect(http.StatusFound, "/admin/prompts?notice="+notice)
}

func (s *Server) loadPromptLibraryData() web.AdminPromptLibraryData {
	data := web.AdminPromptLibraryData{}
	if s.db == nil {
		data.Error = "Database not configured."
		return data
	}
	if err := s.db.Order("text asc, id asc").Find(&data.Prompts).Error; err != nil {
		data.Error = "Failed to load prompt library."
	}
	return data
}

func (s *Server) renderPromptLibraryError(c *gin.Context, message, text string) {
	data := s.loadPromptLibraryData()
	data.Error = message
	data.DraftText = text
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) renderPromptLibraryGenerateError(c *gin.Context, message, instructions string) {
	data := s.loadPromptLibraryData()
	data.Error = message
	data.GenerateInstructions = instructions
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}
