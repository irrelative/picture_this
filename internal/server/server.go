package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"picture-this/internal/config"
	"picture-this/internal/db"
	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgconn"
	"gorm.io/datatypes"
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

const (
	phaseLobby      = "lobby"
	phaseDrawings   = "drawings"
	phaseGuesses    = "guesses"
	phaseGuessVotes = "guesses-votes"
	phaseResults    = "results"
	phaseComplete   = "complete"
)

const (
	revealStageGuesses = "guesses"
	revealStageVotes   = "votes"
)

var phaseOrder = []string{
	phaseLobby,
	phaseDrawings,
	phaseGuesses,
	phaseGuessVotes,
	phaseResults,
	phaseComplete,
}

const (
	maxNameLength     = 20
	maxGuessLength    = 60
	maxPromptLength   = 140
	maxChoiceLength   = 140
	maxCategoryLength = 32
	maxDrawingBytes   = 250 * 1024
	maxRoundsPerGame  = 10
	maxLobbyPlayers   = 12
	rateLimitExceeded = "rate limit exceeded"
)

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

type Store struct {
	mu             sync.Mutex
	nextID         int
	nextPlayerID   int
	nextAudienceID int
	games          map[string]*Game
}

type GameSummary struct {
	ID       string
	JoinCode string
	Phase    string
	Players  int
}

type Game struct {
	ID               string
	DBID             uint
	JoinCode         string
	Phase            string
	PhaseStartedAt   time.Time
	MaxPlayers       int
	PromptCategory   string
	LobbyLocked      bool
	UsedPrompts      map[string]struct{}
	KickedPlayers    map[string]struct{}
	HostID           int
	Players          []Player
	Audience         []AudienceMember
	Rounds           []RoundState
	PromptsPerPlayer int
}

type Player struct {
	ID     int
	Name   string
	IsHost bool
	DBID   uint
	Color  string
}

type AudienceMember struct {
	ID   int
	Name string
}

type RoundState struct {
	Number        int
	DBID          uint
	Prompts       []PromptEntry
	Drawings      []DrawingEntry
	Guesses       []GuessEntry
	Votes         []VoteEntry
	GuessTurns    []GuessTurn
	CurrentGuess  int
	VoteTurns     []VoteTurn
	CurrentVote   int
	RevealIndex   int
	RevealStage   string
	AudienceVotes []AudienceVote
}

type PromptEntry struct {
	PlayerID int
	Text     string
	DBID     uint
}

type DrawingEntry struct {
	PlayerID  int
	ImageData []byte
	Prompt    string
	DBID      uint
}

type GuessEntry struct {
	PlayerID     int
	DrawingIndex int
	Text         string
	DBID         uint
}

type GuessTurn struct {
	DrawingIndex int
	GuesserID    int
}

type VoteEntry struct {
	PlayerID     int
	DrawingIndex int
	ChoiceText   string
	ChoiceType   string
	DBID         uint
}

type VoteTurn struct {
	DrawingIndex int
	VoterID      int
}

type AudienceVote struct {
	AudienceID   int
	DrawingIndex int
	ChoiceText   string
}

func NewStore() *Store {
	return &Store{
		nextID:         1,
		nextPlayerID:   1,
		nextAudienceID: 1,
		games:          make(map[string]*Game),
	}
}

func (s *Store) CreateGame(promptsPerPlayer int) *Game {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("game-%d", s.nextID)
	s.nextID++
	game := &Game{
		ID:               id,
		JoinCode:         newJoinCode(),
		Phase:            phaseLobby,
		PhaseStartedAt:   time.Now().UTC(),
		MaxPlayers:       0,
		PromptCategory:   "",
		LobbyLocked:      false,
		UsedPrompts:      make(map[string]struct{}),
		KickedPlayers:    make(map[string]struct{}),
		PromptsPerPlayer: promptsPerPlayer,
	}
	s.games[id] = game
	return game
}

func (s *Store) GetGame(id string) (*Game, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[id]
	return game, ok
}

func (s *Store) UpdateGame(id string, update func(game *Game) error) (*Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[id]
	if !ok {
		return nil, errors.New("game not found")
	}
	if err := update(game); err != nil {
		return nil, err
	}
	return game, nil
}

func (s *Store) FindGameByJoinCode(code string) (*Game, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, game := range s.games {
		if game.JoinCode == code {
			return game, true
		}
	}
	return nil, false
}

func (s *Store) UpdateGameID(game *Game, newID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if game.ID == newID {
		return
	}
	delete(s.games, game.ID)
	game.ID = newID
	s.games[newID] = game
}

