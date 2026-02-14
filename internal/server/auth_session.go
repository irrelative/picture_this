package server

import (
	"net/http"

	"picture-this/internal/db"

	"github.com/gin-gonic/gin"
)

func (s *Server) currentSessionUser(c *gin.Context) (db.User, bool) {
	if s.sessions == nil {
		return db.User{}, false
	}
	return s.sessions.CurrentUser(c.Writer, c.Request)
}

func (s *Server) requireSessionUser(c *gin.Context) (db.User, bool) {
	user, ok := s.currentSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return db.User{}, false
	}
	return user, true
}

func (s *Server) requireAdminPage(c *gin.Context) bool {
	user, ok := s.currentSessionUser(c)
	if !ok || !user.IsAdmin {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Admin access required.")
		}
		c.Redirect(http.StatusFound, "/")
		return false
	}
	return true
}

func (s *Server) adminPageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.requireAdminPage(c) {
			c.Abort()
			return
		}
		c.Next()
	}
}
