package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"picture-this/internal/db"
	"picture-this/internal/web"

	"github.com/a-h/templ"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Server struct {
	store *Store
	db    *gorm.DB
}

func New(conn *gorm.DB) *Server {
	return &Server{
		store: NewStore(),
		db:    conn,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /", templ.Handler(web.Home()))
	mux.HandleFunc("POST /api/games", s.handleCreateGame)
	mux.HandleFunc("GET /api/games/", s.handleGameSubroutes)
	mux.HandleFunc("POST /api/games/", s.handleGameSubroutes)
	mux.HandleFunc("GET /ws/games/", s.handleWebsocket)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return mux
}

type Store struct {
	mu     sync.Mutex
	nextID int
	games  map[string]*Game
}

type Game struct {
	ID       string
	DBID     uint
	JoinCode string
	Phase    string
	Players  []string
	Prompts  []string
	Drawings []string
	Guesses  []string
	Votes    []string
}

func NewStore() *Store {
	return &Store{
		nextID: 1,
		games:  make(map[string]*Game),
	}
}

func (s *Store) CreateGame() *Game {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("game-%d", s.nextID)
	s.nextID++
	game := &Game{
		ID:       id,
		JoinCode: newJoinCode(),
		Phase:    "lobby",
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
	game := s.store.CreateGame()
	if err := s.persistGame(game); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create game")
		return
	}
	resp := map[string]string{
		"game_id":   game.ID,
		"join_code": game.JoinCode,
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleGameSubroutes(w http.ResponseWriter, r *http.Request) {
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
		case "prompts":
			s.handlePrompts(w, r, gameID)
		case "drawings":
			s.handleDrawings(w, r, gameID)
		case "guesses":
			s.handleGuesses(w, r, gameID)
		case "votes":
			s.handleVotes(w, r, gameID)
		case "advance":
			s.handleAdvance(w, r, gameID)
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

type joinRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleJoinGame(w http.ResponseWriter, r *http.Request, gameID string) {
	var req joinRequest
	if err := readJSON(r.Body, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Players = append(game.Players, req.Name)
		return nil
	})
	if err != nil {
		if matched, ok := s.store.FindGameByJoinCode(gameID); ok {
			game, err = s.store.UpdateGame(matched.ID, func(game *Game) error {
				game.Players = append(game.Players, req.Name)
				return nil
			})
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	playerID, persistErr := s.persistPlayer(game, req.Name)
	if persistErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to join game")
		return
	}

	resp := map[string]any{
		"game_id": game.ID,
		"player":  req.Name,
	}
	if playerID != 0 {
		resp["player_id"] = playerID
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStartGame(w http.ResponseWriter, r *http.Request, gameID string) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Phase = "in_progress"
		return nil
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.persistPhase(game, "game_started", map[string]any{"phase": game.Phase}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start game")
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

type promptsRequest struct {
	Prompts []string `json:"prompts"`
}

func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request, gameID string) {
	var req promptsRequest
	if err := readJSON(r.Body, &req); err != nil || len(req.Prompts) == 0 {
		writeError(w, http.StatusBadRequest, "prompts are required")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Prompts = append(game.Prompts, req.Prompts...)
		return nil
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.persistEvent(game, "prompts_submitted", map[string]any{"prompts": req.Prompts}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save prompts")
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

type drawingsRequest struct {
	Drawings []string `json:"drawings"`
}

func (s *Server) handleDrawings(w http.ResponseWriter, r *http.Request, gameID string) {
	var req drawingsRequest
	if err := readJSON(r.Body, &req); err != nil || len(req.Drawings) == 0 {
		writeError(w, http.StatusBadRequest, "drawings are required")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Drawings = append(game.Drawings, req.Drawings...)
		return nil
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.persistEvent(game, "drawings_submitted", map[string]any{"drawings": req.Drawings}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save drawings")
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

type guessesRequest struct {
	Guesses []string `json:"guesses"`
}

func (s *Server) handleGuesses(w http.ResponseWriter, r *http.Request, gameID string) {
	var req guessesRequest
	if err := readJSON(r.Body, &req); err != nil || len(req.Guesses) == 0 {
		writeError(w, http.StatusBadRequest, "guesses are required")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Guesses = append(game.Guesses, req.Guesses...)
		return nil
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.persistEvent(game, "guesses_submitted", map[string]any{"guesses": req.Guesses}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save guesses")
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

type votesRequest struct {
	Votes []string `json:"votes"`
}

func (s *Server) handleVotes(w http.ResponseWriter, r *http.Request, gameID string) {
	var req votesRequest
	if err := readJSON(r.Body, &req); err != nil || len(req.Votes) == 0 {
		writeError(w, http.StatusBadRequest, "votes are required")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Votes = append(game.Votes, req.Votes...)
		return nil
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.persistEvent(game, "votes_submitted", map[string]any{"votes": req.Votes}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save votes")
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

func (s *Server) handleAdvance(w http.ResponseWriter, r *http.Request, gameID string) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		game.Phase = "next"
		return nil
	})
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to advance game")
		return
	}
	writeJSON(w, http.StatusOK, snapshot(game))
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request, gameID string) {
	game, ok := s.store.GetGame(gameID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game_id": game.ID,
		"phase":   game.Phase,
		"players": game.Players,
		"counts": map[string]int{
			"prompts":  len(game.Prompts),
			"drawings": len(game.Drawings),
			"guesses":  len(game.Guesses),
			"votes":    len(game.Votes),
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
	w.Header().Set("Upgrade", "websocket")
	writeError(w, http.StatusUpgradeRequired, "websocket upgrade required")
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

func snapshot(game *Game) map[string]any {
	return map[string]any{
		"game_id":   game.ID,
		"join_code": game.JoinCode,
		"phase":     game.Phase,
		"players":   game.Players,
		"counts": map[string]int{
			"prompts":  len(game.Prompts),
			"drawings": len(game.Drawings),
			"guesses":  len(game.Guesses),
			"votes":    len(game.Votes),
		},
	}
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
		JoinCode: game.JoinCode,
		Phase:    game.Phase,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	game.DBID = record.ID
	return s.persistEvent(game, "game_created", map[string]any{
		"game_id":   game.ID,
		"join_code": game.JoinCode,
	})
}

func (s *Server) persistPlayer(game *Game, name string) (uint, error) {
	if s.db == nil {
		return 0, nil
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			return 0, err
		}
		if game.DBID == 0 {
			return 0, errors.New("game not found")
		}
	}
	player := db.Player{
		GameID:   game.DBID,
		Name:     name,
		JoinedAt: time.Now().UTC(),
	}
	if err := s.db.Create(&player).Error; err != nil {
		return 0, err
	}
	if err := s.persistEvent(game, "player_joined", map[string]any{"player": name}); err != nil {
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
	return s.persistEvent(game, eventType, payload)
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
