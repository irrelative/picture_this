package server

import (
	"log"
	"net/http"

	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAdminView(c *gin.Context) {
	gameID := c.Param("gameID")
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if _, ok := s.store.GetGame(gameID); !ok {
		log.Printf("admin view missing game_id=%s", gameID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	templ.Handler(web.Admin(gameID)).ServeHTTP(c.Writer, c.Request)
}