func (s *Store) AddPlayer(gameIDOrCode, name string) (*Game, *Player, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, ok := s.games[gameIDOrCode]
	if !ok {
		for _, candidate := range s.games {
			if candidate.JoinCode == gameIDOrCode {
				game = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, nil, errors.New("game not found")
	}

	for i := range game.Players {
		if game.Players[i].Name == name {
			return game, &game.Players[i], nil
		}
	}
	if game.Phase != phaseLobby {
		return nil, nil, errors.New("game already started")
	}
	if game.LobbyLocked {
		return nil, nil, errors.New("lobby locked")
	}
	if game.MaxPlayers > 0 && len(game.Players) >= game.MaxPlayers {
		return nil, nil, errors.New("lobby full")
	}
	if game.KickedPlayers != nil {
		if _, kicked := game.KickedPlayers[strings.ToLower(name)]; kicked {
			return nil, nil, errors.New("player removed")
		}
	}

	player := Player{
		ID:     s.nextPlayerID,
		Name:   name,
		IsHost: len(game.Players) == 0,
		Color:  pickPlayerColor(len(game.Players)),
	}
	s.nextPlayerID++
	game.Players = append(game.Players, player)
	if player.IsHost {
		game.HostID = player.ID
	}
	return game, &game.Players[len(game.Players)-1], nil
}

func (s *Store) AddAudience(gameIDOrCode, name string) (*Game, *AudienceMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, ok := s.games[gameIDOrCode]
	if !ok {
		for _, candidate := range s.games {
			if candidate.JoinCode == gameIDOrCode {
				game = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, nil, errors.New("game not found")
	}
	if game.Phase == phaseComplete {
		return nil, nil, errors.New("game already ended")
	}
	for i := range game.Audience {
		if strings.EqualFold(game.Audience[i].Name, name) {
			return game, &game.Audience[i], nil
		}
	}
	member := AudienceMember{
		ID:   s.nextAudienceID,
		Name: name,
	}
	s.nextAudienceID++
	game.Audience = append(game.Audience, member)
	return game, &game.Audience[len(game.Audience)-1], nil
}

func (s *Store) ListGameSummaries() []GameSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	games := make([]GameSummary, 0, len(s.games))
	for _, game := range s.games {
		games = append(games, GameSummary{
			ID:       game.ID,
			JoinCode: game.JoinCode,
			Phase:    game.Phase,
			Players:  len(game.Players),
		})
	}
	sort.Slice(games, func(i, j int) bool {
		return gameSortKey(games[i].ID) < gameSortKey(games[j].ID)
	})
	return games
}

func gameSortKey(id string) int {
	if strings.HasPrefix(id, "game-") {
		if parsed, err := strconv.Atoi(strings.TrimPrefix(id, "game-")); err == nil {
			return parsed
		}
	}
	return 0
}

func (s *Store) GetPlayer(gameID string, playerID int) (*Game, *Player, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, ok := s.games[gameID]
	if !ok {
		return nil, nil, false
	}
	for i := range game.Players {
		if game.Players[i].ID == playerID {
			return game, &game.Players[i], true
		}
	}
	return game, nil, false
}

func (s *Store) FindPlayer(game *Game, playerID int) (*Player, bool) {
	for i := range game.Players {
		if game.Players[i].ID == playerID {
			return &game.Players[i], true
		}
	}
	return nil, false
}

func newJoinCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "AAAAAA"
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	if !s.enforceRateLimit(w, r, "create") {
		return
	}
	game := s.store.CreateGame(s.cfg.PromptsPerPlayer)
	if err := s.persistGame(game); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create game")
		return
	}
	log.Printf("game created game_id=%s join_code=%s", game.ID, game.JoinCode)
	resp := map[string]string{
		"game_id":   game.ID,
		"join_code": game.JoinCode,
	}
	writeJSON(w, http.StatusCreated, resp)
	s.broadcastHomeUpdate()
}

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
	if !ok {
		if s.sessions != nil {
			s.sessions.SetFlash(w, r, "Game not found. Start a new one or join with a fresh code.")
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if game.Phase == phaseComplete {
		if s.sessions != nil {
			s.sessions.SetFlash(w, r, "That game has ended. Start a new one!")
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	templ.Handler(web.PlayerView(game.ID, player.ID, player.Name)).ServeHTTP(w, r)
}

func (s *Server) handlePlayerPrompt(w http.ResponseWriter, r *http.Request, gameID string, playerID int) {
	game, player, ok := s.store.GetPlayer(gameID, playerID)
	if !ok || player == nil {
		http.NotFound(w, r)
		return
	}
	round := currentRound(game)
	if round == nil {
		writeError(w, http.StatusConflict, "round not started")
		return
	}
	prompt := promptForPlayer(round, player.ID)
	if prompt == "" {
		writeError(w, http.StatusNotFound, "prompt not assigned")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game_id":   game.ID,
		"player_id": player.ID,
		"prompt":    prompt,
	})
}

func (s *Server) handleGameSubroutes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if playerGameID, playerID, ok := parsePlayerPromptPath(r.URL.Path); ok {
			s.handlePlayerPrompt(w, r, playerGameID, playerID)
			return
		}
	}
	if r.Method == http.MethodPost {
		if audienceGameID, action, ok := parseAudiencePath(r.URL.Path); ok {
			switch action {
			case "":
				s.handleAudienceJoin(w, r, audienceGameID)
			case "votes":
				s.handleAudienceVotes(w, r, audienceGameID)
			default:
				http.NotFound(w, r)
			}
			return
		}
	}

	gameID, action, ok := parseGamePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		switch action {
		case "":
			s.handleGetGame(w, r, gameID)
		case "events":
			s.handleEvents(w, r, gameID)
		case "results":
			s.handleResults(w, r, gameID)
		default:
			http.NotFound(w, r)
		}
	case http.MethodPost:
		switch action {
		case "join":
			s.handleJoinGame(w, r, gameID)
		case "start":
			s.handleStartGame(w, r, gameID)
		case "drawings":
			s.handleDrawings(w, r, gameID)
		case "guesses":
			s.handleGuesses(w, r, gameID)
		case "votes":
			s.handleVotes(w, r, gameID)
		case "settings":
			s.handleSettings(w, r, gameID)
		case "kick":
			s.handleKick(w, r, gameID)
		case "rename":
			s.handleRename(w, r, gameID)
		case "advance":
			s.handleAdvance(w, r, gameID)
		case "end":
			s.handleEndGame(w, r, gameID)
		default:
			http.NotFound(w, r)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request, gameID string) {
	game, ok := s.store.GetGame(gameID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

type settingsRequest struct {
	PlayerID       int    `json:"player_id"`
	Rounds         int    `json:"rounds"`
	MaxPlayers     int    `json:"max_players"`
	PromptCategory string `json:"prompt_category"`
	LobbyLocked    bool   `json:"lobby_locked"`
}

type kickRequest struct {
	PlayerID int `json:"player_id"`
	TargetID int `json:"target_id"`
}

type renameRequest struct {
	PlayerID int    `json:"player_id"`
	Name     string `json:"name"`
}

type joinRequest struct {
	Name string `json:"name"`
}

type startRequest struct {
	PlayerID int `json:"player_id"`
}

type audienceJoinRequest struct {
	Name string `json:"name"`
}

type audienceVoteRequest struct {
	AudienceID   int    `json:"audience_id"`
	DrawingIndex int    `json:"drawing_index"`
	Choice       string `json:"choice"`
}

func (s *Server) handleJoinGame(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "join") {
		return
	}
	var req joinRequest
	if err := readJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	name, err := validateName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	game, player, err := s.store.AddPlayer(gameID, name)
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	playerID, persistErr := s.persistPlayer(game, player)
	if persistErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to join game")
		return
	}

	resp := map[string]any{
		"game_id":   game.ID,
		"player":    name,
		"join_code": game.JoinCode,
	}
	resp["player_id"] = playerID
	writeJSON(w, http.StatusOK, resp)
	log.Printf("player joined game_id=%s player_id=%d player_name=%s", game.ID, playerID, name)

	if s.sessions != nil {
		s.sessions.SetName(w, r, name)
	}

	s.broadcastGameUpdate(game)
}

func (s *Server) handleAudienceJoin(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "audience_join") {
		return
	}
	var req audienceJoinRequest
	if err := readJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	name, err := validateName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	game, audience, err := s.store.AddAudience(gameID, name)
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	resp := map[string]any{
		"game_id":     game.ID,
		"audience_id": audience.ID,
		"join_code":   game.JoinCode,
	}
	writeJSON(w, http.StatusOK, resp)
	log.Printf("audience joined game_id=%s audience_id=%d", game.ID, audience.ID)
	s.broadcastGameUpdate(game)
}

func (s *Server) handleAudienceVotes(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "audience_vote") {
		return
	}
	var req audienceVoteRequest
	if err := readJSON(r.Body, &req); err != nil || req.AudienceID <= 0 || req.DrawingIndex < 0 || strings.TrimSpace(req.Choice) == "" {
		writeError(w, http.StatusBadRequest, "audience vote is required")
		return
	}
	choiceText, err := validateChoice(req.Choice)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseGuessVotes {
			return errors.New("audience votes not accepted in this phase")
		}
		found := false
		for _, member := range game.Audience {
			if member.ID == req.AudienceID {
				found = true
				break
			}
		}
		if !found {
			return errors.New("audience member not found")
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		if req.DrawingIndex >= len(round.Drawings) {
			return errors.New("invalid drawing")
		}
		for _, vote := range round.AudienceVotes {
			if vote.AudienceID == req.AudienceID && vote.DrawingIndex == req.DrawingIndex {
				return errors.New("vote already submitted")
			}
		}
		options := voteOptionsForDrawing(round, req.DrawingIndex)
		if !containsOption(options, choiceText) {
			return errors.New("invalid vote option")
		}
		round.AudienceVotes = append(round.AudienceVotes, AudienceVote{
			AudienceID:   req.AudienceID,
			DrawingIndex: req.DrawingIndex,
			ChoiceText:   choiceText,
		})
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	log.Printf("audience vote submitted game_id=%s audience_id=%d", game.ID, req.AudienceID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, gameID string) {
	if s.db == nil {
		writeError(w, http.StatusServiceUnavailable, "events not available")
		return
	}
	game, ok := s.store.GetGame(gameID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load game")
			return
		}
	}
	var records []db.Event
	if err := s.db.Where("game_id = ?", game.DBID).Order("created_at asc").Find(&records).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load events")
		return
	}
	events := make([]map[string]any, 0, len(records))
	for _, record := range records {
		events = append(events, map[string]any{
			"id":         record.ID,
			"type":       record.Type,
			"round_id":   record.RoundID,
			"player_id":  record.PlayerID,
			"created_at": record.CreatedAt,
			"payload":    record.Payload,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game_id": game.ID,
		"events":  events,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "settings") {
		return
	}
	var req settingsRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 {
		writeError(w, http.StatusBadRequest, "player_id is required")
		return
	}
	if req.Rounds > maxRoundsPerGame {
		writeError(w, http.StatusBadRequest, "rounds exceeds maximum")
		return
	}
	if req.MaxPlayers > maxLobbyPlayers {
		writeError(w, http.StatusBadRequest, "max players exceeds maximum")
		return
	}
	if req.MaxPlayers < 0 || req.Rounds < 0 {
		writeError(w, http.StatusBadRequest, "invalid settings")
		return
	}
	category, err := validateCategory(req.PromptCategory)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseLobby {
			return errors.New("settings only available in lobby")
		}
		if game.HostID != 0 && req.PlayerID != game.HostID {
			return errors.New("only host can update settings")
		}
		if req.Rounds > 0 {
			game.PromptsPerPlayer = req.Rounds
		}
		if req.MaxPlayers >= 0 {
			if req.MaxPlayers > 0 && req.MaxPlayers < len(game.Players) {
				return errors.New("max players is below current player count")
			}
			game.MaxPlayers = req.MaxPlayers
		}
		game.PromptCategory = category
		game.LobbyLocked = req.LobbyLocked
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.persistSettings(game); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}
	log.Printf("settings updated game_id=%s rounds=%d max_players=%d category=%s locked=%t", game.ID, game.PromptsPerPlayer, game.MaxPlayers, game.PromptCategory, game.LobbyLocked)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleKick(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "kick") {
		return
	}
	var req kickRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 || req.TargetID <= 0 {
		writeError(w, http.StatusBadRequest, "player_id and target_id are required")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseLobby {
			return errors.New("kick only available in lobby")
		}
		if game.HostID != 0 && req.PlayerID != game.HostID {
			return errors.New("only host can remove players")
		}
		if req.TargetID == game.HostID {
			return errors.New("cannot remove host")
		}
		index := -1
		for i := range game.Players {
			if game.Players[i].ID == req.TargetID {
				index = i
				break
			}
		}
		if index == -1 {
			return errors.New("player not found")
		}
		if game.KickedPlayers == nil {
			game.KickedPlayers = make(map[string]struct{})
		}
		game.KickedPlayers[strings.ToLower(game.Players[index].Name)] = struct{}{}
		game.Players = append(game.Players[:index], game.Players[index+1:]...)
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	log.Printf("player removed game_id=%s target_id=%d", game.ID, req.TargetID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "rename") {
		return
	}
	var req renameRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 {
		writeError(w, http.StatusBadRequest, "player_id and name are required")
		return
	}
	newName, err := validateName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseLobby {
			return errors.New("rename only available in lobby")
		}
		for i := range game.Players {
			if strings.EqualFold(game.Players[i].Name, newName) {
				return errors.New("name already taken")
			}
		}
		for i := range game.Players {
			if game.Players[i].ID == req.PlayerID {
				game.Players[i].Name = newName
				return nil
			}
		}
		return errors.New("player not found")
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if s.sessions != nil {
		s.sessions.SetName(w, r, newName)
	}
	log.Printf("player renamed game_id=%s player_id=%d", game.ID, req.PlayerID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleStartGame(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "start") {
		return
	}
	var req startRequest
	_ = readJSON(r.Body, &req)
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if len(game.Players) < 2 {
			return errors.New("not enough players")
		}
		if game.HostID != 0 && req.PlayerID != 0 && req.PlayerID != game.HostID {
			return errors.New("only host can start")
		}
		if game.Phase != phaseLobby {
			return errors.New("game already started")
		}
		setPhase(game, phaseDrawings)
		game.Rounds = append(game.Rounds, RoundState{
			Number: len(game.Rounds) + 1,
		})
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.persistPhase(game, "game_started", map[string]any{"phase": game.Phase}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start game")
		return
	}
	if err := s.persistRound(game); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create round")
		return
	}
	if err := s.assignPrompts(game); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign prompts")
		return
	}
	log.Printf("game started game_id=%s phase=%s", game.ID, game.Phase)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

type drawingsRequest struct {
	PlayerID  int    `json:"player_id"`
	ImageData string `json:"image_data"`
	Prompt    string `json:"prompt"`
}

func (s *Server) handleDrawings(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "drawings") {
		return
	}
	var req drawingsRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 || req.ImageData == "" {
		writeError(w, http.StatusBadRequest, "drawings are required")
		return
	}
	promptText, err := validatePrompt(req.Prompt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	image, err := decodeImageData(req.ImageData)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image data")
		return
	}
	if len(image) > maxDrawingBytes {
		writeError(w, http.StatusBadRequest, "drawing exceeds size limit")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseDrawings {
			return errors.New("drawings not accepted in this phase")
		}
		player, ok := s.store.FindPlayer(game, req.PlayerID)
		if !ok {
			return errors.New("player not found")
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		for _, drawing := range round.Drawings {
			if drawing.PlayerID == player.ID {
				return errors.New("drawing already submitted")
			}
		}
		promptEntry, ok := findPromptForPlayer(round, player.ID, promptText)
		if !ok {
			return errors.New("prompt not assigned")
		}
		round.Drawings = append(round.Drawings, DrawingEntry{
			PlayerID:  player.ID,
			ImageData: image,
			Prompt:    promptEntry.Text,
		})
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.persistDrawing(game, req.PlayerID, image, promptText); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save drawings")
		return
	}
	advanced, updated, err := s.tryAdvanceToGuesses(gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to advance game")
		return
	}
	if advanced {
		game = updated
		if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to advance game")
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("drawing submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

type guessesRequest struct {
	PlayerID int    `json:"player_id"`
	Guess    string `json:"guess"`
}

func (s *Server) handleGuesses(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "guesses") {
		return
	}
	var req guessesRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 {
		writeError(w, http.StatusBadRequest, "guesses are required")
		return
	}
	guessText, err := validateGuess(req.Guess)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseGuesses {
			return errors.New("guesses not accepted in this phase")
		}
		player, ok := s.store.FindPlayer(game, req.PlayerID)
		if !ok {
			return errors.New("player not found")
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		if round.CurrentGuess >= len(round.GuessTurns) {
			return errors.New("no active guess turn")
		}
		turn := round.GuessTurns[round.CurrentGuess]
		if turn.GuesserID != player.ID {
			return errors.New("not your turn")
		}
		round.Guesses = append(round.Guesses, GuessEntry{
			PlayerID:     player.ID,
			DrawingIndex: turn.DrawingIndex,
			Text:         guessText,
		})
		round.CurrentGuess++
		if round.CurrentGuess >= len(round.GuessTurns) {
			if round.Number < game.PromptsPerPlayer {
				setPhase(game, phaseDrawings)
				game.Rounds = append(game.Rounds, RoundState{
					Number: len(game.Rounds) + 1,
				})
			} else {
				if err := s.buildVoteTurns(game, round); err != nil {
					return err
				}
				setPhase(game, phaseGuessVotes)
			}
		}
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	turnIndex := 0
	if round := currentRound(game); round != nil && round.CurrentGuess > 0 {
		turnIndex = round.CurrentGuess - 1
	}
	drawingIndex := -1
	if round := currentRound(game); round != nil && turnIndex < len(round.GuessTurns) {
		drawingIndex = round.GuessTurns[turnIndex].DrawingIndex
	}
	if err := s.persistGuess(game, req.PlayerID, drawingIndex, guessText); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save guesses")
		return
	}
	if game.Phase == phaseDrawings {
		if err := s.persistRound(game); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create round")
			return
		}
		if err := s.assignPrompts(game); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to assign prompts")
			return
		}
	}
	if game.Phase == phaseDrawings || game.Phase == phaseGuessVotes {
		if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to advance game")
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("guess submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

type votesRequest struct {
	PlayerID int    `json:"player_id"`
	Choice   string `json:"choice"`
	Guess    string `json:"guess"`
}

func (s *Server) handleVotes(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "votes") {
		return
	}
	var req votesRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 {
		writeError(w, http.StatusBadRequest, "votes are required")
		return
	}
	choiceText := strings.TrimSpace(req.Choice)
	if choiceText == "" {
		choiceText = strings.TrimSpace(req.Guess)
	}
	if choiceText == "" {
		writeError(w, http.StatusBadRequest, "votes are required")
		return
	}
	choiceText, err := validateChoice(choiceText)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseGuessVotes {
			return errors.New("votes not accepted in this phase")
		}
		player, ok := s.store.FindPlayer(game, req.PlayerID)
		if !ok {
			return errors.New("player not found")
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		if round.CurrentVote >= len(round.VoteTurns) {
			return errors.New("no active vote turn")
		}
		turn := round.VoteTurns[round.CurrentVote]
		if turn.VoterID != player.ID {
			return errors.New("not your turn")
		}
		for _, vote := range round.Votes {
			if vote.PlayerID == player.ID && vote.DrawingIndex == turn.DrawingIndex {
				return errors.New("vote already submitted")
			}
		}
		options := voteOptionsForDrawing(round, turn.DrawingIndex)
		if !containsOption(options, choiceText) {
			return errors.New("invalid vote option")
		}
		choiceType := "guess"
		if drawingPrompt(round, turn.DrawingIndex) == choiceText {
			choiceType = "prompt"
		}
		round.Votes = append(round.Votes, VoteEntry{
			PlayerID:     player.ID,
			DrawingIndex: turn.DrawingIndex,
			ChoiceText:   choiceText,
			ChoiceType:   choiceType,
		})
		round.CurrentVote++
		if round.CurrentVote >= len(round.VoteTurns) {
			setPhase(game, phaseResults)
			initReveal(round)
		}
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	turnIndex := 0
	if round := currentRound(game); round != nil && round.CurrentVote > 0 {
		turnIndex = round.CurrentVote - 1
	}
	drawingIndex := -1
	if round := currentRound(game); round != nil && turnIndex < len(round.VoteTurns) {
		drawingIndex = round.VoteTurns[turnIndex].DrawingIndex
	}
	choiceType := "guess"
	if round := currentRound(game); round != nil && drawingPrompt(round, drawingIndex) == choiceText {
		choiceType = "prompt"
	}
	if err := s.persistVote(game, req.PlayerID, drawingIndex, choiceText, choiceType); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save votes")
		return
	}
	if game.Phase == phaseResults {
		if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to advance game")
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("vote submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

func (s *Server) handleAdvance(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "advance") {
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		next, ok := nextPhase(game.Phase)
		if !ok {
			return errors.New("no next phase")
		}
		setPhase(game, next)
		if next == phaseGuesses {
			round := currentRound(game)
			if err := s.buildGuessTurns(game, round); err != nil {
				return err
			}
		}
		if next == phaseGuessVotes {
			round := currentRound(game)
			if err := s.buildVoteTurns(game, round); err != nil {
				return err
			}
		}
		if next == phaseResults {
			round := currentRound(game)
			initReveal(round)
		}
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to advance game")
		return
	}
	log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

func (s *Server) handleEndGame(w http.ResponseWriter, r *http.Request, gameID string) {
	if !s.enforceRateLimit(w, r, "end") {
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase == phaseComplete {
			return errors.New("game already ended")
		}
		setPhase(game, phaseComplete)
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.persistPhase(game, "game_ended", map[string]any{"phase": game.Phase}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to end game")
		return
	}
	log.Printf("game ended game_id=%s", game.ID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
	s.cancelPhaseTimer(game.ID)
}

func (s *Server) endGameFromHost(gameID string) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase == phaseComplete {
			return errors.New("game already ended")
		}
		setPhase(game, phaseComplete)
		return nil
	})
	if err != nil {
		return
	}
	if err := s.persistPhase(game, "game_ended", map[string]any{"phase": game.Phase}); err != nil {
		return
	}
	log.Printf("game ended game_id=%s reason=host_disconnected", game.ID)
	s.broadcastGameUpdate(game)
	s.cancelPhaseTimer(game.ID)
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request, gameID string) {
	game, ok := s.store.GetGame(gameID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	round := currentRound(game)
	promptsCount := 0
	drawingsCount := 0
	guessesCount := 0
	votesCount := 0
	if round != nil {
		promptsCount = len(round.Prompts)
		drawingsCount = len(round.Drawings)
		guessesCount = len(round.Guesses)
		votesCount = len(round.Votes)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game_id": game.ID,
		"phase":   game.Phase,
		"players": extractPlayerNames(game.Players),
		"results": buildResults(game),
		"scores":  buildScores(game),
		"counts": map[string]int{
			"prompts":  promptsCount,
			"drawings": drawingsCount,
			"guesses":  guessesCount,
			"votes":    votesCount,
		},
	})
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	gameID, ok := parseWebsocketPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, exists := s.store.GetGame(gameID); !exists {
		http.NotFound(w, r)
		return
	}
	role := r.URL.Query().Get("role")
	isHost := role == "host"
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	log.Printf("ws connected game_id=%s remote=%s", gameID, r.RemoteAddr)
	s.ws.Add(gameID, conn, isHost)
	if game, ok := s.store.GetGame(gameID); ok {
		s.ws.Send(conn, snapshot(game))
	}
	go s.readWS(gameID, conn, isHost)
}

func (s *Server) handleHomeWebsocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	log.Printf("ws connected home remote=%s", r.RemoteAddr)
	s.homeWS.Add(conn)
	s.homeWS.Send(conn, map[string]any{
		"games": s.homeSummaries(),
	})
	go s.readHomeWS(conn)
}

func (s *Server) handlePromptCategories(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, map[string]any{"categories": []string{}})
		return
	}
	var categories []string
	if err := s.db.Model(&db.PromptLibrary{}).Distinct("category").Order("category asc").Pluck("category", &categories).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load categories")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func parseGamePath(path string) (string, string, bool) {
	const prefix = "/api/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false
	}
	gameID := parts[0]
	if len(parts) == 1 {
		return gameID, "", true
	}
	if len(parts) == 2 {
		return gameID, parts[1], true
	}
	return "", "", false
}

func parseWebsocketPath(path string) (string, bool) {
	const prefix = "/ws/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return rest, true
}

func parseAudiencePath(path string) (string, string, bool) {
	const prefix = "/api/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 {
		return "", "", false
	}
	if parts[1] != "audience" {
		return "", "", false
	}
	if len(parts) == 2 {
		return parts[0], "", true
	}
	if len(parts) == 3 {
		return parts[0], parts[2], true
	}
	return "", "", false
}

func parseAudienceViewPath(path string) (string, int, bool) {
	const prefix = "/audience/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		return "", 0, false
	}
	gameID := parts[0]
	audienceID, err := strconv.Atoi(parts[1])
	if err != nil || audienceID <= 0 {
		return "", 0, false
	}
	return gameID, audienceID, true
}

