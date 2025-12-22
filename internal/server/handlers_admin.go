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
	data := web.AdminData{}
	if s.db == nil {
		data.Error = "Database not configured."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	game, ok := s.store.GetGame(gameID)
	if !ok || game == nil {
		data.Error = "Game not found in memory."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	if err := s.ensureGameDBID(game); err != nil {
		data.Error = "Failed to resolve game in database."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	if game.DBID == 0 {
		data.Error = "Game not found in database."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}

	if err := s.db.First(&data.Game, game.DBID).Error; err != nil {
		data.Error = "Failed to load game record."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	if err := s.db.Where("game_id = ?", game.DBID).Order("id asc").Find(&data.Players).Error; err != nil {
		data.Error = "Failed to load players."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	if err := s.db.Where("game_id = ?", game.DBID).Order("number asc").Find(&data.Rounds).Error; err != nil {
		data.Error = "Failed to load rounds."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	if err := s.db.Where("game_id = ?", game.DBID).Order("created_at asc").Find(&data.Events).Error; err != nil {
		data.Error = "Failed to load events."
		templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}

	roundIDs := make([]uint, 0, len(data.Rounds))
	for _, round := range data.Rounds {
		roundIDs = append(roundIDs, round.ID)
	}
	if len(roundIDs) > 0 {
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Prompts).Error; err != nil {
			data.Error = "Failed to load prompts."
			templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
			return
		}
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Drawings).Error; err != nil {
			data.Error = "Failed to load drawings."
			templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
			return
		}
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Guesses).Error; err != nil {
			data.Error = "Failed to load guesses."
			templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
			return
		}
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Votes).Error; err != nil {
			data.Error = "Failed to load votes."
			templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
			return
		}
	}

	templ.Handler(web.Admin(gameID, data)).ServeHTTP(c.Writer, c.Request)
}
