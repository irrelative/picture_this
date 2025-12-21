package server

import (
	"net/http"
	"sync"
	"time"

	"picture-this/internal/config"

	"gorm.io/gorm"
)

type Server struct {
	store    *Store
	db       *gorm.DB
	ws       *wsHub
	homeWS   *homeHub
	cfg      config.Config
	sessions *sessionStore
	limiter  *rateLimiter
	timersMu sync.Mutex
	timers   map[string]*time.Timer
}

func New(conn *gorm.DB, cfg config.Config) *Server {
	return &Server{
		store:    NewStore(),
		db:       conn,
		ws:       newWSHub(),
		homeWS:   newHomeHub(),
		cfg:      cfg,
		sessions: newSessionStore(conn),
		limiter:  newRateLimiter(),
		timers:   make(map[string]*time.Timer),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /join", s.handleJoinView)
	mux.HandleFunc("GET /join/", s.handleJoinView)
	mux.HandleFunc("GET /play/", s.handlePlayerView)
	mux.HandleFunc("GET /games/", s.handleGameView)
	mux.HandleFunc("GET /display/", s.handleDisplayView)
	mux.HandleFunc("GET /replay/", s.handleReplayView)
	mux.HandleFunc("GET /audience/", s.handleAudienceView)
	mux.HandleFunc("POST /api/games", s.handleCreateGame)
	mux.HandleFunc("GET /api/games/", s.handleGameSubroutes)
	mux.HandleFunc("POST /api/games/", s.handleGameSubroutes)
	mux.HandleFunc("GET /api/prompts/categories", s.handlePromptCategories)
	mux.HandleFunc("GET /ws/games/", s.handleWebsocket)
	mux.HandleFunc("GET /ws/home", s.handleHomeWebsocket)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return mux
}

func (s *Server) snapshot(game *Game) map[string]any {
	return snapshotWithConfig(game, s.cfg)
}