func parseReplayPath(path string) (string, bool) {
	const prefix = "/replay/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	gameID := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if gameID == "" || strings.Contains(gameID, "/") {
		return "", false
	}
	return gameID, true
}

func parsePlayerPath(path string) (string, int, bool) {
	const prefix = "/play/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", 0, false
	}
	playerID, err := strconv.Atoi(parts[1])
	if err != nil || playerID <= 0 {
		return "", 0, false
	}
	return parts[0], playerID, true
}

func parsePlayerPromptPath(path string) (string, int, bool) {
	const prefix = "/api/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 4 {
		return "", 0, false
	}
	if parts[1] != "players" || parts[3] != "prompt" {
		return "", 0, false
	}
	playerID, err := strconv.Atoi(parts[2])
	if err != nil || playerID <= 0 {
		return "", 0, false
	}
	return parts[0], playerID, true
}

func nextPhase(current string) (string, bool) {
	for i, phase := range phaseOrder {
		if phase == current {
			if i+1 >= len(phaseOrder) {
				return "", false
			}
			return phaseOrder[i+1], true
		}
	}
	return "", false
}

func decodeImageData(data string) ([]byte, error) {
	if data == "" {
		return nil, errors.New("empty image")
	}
	parts := strings.SplitN(data, ",", 2)
	payload := data
	if len(parts) == 2 {
		payload = parts[1]
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func encodeImageData(image []byte) string {
	if len(image) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
}

func promptForPlayer(round *RoundState, playerID int) string {
	if round == nil {
		return ""
	}
	for _, entry := range round.Prompts {
		if entry.PlayerID == playerID {
			return entry.Text
		}
	}
	return ""
}

func findPromptForPlayer(round *RoundState, playerID int, promptText string) (PromptEntry, bool) {
	if round == nil {
		return PromptEntry{}, false
	}
	for _, entry := range round.Prompts {
		if entry.PlayerID == playerID {
			if promptText == "" || entry.Text == promptText {
				return entry, true
			}
		}
	}
	return PromptEntry{}, false
}

func drawingPrompt(round *RoundState, drawingIndex int) string {
	if round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return ""
	}
	return round.Drawings[drawingIndex].Prompt
}

func voteOptionsForDrawing(round *RoundState, drawingIndex int) []string {
	if round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return nil
	}
	seen := make(map[string]struct{})
	options := make([]string, 0, 1+len(round.Guesses))
	prompt := round.Drawings[drawingIndex].Prompt
	if prompt != "" {
		seen[prompt] = struct{}{}
		options = append(options, prompt)
	}
	for _, guess := range round.Guesses {
		if guess.DrawingIndex != drawingIndex {
			continue
		}
		if _, ok := seen[guess.Text]; ok {
			continue
		}
		seen[guess.Text] = struct{}{}
		options = append(options, guess.Text)
	}
	return options
}

