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
)

func (s *Server) handleAdminPromptsView(c *gin.Context) {
	searchQuery := normalizePromptLibraryQuery(c.Query("q"))
	page, perPage := parsePagination(c, promptLibraryDefaultPerPage, promptLibraryMaxPerPage)
	data := s.loadPromptLibraryData(page, perPage, searchQuery)
	if data.Error == "" {
		if msg := strings.TrimSpace(c.Query("error")); msg != "" {
			data.Error = msg
		}
	}
	data.Notice = strings.TrimSpace(c.Query("notice"))
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleAdminPromptCreate(c *gin.Context) {
	searchQuery := normalizePromptLibraryQuery(c.PostForm("q"))
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage, searchQuery)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	text, err := validatePrompt(c.PostForm("text"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"), searchQuery)
		return
	}
	joke, err := validateJoke(c.PostForm("joke"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"), searchQuery)
		return
	}

	entry := db.PromptLibrary{Text: text, Joke: joke}
	if err := s.db.Create(&entry).Error; err != nil {
		s.renderPromptLibraryError(c, "Failed to save prompt (it may already exist).", text, joke, searchQuery)
		return
	}
	if err := s.ensurePromptLibraryEmbedding(c.Request.Context(), entry.ID, entry.Text); err != nil {
		s.renderPromptLibraryError(c, "Prompt saved, but embedding generation failed.", text, joke, searchQuery)
		return
	}

	c.Redirect(http.StatusFound, promptLibraryRedirectURL(searchQuery, "Prompt added."))
}

func (s *Server) handleAdminPromptGenerate(c *gin.Context) {
	searchQuery := normalizePromptLibraryQuery(c.PostForm("q"))
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage, searchQuery)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	instructions := strings.TrimSpace(c.PostForm("instructions"))
	if instructions == "" {
		s.renderPromptLibraryGenerateError(c, "Please provide guidance for the prompt generation.", instructions, searchQuery)
		return
	}
	prompts, err := s.generatePromptsFromOpenAI(c.Request.Context(), instructions)
	if err != nil {
		s.renderPromptLibraryGenerateError(c, err.Error(), instructions, searchQuery)
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
		s.renderPromptLibraryGenerateError(c, "No valid prompts were generated. Try again.", instructions, searchQuery)
		return
	}

	filteredEntries, embeddingByText, err := s.filterGeneratedPromptEntries(c.Request.Context(), entries)
	if err != nil {
		s.renderPromptLibraryGenerateError(c, "Failed to compare generated prompts with existing prompts.", instructions, searchQuery)
		return
	}
	if len(filteredEntries) == 0 {
		s.renderPromptLibraryGenerateError(c, "All generated prompts were too similar to existing prompts. Try different guidance.", instructions, searchQuery)
		return
	}
	added, err := s.insertPromptLibraryEntries(c.Request.Context(), filteredEntries, embeddingByText)
	if err != nil {
		s.renderPromptLibraryGenerateError(c, "Failed to save generated prompts.", instructions, searchQuery)
		return
	}
	if added == 0 {
		s.renderPromptLibraryGenerateError(c, "Generated prompts already exist or were too similar.", instructions, searchQuery)
		return
	}

	c.Redirect(http.StatusFound, promptLibraryRedirectURL(searchQuery, fmt.Sprintf("Added %d prompt(s) to the library.", added)))
}

func (s *Server) handleAdminPromptUpdate(c *gin.Context) {
	searchQuery := normalizePromptLibraryQuery(c.PostForm("q"))
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage, searchQuery)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		s.renderPromptLibraryError(c, "Invalid prompt id.", "", "", searchQuery)
		return
	}
	text, err := validatePrompt(c.PostForm("text"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"), searchQuery)
		return
	}
	joke, err := validateJoke(c.PostForm("joke"))
	if err != nil {
		s.renderPromptLibraryError(c, err.Error(), c.PostForm("text"), c.PostForm("joke"), searchQuery)
		return
	}

	var entry db.PromptLibrary
	if err := s.db.First(&entry, uint(id)).Error; err != nil {
		s.renderPromptLibraryError(c, "Prompt not found.", "", "", searchQuery)
		return
	}
	entry.Text = text
	if err := s.db.Model(&entry).Updates(map[string]any{
		"Text": text,
		"Joke": joke,
	}).Error; err != nil {
		s.renderPromptLibraryError(c, "Failed to update prompt (it may already exist).", text, joke, searchQuery)
		return
	}
	if err := s.ensurePromptLibraryEmbedding(c.Request.Context(), entry.ID, text); err != nil {
		s.renderPromptLibraryError(c, "Prompt updated, but embedding generation failed.", text, joke, searchQuery)
		return
	}

	c.Redirect(http.StatusFound, promptLibraryRedirectURL(searchQuery, "Prompt updated."))
}

