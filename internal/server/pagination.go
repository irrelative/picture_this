package server

import (
	"strconv"
	"strings"

	"picture-this/internal/web"

	"github.com/gin-gonic/gin"
)

func parsePagination(c *gin.Context, defaultPerPage, maxPerPage int) (int, int) {
	page := 1
	perPage := defaultPerPage
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			page = value
		}
	}
	if raw := strings.TrimSpace(c.Query("per_page")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			perPage = value
		}
	}
	if maxPerPage > 0 && perPage > maxPerPage {
		perPage = maxPerPage
	}
	return page, perPage
}

func buildPaginationData(basePath string, page, perPage int, total int64) web.PaginationData {
	if perPage <= 0 {
		perPage = 1
	}
	totalPages := int((total + int64(perPage) - 1) / int64(perPage))
	if totalPages == 0 {
		totalPages = 1
	}
	if page <= 0 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	data := web.PaginationData{
		BasePath:   basePath,
		Page:       page,
		PerPage:    perPage,
		Total:      int(total),
		TotalPages: totalPages,
	}
	data.HasPrev = page > 1
	data.HasNext = page < totalPages
	if data.HasPrev {
		data.PrevPage = page - 1
	}
	if data.HasNext {
		data.NextPage = page + 1
	}
	return data
}