func containsOption(options []string, choice string) bool {
	for _, option := range options {
		if option == choice {
			return true
		}
	}
	return false
}

func guessOwner(round *RoundState, drawingIndex int, text string) int {
	if round == nil {
		return 0
	}
	for _, guess := range round.Guesses {
		if guess.DrawingIndex == drawingIndex && guess.Text == text {
			return guess.PlayerID
		}
	}
	return 0
}

func pickPlayerColor(index int) string {
	palette := []string{
		"#ff6b6b",
		"#4dabf7",
		"#51cf66",
		"#ffa94d",
		"#ffd43b",
		"#845ef7",
		"#20c997",
		"#e64980",
	}
	if len(palette) == 0 {
		return "#1a1a1a"
	}
	if index < 0 {
		index = 0
	}
	return palette[index%len(palette)]
}

type wsHub struct {
	mu     sync.Mutex
	groups map[string]map[*websocket.Conn]struct{}
	hosts  map[string]map[*websocket.Conn]struct{}
}

type homeHub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		groups: make(map[string]map[*websocket.Conn]struct{}),
		hosts:  make(map[string]map[*websocket.Conn]struct{}),
	}
}

func newHomeHub() *homeHub {
	return &homeHub{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

func (h *wsHub) Add(gameID string, conn *websocket.Conn, isHost bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	group := h.groups[gameID]
	if group == nil {
		group = make(map[*websocket.Conn]struct{})
		h.groups[gameID] = group
	}
	group[conn] = struct{}{}
	if isHost {
		hostGroup := h.hosts[gameID]
		if hostGroup == nil {
			hostGroup = make(map[*websocket.Conn]struct{})
			h.hosts[gameID] = hostGroup
		}
		hostGroup[conn] = struct{}{}
	}
}

func (h *homeHub) Add(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn] = struct{}{}
}

func (h *homeHub) Remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, conn)
	_ = conn.Close()
}

func (h *homeHub) Send(conn *websocket.Conn, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (h *homeHub) Broadcast(payload any) {
	h.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for conn := range h.conns {
		conns = append(conns, conn)
	}
	h.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.Remove(conn)
		}
	}
}

