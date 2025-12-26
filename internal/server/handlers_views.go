package server

import (
	"log"
	"net/http"

	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

type gameURI struct {
	GameID string `uri:"gameID" binding:"required"`
}

type playerViewURI struct {
	GameID   string `uri:"gameID" binding:"required"`
	PlayerID int    `uri:"playerID" binding:"required,gt=0"`
}

func (s *Server) handleGameView(c *gin.Context) {
	var uri gameURI
	if !bindURI(c, &uri) {
		return
	}
	if _, ok := s.store.GetGame(uri.GameID); !ok {
		log.Printf("game view missing game_id=%s", uri.GameID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	templ.Handler(web.GameView(uri.GameID)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleDisplayView(c *gin.Context) {
	var uri gameURI
	if !bindURI(c, &uri) {
		return
	}
	game, ok := s.store.GetGame(uri.GameID)
	if !ok {
		log.Printf("display view missing game_id=%s", uri.GameID)
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
	var uri gameURI
	if !bindURI(c, &uri) {
		return
	}
	if _, exists := s.store.GetGame(uri.GameID); !exists {
		c.Status(http.StatusNotFound)
		return
	}
	templ.Handler(web.ReplayView(uri.GameID)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handlePlayerView(c *gin.Context) {
	var uri playerViewURI
	if !bindURI(c, &uri) {
		return
	}
	game, player, ok := s.store.GetPlayer(uri.GameID, uri.PlayerID)
	if !ok || game == nil || player == nil {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "Game not found. Start a new one or join with a fresh code.")
		}
		log.Printf("player view missing game_id=%s player_id=%d", uri.GameID, uri.PlayerID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	if game.Phase == phaseComplete {
		if s.sessions != nil {
			s.sessions.SetFlash(c.Writer, c.Request, "That game has ended. Start a new one!")
		}
		log.Printf("player view ended game_id=%s player_id=%d", uri.GameID, uri.PlayerID)
		c.Redirect(http.StatusFound, "/")
		return
	}
	templ.Handler(web.PlayerView(uri.GameID, uri.PlayerID, player.Name)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleDisplayPartial(c *gin.Context) {
	var uri gameURI
	if !bindURI(c, &uri) {
		return
	}
	game, ok := s.store.GetGame(uri.GameID)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	templ.Handler(web.DisplayContent(s.buildDisplayState(game))).ServeHTTP(c.Writer, c.Request)
}
