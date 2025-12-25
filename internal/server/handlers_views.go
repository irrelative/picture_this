package server

import (
	"log"
	"net/http"
	"strconv"

	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleGameView(c *gin.Context) {
	gameID := c.Param("gameID")
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if _, ok := s.store.GetGame(gameID); !ok {
		log.Printf("game view missing game_id=%s", gameID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	templ.Handler(web.GameView(gameID)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleDisplayView(c *gin.Context) {
	gameID := c.Param("gameID")
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	game, ok := s.store.GetGame(gameID)
	if !ok {
		log.Printf("display view missing game_id=%s", gameID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	templ.Handler(web.DisplayView(s.buildDisplayState(game))).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleHome(c *gin.Context) {
	flash := ""
	name := ""
	if s.sessions != nil {
		flash = s.sessions.PopFlash(c.Writer, c.Request)
		name = s.sessions.GetName(c.Writer, c.Request)
	}
	templ.Handler(web.Home(flash, name, s.homeSummaries())).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleHomeGamesPartial(c *gin.Context) {
	templ.Handler(web.ActiveGamesList(s.homeSummaries())).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleJoinView(c *gin.Context) {
	code := c.Param("code")
	name := ""
	if s.sessions != nil {
		name = s.sessions.GetName(c.Writer, c.Request)
	}
	templ.Handler(web.JoinView(code, name)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleReplayView(c *gin.Context) {
	gameID := c.Param("gameID")
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if _, exists := s.store.GetGame(gameID); !exists {
		c.Status(http.StatusNotFound)
		return
	}
	templ.Handler(web.ReplayView(gameID)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handlePlayerView(c *gin.Context) {
	gameID := c.Param("gameID")
	playerID, err := strconv.Atoi(c.Param("playerID"))
	if gameID == "" || err != nil || playerID <= 0 {
		c.Status(http.StatusNotFound)
		return
	}
	game, player, ok := s.store.GetPlayer(gameID, playerID)
	if !ok || game == nil || player == nil {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Game not found. Start a new one or join with a fresh code.")
		}
		log.Printf("player view missing game_id=%s player_id=%d", gameID, playerID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	if game.Phase == phaseComplete {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "That game has ended. Start a new one!")
		}
		log.Printf("player view ended game_id=%s player_id=%d", gameID, playerID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	templ.Handler(web.PlayerView(gameID, playerID, player.Name)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleDisplayPartial(c *gin.Context) {
	gameID := c.Param("gameID")
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	game, ok := s.store.GetGame(gameID)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	templ.Handler(web.DisplayContent(s.buildDisplayState(game))).ServeHTTP(c.Writer, c.Request)
}
