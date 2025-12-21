package server

import (
	"log"
	"net/http"
	"strings"

	"picture-this/internal/web"

	"github.com/a-h/templ"
)

func (s *Server) handleGameView(w http.ResponseWriter, r *http.Request) {
	gameID := strings.TrimPrefix(r.URL.Path, "/games/")
	gameID = strings.Trim(gameID, "/")
	if gameID == "" || strings.Contains(gameID, "/") {
		http.NotFound(w, r)
		return
	}
	if _, ok := s.store.GetGame(gameID); !ok {
		log.Printf("game view missing game_id=%s", gameID)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	templ.Handler(web.GameView(gameID)).ServeHTTP(w, r)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	flash := ""
	name := ""
	if s.sessions != nil {
		flash = s.sessions.PopFlash(w, r)
		name = s.sessions.GetName(w, r)
	}
	templ.Handler(web.Home(flash, name, s.homeSummaries())).ServeHTTP(w, r)
}

func (s *Server) handleJoinView(w http.ResponseWriter, r *http.Request) {
	code := ""
	name := ""
	if strings.HasPrefix(r.URL.Path, "/join/") {
		code = strings.TrimPrefix(r.URL.Path, "/join/")
		code = strings.Trim(code, "/")
		if code != "" && strings.Contains(code, "/") {
			http.NotFound(w, r)
			return
		}
	}
	if s.sessions != nil {
		name = s.sessions.GetName(w, r)
	}
	templ.Handler(web.JoinView(code, name)).ServeHTTP(w, r)
}

func (s *Server) handleReplayView(w http.ResponseWriter, r *http.Request) {
	gameID, ok := parseReplayPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, exists := s.store.GetGame(gameID); !exists {
		http.NotFound(w, r)
		return
	}
	templ.Handler(web.ReplayView(gameID)).ServeHTTP(w, r)
}

func (s *Server) handleAudienceView(w http.ResponseWriter, r *http.Request) {
	gameID, audienceID, ok := parseAudienceViewPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	game, ok := s.store.GetGame(gameID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	name := ""
	for _, member := range game.Audience {
		if member.ID == audienceID {
			name = member.Name
			break
		}
	}
	if name == "" {
		http.NotFound(w, r)
		return
	}
	templ.Handler(web.AudienceView(gameID, audienceID, name)).ServeHTTP(w, r)
}

func (s *Server) handlePlayerView(w http.ResponseWriter, r *http.Request) {
	gameID, playerID, ok := parsePlayerPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	game, player, ok := s.store.GetPlayer(gameID, playerID)
	if !ok || game == nil || player == nil {
		if s.sessions != nil {
			s.sessions.SetFlash(w, r, "Game not found. Start a new one or join with a fresh code.")
		}
		log.Printf("player view missing game_id=%s player_id=%d", gameID, playerID)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if game.Phase == phaseComplete {
		if s.sessions != nil {
			s.sessions.SetFlash(w, r, "That game has ended. Start a new one!")
		}
		log.Printf("player view ended game_id=%s player_id=%d", gameID, playerID)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	templ.Handler(web.PlayerView(gameID, playerID, player.Name)).ServeHTTP(w, r)
}