func (s *Server) handleAdminPromptDelete(c *gin.Context) {
	searchQuery := normalizePromptLibraryQuery(c.PostForm("q"))
	if s.db == nil {
		data := s.loadPromptLibraryData(1, promptLibraryDefaultPerPage, searchQuery)
		data.Error = "Database not configured."
		templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		s.renderPromptLibraryError(c, "Invalid prompt id.", "", "", searchQuery)
		return
	}
	result := s.db.Delete(&db.PromptLibrary{}, uint(id))
	if result.Error != nil {
		s.renderPromptLibraryError(c, "Failed to delete prompt.", "", "", searchQuery)
		return
	}
	if result.RowsAffected == 0 {
		s.renderPromptLibraryError(c, "Prompt not found.", "", "", searchQuery)
		return
	}

	c.Redirect(http.StatusFound, promptLibraryRedirectURL(searchQuery, "Prompt deleted."))
}

func (s *Server) loadPromptLibraryData(page, perPage int, searchQuery string) web.AdminPromptLibraryData {
	searchQuery = normalizePromptLibraryQuery(searchQuery)
	data := web.AdminPromptLibraryData{SearchQuery: searchQuery}
	if s.db == nil {
		data.Error = "Database not configured."
		return data
	}
	query := s.db.Model(&db.PromptLibrary{})
	if searchQuery != "" {
		pattern := "%" + searchQuery + "%"
		query = query.Where("text ILIKE ? OR joke ILIKE ?", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		data.Error = "Failed to load prompt library."
		return data
	}
	pagination := buildPaginationData(promptLibraryBasePath(searchQuery), page, perPage, total)
	offset := (pagination.Page - 1) * pagination.PerPage
	if err := query.Order("id asc").Limit(pagination.PerPage).Offset(offset).Find(&data.Prompts).Error; err != nil {
		data.Error = "Failed to load prompt library."
	}
	data.Pagination = pagination
	return data
}

func (s *Server) renderPromptLibraryError(c *gin.Context, message, text, joke, searchQuery string) {
	page, perPage := parsePagination(c, promptLibraryDefaultPerPage, promptLibraryMaxPerPage)
	data := s.loadPromptLibraryData(page, perPage, searchQuery)
	data.Error = message
	data.DraftText = text
	data.DraftJoke = joke
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) renderPromptLibraryGenerateError(c *gin.Context, message, instructions, searchQuery string) {
	page, perPage := parsePagination(c, promptLibraryDefaultPerPage, promptLibraryMaxPerPage)
	data := s.loadPromptLibraryData(page, perPage, searchQuery)
	data.Error = message
	data.GenerateInstructions = instructions
	templ.Handler(web.AdminPromptLibrary(data)).ServeHTTP(c.Writer, c.Request)
}

func normalizePromptLibraryQuery(raw string) string {
	return strings.TrimSpace(raw)
}

func promptLibraryBasePath(searchQuery string) string {
	searchQuery = normalizePromptLibraryQuery(searchQuery)
	if searchQuery == "" {
		return "/admin/prompts"
	}
	return "/admin/prompts?q=" + url.QueryEscape(searchQuery)
}

func promptLibraryRedirectURL(searchQuery, notice string) string {
	values := url.Values{}
	if clean := strings.TrimSpace(notice); clean != "" {
		values.Set("notice", clean)
	}
	if clean := normalizePromptLibraryQuery(searchQuery); clean != "" {
		values.Set("q", clean)
	}
	encoded := values.Encode()
	if encoded == "" {
		return "/admin/prompts"
	}
	return "/admin/prompts?" + encoded
}

const (
	promptLibraryDefaultPerPage = 20
	promptLibraryMaxPerPage     = 200
)