func (h *wsHub) Remove(gameID string, conn *websocket.Conn, isHost bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	group := h.groups[gameID]
	if group == nil {
		return
	}
	delete(group, conn)
	_ = conn.Close()
	if len(group) == 0 {
		delete(h.groups, gameID)
	}
	if isHost {
		hostGroup := h.hosts[gameID]
		if hostGroup != nil {
			delete(hostGroup, conn)
			if len(hostGroup) == 0 {
				delete(h.hosts, gameID)
			}
		}
	}
}

func (h *wsHub) Send(conn *websocket.Conn, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (h *wsHub) Broadcast(gameID string, payload any) {
	h.mu.Lock()
	group := h.groups[gameID]
	conns := make([]*websocket.Conn, 0, len(group))
	for conn := range group {
		conns = append(conns, conn)
	}
	h.mu.Unlock()

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.Remove(gameID, conn, false)
		}
	}
}

func snapshot(game *Game) map[string]any {
	round := currentRound(game)
	promptsCount := 0
	drawingsCount := 0
	guessesCount := 0
	votesCount := 0
	roundNumber := 0
	var guessTurn map[string]any
	var voteTurn map[string]any
	var results any
	var scores any
	var reveal any
	if round != nil {
		roundNumber = round.Number
		promptsCount = len(round.Prompts)
		drawingsCount = len(round.Drawings)
		guessesCount = len(round.Guesses)
		votesCount = len(round.Votes)
		if round.CurrentGuess < len(round.GuessTurns) {
			turn := round.GuessTurns[round.CurrentGuess]
			guessTurn = map[string]any{
				"drawing_index": turn.DrawingIndex,
				"guesser_id":    turn.GuesserID,
			}
			if turn.DrawingIndex >= 0 && turn.DrawingIndex < len(round.Drawings) {
				drawing := round.Drawings[turn.DrawingIndex]
				guessTurn["drawing_owner"] = drawing.PlayerID
				guessTurn["drawing_image"] = encodeImageData(drawing.ImageData)
			}
		}
		if round.CurrentVote < len(round.VoteTurns) {
			turn := round.VoteTurns[round.CurrentVote]
			voteTurn = map[string]any{
				"drawing_index": turn.DrawingIndex,
				"voter_id":      turn.VoterID,
			}
			if turn.DrawingIndex >= 0 && turn.DrawingIndex < len(round.Drawings) {
				drawing := round.Drawings[turn.DrawingIndex]
				voteTurn["drawing_owner"] = drawing.PlayerID
				voteTurn["drawing_image"] = encodeImageData(drawing.ImageData)
				voteTurn["options"] = voteOptionsForDrawing(round, turn.DrawingIndex)
			}
		}
		if game.Phase == phaseResults || game.Phase == phaseComplete {
			results = buildResults(game)
			scores = buildScores(game)
			if game.Phase == phaseResults {
				reveal = buildReveal(game)
			}
		}
	}
	return map[string]any{
		"game_id":            game.ID,
		"join_code":          game.JoinCode,
		"phase":              game.Phase,
		"host_id":            game.HostID,
		"max_players":        game.MaxPlayers,
		"prompt_category":    game.PromptCategory,
		"lobby_locked":       game.LobbyLocked,
		"can_join":           game.Phase == phaseLobby && !game.LobbyLocked && (game.MaxPlayers == 0 || len(game.Players) < game.MaxPlayers),
		"players":            extractPlayerNames(game.Players),
		"player_ids":         extractPlayerIDs(game.Players),
		"player_colors":      extractPlayerColors(game.Players),
		"audience_count":     len(game.Audience),
		"round_number":       roundNumber,
		"total_rounds":       game.PromptsPerPlayer,
		"guess_turn":         guessTurn,
		"vote_turn":          voteTurn,
		"prompts_per_player": game.PromptsPerPlayer,
		"results":            results,
		"scores":             scores,
		"reveal":             reveal,
		"audience_options":   buildAudienceOptions(game),
		"counts": map[string]int{
			"prompts":  promptsCount,
			"drawings": drawingsCount,
			"guesses":  guessesCount,
			"votes":    votesCount,
		},
	}
}

func extractPlayerNames(players []Player) []string {
	names := make([]string, 0, len(players))
	for _, player := range players {
		names = append(names, player.Name)
	}
	return names
}

func extractPlayerColors(players []Player) map[int]string {
	colors := make(map[int]string, len(players))
	for _, player := range players {
		if player.Color == "" {
			continue
		}
		colors[player.ID] = player.Color
	}
	return colors
}

func extractPlayerIDs(players []Player) []int {
	ids := make([]int, 0, len(players))
	for _, player := range players {
		ids = append(ids, player.ID)
	}
	return ids
}

func buildResults(game *Game) []map[string]any {
	round := currentRound(game)
	if round == nil {
		return nil
	}
	playerNames := make(map[int]string, len(game.Players))
	for _, player := range game.Players {
		playerNames[player.ID] = player.Name
	}
	results := make([]map[string]any, 0, len(round.Drawings))
	for drawingIndex, drawing := range round.Drawings {
		guesses := make([]map[string]any, 0)
		for _, guess := range round.Guesses {
			if guess.DrawingIndex != drawingIndex {
				continue
			}
			guesses = append(guesses, map[string]any{
				"player_id":   guess.PlayerID,
				"player_name": playerNames[guess.PlayerID],
				"text":        guess.Text,
			})
		}
		votes := make([]map[string]any, 0)
		for _, vote := range round.Votes {
			if vote.DrawingIndex != drawingIndex {
				continue
			}
			votes = append(votes, map[string]any{
				"player_id":   vote.PlayerID,
				"player_name": playerNames[vote.PlayerID],
				"text":        vote.ChoiceText,
				"type":        vote.ChoiceType,
			})
		}
		results = append(results, map[string]any{
			"drawing_index":      drawingIndex,
			"drawing_owner":      drawing.PlayerID,
			"drawing_owner_name": playerNames[drawing.PlayerID],
			"drawing_image":      encodeImageData(drawing.ImageData),
			"prompt":             drawing.Prompt,
			"guesses":            guesses,
			"votes":              votes,
		})
	}
	return results
}

func buildScores(game *Game) []map[string]any {
	round := currentRound(game)
	if round == nil {
		return nil
	}
	playerNames := make(map[int]string, len(game.Players))
	scores := make(map[int]int, len(game.Players))
	for _, player := range game.Players {
		playerNames[player.ID] = player.Name
		scores[player.ID] = 0
	}

	for drawingIndex, drawing := range round.Drawings {
		totalVotes := 0
		correctVotes := 0
		fooledVotes := 0
		for _, vote := range round.Votes {
			if vote.DrawingIndex != drawingIndex {
				continue
			}
			totalVotes++
			if vote.ChoiceType == "prompt" {
				scores[vote.PlayerID] += 1000
				correctVotes++
			} else {
				if ownerID := guessOwner(round, drawingIndex, vote.ChoiceText); ownerID != 0 {
					scores[ownerID] += 500
				}
				fooledVotes++
			}
		}
		if fooledVotes > 0 {
			scores[drawing.PlayerID] += 500 * fooledVotes
		}
		if totalVotes > 0 && (correctVotes == 0 || correctVotes == totalVotes) {
			scores[drawing.PlayerID] += 1000
		}
	}

	entries := make([]map[string]any, 0, len(game.Players))
	for _, player := range game.Players {
		entries = append(entries, map[string]any{
			"player_id":   player.ID,
			"player_name": playerNames[player.ID],
			"score":       scores[player.ID],
		})
	}
	return entries
}

func buildReveal(game *Game) map[string]any {
	round := currentRound(game)
	if round == nil || len(round.Drawings) == 0 {
		return nil
	}
	if round.RevealStage == "" {
		initReveal(round)
	}
	if round.RevealIndex < 0 || round.RevealIndex >= len(round.Drawings) {
		return nil
	}
	playerNames := make(map[int]string, len(game.Players))
	for _, player := range game.Players {
		playerNames[player.ID] = player.Name
	}
	drawing := round.Drawings[round.RevealIndex]
	payload := map[string]any{
		"index":              round.RevealIndex,
		"stage":              round.RevealStage,
		"total":              len(round.Drawings),
		"drawing_owner":      drawing.PlayerID,
		"drawing_owner_name": playerNames[drawing.PlayerID],
		"drawing_image":      encodeImageData(drawing.ImageData),
	}
	if round.RevealStage == revealStageGuesses {
		guesses := make([]map[string]any, 0)
		for _, guess := range round.Guesses {
			if guess.DrawingIndex != round.RevealIndex {
				continue
			}
			guesses = append(guesses, map[string]any{
				"player_id":   guess.PlayerID,
				"player_name": playerNames[guess.PlayerID],
				"text":        guess.Text,
			})
		}
		payload["guesses"] = guesses
	} else if round.RevealStage == revealStageVotes {
		votes := make([]map[string]any, 0)
		for _, vote := range round.Votes {
			if vote.DrawingIndex != round.RevealIndex {
				continue
			}
			votes = append(votes, map[string]any{
				"player_id":   vote.PlayerID,
				"player_name": playerNames[vote.PlayerID],
				"text":        vote.ChoiceText,
				"type":        vote.ChoiceType,
			})
		}
		payload["prompt"] = drawing.Prompt
		payload["votes"] = votes
	}
	return payload
}

