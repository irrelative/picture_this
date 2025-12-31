package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleAdminRestoreGame(c *gin.Context) {
	gameID := strings.TrimSpace(c.Param("gameID"))
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	game, displayID, err := s.restoreGameFromDB(gameID)
	if err != nil {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Restore failed: "+err.Error())
		}
		if displayID != "" {
			c.Redirect(http.StatusFound, "/admin/"+displayID)
			return
		}
		c.Redirect(http.StatusFound, "/admin")
		return
	}
	if s.sessions != nil {
		message := "Game restored and paused"
		if game.PausedPhase != "" {
			message += " (was " + game.PausedPhase + ")"
		}
		s.sessions.SetFlash(c.Writer, c.Request, message+".")
	}
	if displayID != "" {
		c.Redirect(http.StatusFound, "/admin/"+displayID)
		return
	}
	c.Redirect(http.StatusFound, "/admin")
}

func (s *Server) handleAdminResumeGame(c *gin.Context) {
	gameID := strings.TrimSpace(c.Param("gameID"))
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	game, ok := s.findGameInStore(gameID)
	if !ok {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Game is not loaded in memory.")
		}
		c.Redirect(http.StatusFound, "/admin/"+gameID)
		return
	}
	if game.Phase != phasePaused {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Game is not paused.")
		}
		c.Redirect(http.StatusFound, "/admin/"+gameID)
		return
	}
	if !allPlayersClaimed(game.Players) {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "All players must rejoin before resuming.")
		}
		c.Redirect(http.StatusFound, "/admin/"+gameID)
		return
	}
	resumePhase := game.PausedPhase
	if resumePhase == "" {
		resumePhase = phaseLobby
	}
	game, err := s.store.UpdateGame(game.ID, func(game *Game) error {
		setPhase(game, resumePhase)
		game.PausedPhase = ""
		return nil
	})
	if err != nil {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Failed to resume game.")
		}
		c.Redirect(http.StatusFound, "/admin/"+gameID)
		return
	}
	if game.Phase == phaseComplete {
		s.cancelPhaseTimer(game.ID)
	} else {
		s.schedulePhaseTimer(game)
	}
	_ = s.persistEvent(game, "game_resumed", EventPayload{Phase: game.Phase, Reason: "resume"})
	if s.sessions != nil {
		s.sessions.SetFlash(c.Writer, c.Request, "Game resumed.")
	}
	s.broadcastGameUpdate(game)
	c.Redirect(http.StatusFound, "/admin/"+gameID)
}

func (s *Server) findGameInStore(gameID string) (*Game, bool) {
	if game, ok := s.store.GetGame(gameID); ok {
		return game, true
	}
	if strings.HasPrefix(gameID, "db-") {
		candidate := strings.TrimPrefix(gameID, "db-")
		if candidate != "" {
			if game, ok := s.store.GetGame("game-" + candidate); ok {
				return game, true
			}
		}
	}
	return s.store.FindGameByJoinCode(gameID)
}
