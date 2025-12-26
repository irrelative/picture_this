package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type bindMessages map[string]map[string]string

func bindJSON(c *gin.Context, req any, messages bindMessages, fallback string) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": resolveBindError(err, messages, fallback)})
		return false
	}
	return true
}

func bindURI(c *gin.Context, req any) bool {
	if err := c.ShouldBindUri(req); err != nil {
		c.Status(http.StatusNotFound)
		return false
	}
	return true
}

func bindQuery(c *gin.Context, req any) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		c.Status(http.StatusBadRequest)
		return false
	}
	return true
}

func resolveBindError(err error, messages bindMessages, fallback string) string {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, verr := range verrs {
			if fieldMsgs, ok := messages[verr.Field()]; ok {
				if msg, ok := fieldMsgs[verr.Tag()]; ok {
					return msg
				}
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	return "invalid request"
}