func buildAudienceOptions(game *Game) []map[string]any {
	if game.Phase != phaseGuessVotes {
		return nil
	}
	round := currentRound(game)
	if round == nil {
		return nil
	}
	options := make([]map[string]any, 0, len(round.Drawings))
	for index, drawing := range round.Drawings {
		entry := map[string]any{
			"drawing_index": index,
			"drawing_image": encodeImageData(drawing.ImageData),
			"options":       voteOptionsForDrawing(round, index),
		}
		options = append(options, entry)
	}
	return options
}

func currentRound(game *Game) *RoundState {
	if len(game.Rounds) == 0 {
		return nil
	}
	return &game.Rounds[len(game.Rounds)-1]
}

func setPhase(game *Game, phase string) {
	game.Phase = phase
	game.PhaseStartedAt = time.Now().UTC()
}

func initReveal(round *RoundState) {
	if round == nil {
		return
	}
	round.RevealIndex = 0
	round.RevealStage = revealStageGuesses
}

func (s *Server) buildGuessTurns(game *Game, round *RoundState) error {
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Drawings) == 0 {
		return errors.New("no drawings submitted")
	}
	if len(round.GuessTurns) > 0 {
		return nil
	}
	round.GuessTurns = nil
	round.CurrentGuess = 0
	for drawingIndex, drawing := range round.Drawings {
		for _, player := range game.Players {
			if player.ID == drawing.PlayerID {
				continue
			}
			round.GuessTurns = append(round.GuessTurns, GuessTurn{
				DrawingIndex: drawingIndex,
				GuesserID:    player.ID,
			})
		}
	}
	if len(round.GuessTurns) == 0 {
		return errors.New("no guess turns available")
	}
	return nil
}

func (s *Server) buildVoteTurns(game *Game, round *RoundState) error {
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Drawings) == 0 {
		return errors.New("no drawings submitted")
	}
	if len(round.VoteTurns) > 0 {
		return nil
	}
	round.VoteTurns = nil
	round.CurrentVote = 0
	for drawingIndex, drawing := range round.Drawings {
		for _, player := range game.Players {
			if player.ID == drawing.PlayerID {
				continue
			}
			round.VoteTurns = append(round.VoteTurns, VoteTurn{
				DrawingIndex: drawingIndex,
				VoterID:      player.ID,
			})
		}
	}
	if len(round.VoteTurns) == 0 {
		return errors.New("no vote turns available")
	}
	return nil
}

func drawingsComplete(game *Game) bool {
	if game == nil {
		return false
	}
	round := currentRound(game)
	if round == nil {
		return false
	}
	return len(round.Drawings) >= len(game.Players) && len(game.Players) > 0
}

func (s *Server) tryAdvanceToGuesses(gameID string) (bool, *Game, error) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseDrawings {
			return nil
		}
		if !drawingsComplete(game) {
			return nil
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		if err := s.buildGuessTurns(game, round); err != nil {
			return err
		}
		setPhase(game, phaseGuesses)
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	if game.Phase != phaseGuesses {
		return false, game, nil
	}
	s.schedulePhaseTimer(game)
	return true, game, nil
}

type rateLimitRule struct {
	Capacity int
	Window   time.Duration
}

var rateLimitRules = map[string]rateLimitRule{
	"create":        {Capacity: 5, Window: time.Minute},
	"join":          {Capacity: 10, Window: time.Minute},
	"audience_join": {Capacity: 10, Window: time.Minute},
	"audience_vote": {Capacity: 30, Window: time.Minute},
	"settings":      {Capacity: 10, Window: time.Minute},
	"kick":          {Capacity: 10, Window: time.Minute},
	"rename":        {Capacity: 10, Window: time.Minute},
	"start":         {Capacity: 5, Window: time.Minute},
	"advance":       {Capacity: 10, Window: time.Minute},
	"end":           {Capacity: 5, Window: time.Minute},
	"drawings":      {Capacity: 30, Window: time.Minute},
	"guesses":       {Capacity: 30, Window: time.Minute},
	"votes":         {Capacity: 30, Window: time.Minute},
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	now     func() time.Time
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		now:     time.Now,
	}
}

