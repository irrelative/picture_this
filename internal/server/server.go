package server

import (
	"net/http"
	"sync"
	"time"

	"picture-this/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	store    *Store
	db       *gorm.DB
	ws       *wsHub
	homeWS   *homeHub
	cfg      config.Config
	sessions *sessionStore
	timersMu sync.Mutex
	timers   map[string]*time.Timer
}

func New(conn *gorm.DB, cfg config.Config) *Server {
	registerValidators()
	return &Server{
		store:    NewStore(),
		db:       conn,
		ws:       newWSHub(),
		homeWS:   newHomeHub(),
		cfg:      cfg,
		sessions: newSessionStore(conn),
		timers:   make(map[string]*time.Timer),
	}
}

func (s *Server) Handler() http.Handler {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	_ = router.SetTrustedProxies(nil)

	router.GET("/", s.handleHome)
	router.GET("/partials/home/games", s.handleHomeGamesPartial)
	router.GET("/join", s.handleJoinView)
	router.GET("/join/:code", s.handleJoinView)
	router.GET("/audience/:gameID", s.handleAudienceView)
	router.GET("/play/:gameID/:playerID", s.handlePlayerView)
	router.GET("/display/:gameID", s.handleDisplayView)
	router.GET("/partials/games/:gameID/display", s.handleDisplayPartial)
	router.GET("/replay/:gameID", s.handleReplayView)

	admin := router.Group("/admin", s.adminPageMiddleware())
	{
		admin.GET("", s.handleAdminHome)
		admin.GET("/prompts", s.handleAdminPromptsView)
		admin.POST("/prompts", s.handleAdminPromptCreate)
		admin.POST("/prompts/generate", s.handleAdminPromptGenerate)
		admin.POST("/prompts/:id", s.handleAdminPromptUpdate)
		admin.POST("/prompts/:id/delete", s.handleAdminPromptDelete)
		admin.POST("/:gameID/restore", s.handleAdminRestoreGame)
		admin.POST("/:gameID/resume", s.handleAdminResumeGame)
		admin.GET("/:gameID", s.handleAdminView)
	}

	api := router.Group("/api")
	{
		api.POST("/auth/register", s.handleRegister)
		api.POST("/auth/login", s.handleLogin)
		api.POST("/auth/logout", s.handleLogout)
		api.POST("/games", s.handleCreateGame)
		api.GET("/games/:gameID", s.handleGetGame)
		api.GET("/games/:gameID/events", s.handleEvents)
		api.GET("/games/:gameID/results", s.handleResults)
		api.GET("/games/:gameID/players/:playerID/prompt", s.handlePlayerPrompt)
		api.POST("/games/:gameID/join", s.handleJoinGame)
		api.POST("/games/:gameID/audience", s.handleAudienceJoin)
		api.POST("/games/:gameID/audience/votes", s.handleAudienceVote)
		api.POST("/games/:gameID/avatar", s.handleAvatar)
		api.POST("/games/:gameID/start", s.handleStartGame)
		api.POST("/games/:gameID/drawings", s.handleDrawings)
		api.POST("/games/:gameID/guesses", s.handleGuesses)
		api.POST("/games/:gameID/votes", s.handleVotes)
		api.POST("/games/:gameID/settings", s.handleSettings)
		api.POST("/games/:gameID/kick", s.handleKick)
		api.POST("/games/:gameID/advance", s.handleAdvance)
		api.POST("/games/:gameID/end", s.handleEndGame)
	}

	router.GET("/ws/games/:gameID", s.handleWebsocket)
	router.GET("/ws/home", s.handleHomeWebsocket)
	router.Static("/static", "./static")
	return router
}

func (s *Server) snapshot(game *Game) map[string]any {
	return snapshotWithConfig(game, s.cfg)
}