func (r *rateLimiter) allow(key string, capacity int, window time.Duration) bool {
	if capacity <= 0 || window <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	bucket, ok := r.buckets[key]
	if !ok {
		r.buckets[key] = &tokenBucket{
			tokens: float64(capacity - 1),
			last:   now,
		}
		return true
	}
	elapsed := now.Sub(bucket.last).Seconds()
	refill := (float64(capacity) / window.Seconds()) * elapsed
	bucket.tokens = minFloat(float64(capacity), bucket.tokens+refill)
	bucket.last = now
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func (s *Server) enforceRateLimit(w http.ResponseWriter, r *http.Request, action string) bool {
	if s.limiter == nil {
		return true
	}
	rule, ok := rateLimitRules[action]
	if !ok {
		return true
	}
	key := action + ":" + clientKey(r)
	if s.limiter.allow(key, rule.Capacity, rule.Window) {
		return true
	}
	writeError(w, http.StatusTooManyRequests, rateLimitExceeded)
	return false
}

func clientKey(r *http.Request) string {
	if cookie, err := r.Cookie("pt_session"); err == nil && cookie.Value != "" {
		return "session:" + cookie.Value
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return strings.TrimSpace(parts[0])
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func validateName(name string) (string, error) {
	return validateText("name", name, maxNameLength)
}

func validateGuess(text string) (string, error) {
	return validateText("guess", text, maxGuessLength)
}

func validatePrompt(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", errors.New("prompt is required")
	}
	if len(trimmed) > maxPromptLength {
		return "", fmt.Errorf("prompt must be %d characters or fewer", maxPromptLength)
	}
	if !isSafeText(trimmed) {
		return "", errors.New("prompt contains unsupported characters")
	}
	return trimmed, nil
}

func validateChoice(text string) (string, error) {
	return validateText("choice", text, maxChoiceLength)
}

func validateCategory(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > maxCategoryLength {
		return "", fmt.Errorf("prompt category must be %d characters or fewer", maxCategoryLength)
	}
	for _, r := range trimmed {
		if r > 127 {
			return "", errors.New("prompt category contains unsupported characters")
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' {
			continue
		}
		return "", errors.New("prompt category contains unsupported characters")
	}
	return trimmed, nil
}

func validateText(label, text string, maxLen int) (string, error) {
	trimmed := normalizeText(text)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if len(trimmed) > maxLen {
		return "", fmt.Errorf("%s must be %d characters or fewer", label, maxLen)
	}
	if !isSafeText(trimmed) {
		return "", fmt.Errorf("%s contains unsupported characters", label)
	}
	return trimmed, nil
}

func normalizeText(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	return strings.Join(fields, " ")
}

func isSafeText(text string) bool {
	for _, r := range text {
		if r > 127 {
			return false
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case ' ', '-', '_', '\'', '"', '.', ',', '!', '?', ':', ';', '&', '(', ')', '/':
			continue
		default:
			return false
		}
	}
	return true
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func readJSON(body io.Reader, dest any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func (s *Server) persistGame(game *Game) error {
	if s.db == nil {
		return nil
	}
	record := db.Game{
		JoinCode:         game.JoinCode,
		Phase:            game.Phase,
		PromptsPerPlayer: game.PromptsPerPlayer,
		MaxPlayers:       game.MaxPlayers,
		PromptCategory:   game.PromptCategory,
		LobbyLocked:      game.LobbyLocked,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	game.DBID = record.ID
	newID := fmt.Sprintf("game-%d", record.ID)
	if game.ID != newID {
		s.store.UpdateGameID(game, newID)
	}
	return s.persistEvent(game, "game_created", map[string]any{
		"game_id":   game.ID,
		"join_code": game.JoinCode,
	})
}

func (s *Server) persistPlayer(game *Game, player *Player) (int, error) {
	if s.db == nil {
		return player.ID, nil
	}
	if player.DBID != 0 {
		return player.ID, nil
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			return 0, err
		}
		if game.DBID == 0 {
			return 0, errors.New("game not found")
		}
	}
	record := db.Player{
		GameID:   game.DBID,
		Name:     player.Name,
		IsHost:   player.IsHost,
		JoinedAt: time.Now().UTC(),
	}
	if err := s.db.Create(&record).Error; err != nil {
		if isUniqueViolation(err) {
			existing, lookupErr := s.findPlayerDBID(game.DBID, player.Name)
			if lookupErr == nil && existing != 0 {
				player.DBID = existing
				return player.ID, nil
			}
		}
		return 0, err
	}
	player.DBID = record.ID
	if err := s.persistEvent(game, "player_joined", map[string]any{"player": player.Name}); err != nil {
		return player.ID, err
	}
	return player.ID, nil
}

func (s *Server) persistPhase(game *Game, eventType string, payload map[string]any) error {
	if s.db == nil {
		return nil
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			return err
		}
	}
	if game.DBID == 0 {
		return errors.New("game not found")
	}
	if err := s.db.Model(&db.Game{}).Where("id = ?", game.DBID).Update("phase", game.Phase).Error; err != nil {
		return err
	}
	if round := currentRound(game); round != nil && round.DBID != 0 {
		if err := s.db.Model(&db.Round{}).Where("id = ?", round.DBID).Update("status", game.Phase).Error; err != nil {
			return err
		}
	}
	return s.persistEvent(game, eventType, payload)
}

func (s *Server) persistSettings(game *Game) error {
	if s.db == nil {
		return nil
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			return err
		}
	}
	if game.DBID == 0 {
		return errors.New("game not found")
	}
	updates := map[string]any{
		"prompts_per_player": game.PromptsPerPlayer,
		"max_players":        game.MaxPlayers,
		"prompt_category":    game.PromptCategory,
		"lobby_locked":       game.LobbyLocked,
	}
	if err := s.db.Model(&db.Game{}).Where("id = ?", game.DBID).Updates(updates).Error; err != nil {
		return err
	}
	return s.persistEvent(game, "settings_updated", updates)
}

func (s *Server) persistEvent(game *Game, eventType string, payload map[string]any) error {
	if s.db == nil {
		return nil
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			return err
		}
	}
	if game.DBID == 0 {
		return errors.New("game not found")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := db.Event{
		GameID:  game.DBID,
		Type:    eventType,
		Payload: datatypes.JSON(data),
	}
	return s.db.Create(&event).Error
}

func (s *Server) ensureGameDBID(game *Game) error {
	if s.db == nil || game.DBID != 0 {
		return nil
	}
	var record db.Game
	if err := s.db.Where("join_code = ?", game.JoinCode).First(&record).Error; err != nil {
		return nil
	}
	game.DBID = record.ID
	return nil
}

func (s *Server) persistRound(game *Game) error {
	if s.db == nil {
		return nil
	}
	round := currentRound(game)
	if round == nil || round.DBID != 0 {
		return nil
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			return err
		}
	}
	if game.DBID == 0 {
		return errors.New("game not found")
	}
	record := db.Round{
		GameID: game.DBID,
		Number: round.Number,
		Status: game.Phase,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	round.DBID = record.ID
	return nil
}

func (s *Server) assignPrompts(game *Game) error {
	round := currentRound(game)
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Prompts) > 0 {
		return nil
	}
	if game.UsedPrompts == nil {
		game.UsedPrompts = make(map[string]struct{})
	}
	total := len(game.Players)
	if total == 0 {
		return errors.New("no players to assign prompts")
	}

	prompts, err := s.loadPromptLibrary(total, game.PromptCategory, game.UsedPrompts)
	if err != nil {
		return err
	}
	if len(prompts) < total {
		return errors.New("not enough prompts available")
	}

	idx := 0
	for _, player := range game.Players {
		prompt := prompts[idx]
		round.Prompts = append(round.Prompts, PromptEntry{
			PlayerID: player.ID,
			Text:     prompt,
		})
		game.UsedPrompts[prompt] = struct{}{}
		idx++
	}
	if err := s.persistAssignedPrompts(game, round); err != nil {
		return err
	}
	if err := s.persistEvent(game, "prompts_assigned", map[string]any{
		"count": total,
	}); err != nil {
		return err
	}
	log.Printf("prompts assigned game_id=%s count=%d", game.ID, total)
	return nil
}

func (s *Server) loadPromptLibrary(limit int, category string, used map[string]struct{}) ([]string, error) {
	if s.db == nil {
		return selectPrompts(fallbackPromptsList(), limit, used), nil
	}
	var records []db.PromptLibrary
	query := s.db
	if category = strings.TrimSpace(category); category != "" {
		query = query.Where("category = ?", category)
	}
	if len(used) > 0 {
		exclusions := make([]string, 0, len(used))
		for prompt := range used {
			exclusions = append(exclusions, prompt)
		}
		query = query.Where("text NOT IN ?", exclusions)
	}
	if err := query.Order("random()").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	prompts := make([]string, 0, len(records))
	for _, record := range records {
		prompts = append(prompts, record.Text)
	}
	return prompts, nil
}

func fallbackPromptsList() []string {
	return []string{
		"Draw a llama in a suit",
		"Draw a castle made of pancakes",
		"Draw a robot learning to dance",
		"Draw a pirate cat at a tea party",
		"Draw a rocket powered skateboard",
		"Draw a haunted treehouse",
		"Draw a snowy beach day",
		"Draw a giant sunflower city",
	}
}

func selectPrompts(pool []string, limit int, used map[string]struct{}) []string {
	if limit <= 0 {
		return nil
	}
	selected := make([]string, 0, limit)
	for _, prompt := range pool {
		if len(selected) >= limit {
			break
		}
		if _, ok := used[prompt]; ok {
			continue
		}
		selected = append(selected, prompt)
	}
	return selected
}

func (s *Server) persistAssignedPrompts(game *Game, round *RoundState) error {
	if s.db == nil {
		return nil
	}
	if round.DBID == 0 {
		if err := s.persistRound(game); err != nil {
			return err
		}
	}
	for i := range round.Prompts {
		entry := &round.Prompts[i]
		if entry.DBID != 0 {
			continue
		}
		player, ok := s.store.FindPlayer(game, entry.PlayerID)
		if !ok || player.DBID == 0 {
			return errors.New("player not found")
		}
		record := db.Prompt{
			RoundID:  round.DBID,
			PlayerID: player.DBID,
			Text:     entry.Text,
		}
		if err := s.db.Create(&record).Error; err != nil {
			return err
		}
		entry.DBID = record.ID
	}
	return nil
}

func (s *Server) persistDrawing(game *Game, playerID int, image []byte, promptText string) error {
	if s.db == nil {
		return s.persistEvent(game, "drawings_submitted", map[string]any{
			"player_id": playerID,
			"prompt":    promptText,
		})
	}
	round := currentRound(game)
	if round == nil {
		return errors.New("round not started")
	}
	if round.DBID == 0 {
		if err := s.persistRound(game); err != nil {
			return err
		}
	}
	player, ok := s.store.FindPlayer(game, playerID)
	if !ok || player.DBID == 0 {
		return errors.New("player not found")
	}
	promptEntry, ok := findPromptForPlayer(round, playerID, promptText)
	if !ok || promptEntry.DBID == 0 {
		return errors.New("prompt not assigned")
	}
	record := db.Drawing{
		RoundID:   round.DBID,
		PlayerID:  player.DBID,
		PromptID:  promptEntry.DBID,
		ImageData: image,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	s.store.UpdateGame(game.ID, func(game *Game) error {
		round := currentRound(game)
		if round == nil {
			return nil
		}
		for i := range round.Drawings {
			if round.Drawings[i].PlayerID == playerID {
				round.Drawings[i].DBID = record.ID
				break
			}
		}
		return nil
	})
	return s.persistEvent(game, "drawings_submitted", map[string]any{
		"player_id": playerID,
	})
}

func (s *Server) persistGuess(game *Game, playerID int, drawingIndex int, guess string) error {
	if s.db == nil {
		return s.persistEvent(game, "guesses_submitted", map[string]any{
			"player_id": playerID,
			"guess":     guess,
		})
	}
	round := currentRound(game)
	if round == nil {
		return errors.New("round not started")
	}
	if round.DBID == 0 {
		if err := s.persistRound(game); err != nil {
			return err
		}
	}
	player, ok := s.store.FindPlayer(game, playerID)
	if !ok || player.DBID == 0 {
		return errors.New("player not found")
	}
	drawingID := uint(0)
	if drawingIndex >= 0 && drawingIndex < len(round.Drawings) {
		drawingID = round.Drawings[drawingIndex].DBID
	}
	record := db.Guess{
		RoundID:   round.DBID,
		PlayerID:  player.DBID,
		DrawingID: drawingID,
		Text:      guess,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	return s.persistEvent(game, "guesses_submitted", map[string]any{
		"player_id": playerID,
		"guess":     guess,
	})
}

func (s *Server) persistVote(game *Game, playerID int, drawingIndex int, choiceText string, choiceType string) error {
	if s.db == nil {
		return s.persistEvent(game, "votes_submitted", map[string]any{
			"player_id": playerID,
			"choice":    choiceText,
		})
	}
	round := currentRound(game)
	if round == nil {
		return errors.New("round not started")
	}
	if round.DBID == 0 {
		if err := s.persistRound(game); err != nil {
			return err
		}
	}
	player, ok := s.store.FindPlayer(game, playerID)
	if !ok || player.DBID == 0 {
		return errors.New("player not found")
	}
	drawingID := uint(0)
	if drawingIndex >= 0 && drawingIndex < len(round.Drawings) {
		drawingID = round.Drawings[drawingIndex].DBID
	}
	record := db.Vote{
		RoundID:    round.DBID,
		PlayerID:   player.DBID,
		DrawingID:  drawingID,
		GuessID:    0,
		ChoiceText: choiceText,
		ChoiceType: choiceType,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	return s.persistEvent(game, "votes_submitted", map[string]any{
		"player_id": playerID,
		"choice":    choiceText,
	})
}

func (s *Server) findPlayerDBID(gameDBID uint, name string) (uint, error) {
	var record db.Player
	if err := s.db.Where("game_id = ? AND name = ?", gameDBID, name).First(&record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

type sessionStore struct {
	db       *gorm.DB
	mu       sync.Mutex
	sessions map[string]sessionData
}

type sessionData struct {
	Flash string
	Name  string
}

func newSessionStore(conn *gorm.DB) *sessionStore {
	return &sessionStore{
		db:       conn,
		sessions: make(map[string]sessionData),
	}
}

func (s *sessionStore) SetFlash(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		return
	}
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		data := s.sessions[id]
		data.Flash = message
		s.sessions[id] = data
		s.mu.Unlock()
		return
	}
	record := db.Session{
		ID:    id,
		Flash: message,
	}
	_ = s.db.Save(&record).Error
}

func (s *sessionStore) PopFlash(w http.ResponseWriter, r *http.Request) string {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		data := s.sessions[id]
		message := data.Flash
		data.Flash = ""
		s.sessions[id] = data
		return message
	}
	var record db.Session
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return ""
	}
	if record.Flash == "" {
		return ""
	}
	message := record.Flash
	record.Flash = ""
	_ = s.db.Save(&record).Error
	return message
}

func (s *sessionStore) SetName(w http.ResponseWriter, r *http.Request, name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		data := s.sessions[id]
		data.Name = name
		s.sessions[id] = data
		s.mu.Unlock()
		return
	}
	record := db.Session{
		ID:         id,
		PlayerName: name,
	}
	_ = s.db.Save(&record).Error
}

func (s *sessionStore) GetName(w http.ResponseWriter, r *http.Request) string {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.sessions[id].Name
	}
	var record db.Session
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return ""
	}
	return record.PlayerName
}

func (s *sessionStore) ensureSessionID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("pt_session")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	id := newSessionID()
	http.SetCookie(w, &http.Cookie{
		Name:     "pt_session",
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return id
}

func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buf)
}

func (s *Server) broadcastGameUpdate(game *Game) {
	if s.ws == nil {
		return
	}
	s.ws.Broadcast(game.ID, snapshot(game))
	s.broadcastHomeUpdate()
}

func (s *Server) schedulePhaseTimer(game *Game) {
	duration := s.phaseDuration(game.Phase)
	if duration <= 0 {
		s.cancelPhaseTimer(game.ID)
		return
	}
	s.timersMu.Lock()
	if existing, ok := s.timers[game.ID]; ok {
		existing.Stop()
	}
	timer := time.AfterFunc(duration, func() {
		s.autoAdvancePhase(game.ID, game.Phase)
	})
	s.timers[game.ID] = timer
	s.timersMu.Unlock()
}

func (s *Server) cancelPhaseTimer(gameID string) {
	s.timersMu.Lock()
	defer s.timersMu.Unlock()
	if timer, ok := s.timers[gameID]; ok {
		timer.Stop()
		delete(s.timers, gameID)
	}
}

func (s *Server) phaseDuration(phase string) time.Duration {
	switch phase {
	case phaseDrawings:
		return time.Duration(s.cfg.DrawDurationSeconds) * time.Second
	case phaseGuesses:
		return time.Duration(s.cfg.GuessDurationSeconds) * time.Second
	case phaseGuessVotes:
		return time.Duration(s.cfg.VoteDurationSeconds) * time.Second
	case phaseResults:
		return time.Duration(s.cfg.RevealDurationSeconds) * time.Second
	default:
		return 0
	}
}

func (s *Server) readWS(gameID string, conn *websocket.Conn, isHost bool) {
	defer s.ws.Remove(gameID, conn, isHost)
	if isHost {
		defer s.endGameFromHost(gameID)
	}
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("ws disconnected game_id=%s error=%v", gameID, err)
			return
		}
	}
}

func (s *Server) readHomeWS(conn *websocket.Conn) {
	defer s.homeWS.Remove(conn)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("home ws disconnected error=%v", err)
			return
		}
	}
}

func (s *Server) autoAdvancePhase(gameID string, expectedPhase string) {
	now := time.Now().UTC()
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != expectedPhase {
			return errors.New("phase changed")
		}
		switch expectedPhase {
		case phaseDrawings:
			round := currentRound(game)
			if round == nil {
				return errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				game.Phase = phaseComplete
				game.PhaseStartedAt = now
				return nil
			}
			if err := s.buildGuessTurns(game, round); err != nil {
				return err
			}
			game.Phase = phaseGuesses
			game.PhaseStartedAt = now
		case phaseGuesses:
			round := currentRound(game)
			if round == nil {
				return errors.New("round not started")
			}
			round.CurrentGuess = len(round.GuessTurns)
			if round.Number < game.PromptsPerPlayer {
				game.Phase = phaseDrawings
				game.PhaseStartedAt = now
				game.Rounds = append(game.Rounds, RoundState{
					Number: len(game.Rounds) + 1,
				})
			} else {
				if err := s.buildVoteTurns(game, round); err != nil {
					return err
				}
				game.Phase = phaseGuessVotes
				game.PhaseStartedAt = now
			}
		case phaseGuessVotes:
			round := currentRound(game)
			if round == nil {
				return errors.New("round not started")
			}
			round.CurrentVote = len(round.VoteTurns)
			game.Phase = phaseResults
			game.PhaseStartedAt = now
			initReveal(round)
		case phaseResults:
			round := currentRound(game)
			if round == nil {
				return errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				game.Phase = phaseComplete
				game.PhaseStartedAt = now
				return nil
			}
			if round.RevealStage == "" {
				initReveal(round)
			} else if round.RevealStage == revealStageGuesses {
				round.RevealStage = revealStageVotes
			} else if round.RevealStage == revealStageVotes {
				round.RevealIndex++
				if round.RevealIndex >= len(round.Drawings) {
					game.Phase = phaseComplete
					game.PhaseStartedAt = now
					return nil
				}
				round.RevealStage = revealStageGuesses
			}
		default:
			return errors.New("phase not timed")
		}
		return nil
	})
	if err != nil {
		return
	}
	if game.Phase == phaseDrawings && expectedPhase == phaseGuesses {
		if err := s.persistRound(game); err != nil {
			log.Printf("auto-advance persist round failed game_id=%s error=%v", game.ID, err)
			return
		}
		if err := s.assignPrompts(game); err != nil {
			log.Printf("auto-advance assign prompts failed game_id=%s error=%v", game.ID, err)
			return
		}
	}
	if game.Phase != expectedPhase {
		if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase, "reason": "timeout"}); err != nil {
			log.Printf("auto-advance persist phase failed game_id=%s error=%v", game.ID, err)
			return
		}
		log.Printf("game auto-advanced game_id=%s from=%s to=%s", game.ID, expectedPhase, game.Phase)
	}
	if game.Phase == phaseComplete {
		s.cancelPhaseTimer(game.ID)
	} else {
		s.schedulePhaseTimer(game)
	}
	s.broadcastGameUpdate(game)
}

func (s *Server) broadcastHomeUpdate() {
	if s.homeWS == nil {
		return
	}
	s.homeWS.Broadcast(map[string]any{
		"games": s.homeSummaries(),
	})
}

func (s *Server) homeSummaries() []web.GameSummary {
	summaries := make([]web.GameSummary, 0)
	for _, game := range s.store.ListGameSummaries() {
		summaries = append(summaries, web.GameSummary{
			ID:       game.ID,
			JoinCode: game.JoinCode,
			Phase:    game.Phase,
			Players:  game.Players,
		})
	}
	return summaries
}
