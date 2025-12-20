package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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
	cfg      config.Config
	sessions *sessionStore
}

const (
	phaseLobby    = "lobby"
	phaseDrawings = "drawings"
	phaseGuesses  = "guesses"
	phaseVotes    = "votes"
	phaseResults  = "results"
	phaseComplete = "complete"
)

var phaseOrder = []string{
	phaseLobby,
	phaseDrawings,
	phaseGuesses,
	phaseVotes,
	phaseResults,
	phaseComplete,
}

func New(conn *gorm.DB, cfg config.Config) *Server {
	return &Server{
		store:    NewStore(),
		db:       conn,
		ws:       newWSHub(),
		cfg:      cfg,
		sessions: newSessionStore(conn),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /join", s.handleJoinView)
	mux.HandleFunc("GET /join/", s.handleJoinView)
	mux.HandleFunc("GET /play/", s.handlePlayerView)
	mux.HandleFunc("GET /games/", s.handleGameView)
	mux.HandleFunc("POST /api/games", s.handleCreateGame)
	mux.HandleFunc("GET /api/games/", s.handleGameSubroutes)
	mux.HandleFunc("POST /api/games/", s.handleGameSubroutes)
	mux.HandleFunc("GET /ws/games/", s.handleWebsocket)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return mux
}

type Store struct {
	mu           sync.Mutex
	nextID       int
	nextPlayerID int
	games        map[string]*Game
}

type Game struct {
	ID               string
	DBID             uint
	JoinCode         string
	Phase            string
	Players          []Player
	Rounds           []RoundState
	PromptsPerPlayer int
	Prompts          []string
	Drawings         []string
	Guesses          []string
	Votes            []string
}

type Player struct {
	ID   int
	Name string
	DBID uint
}

type RoundState struct {
	Number   int
	DBID     uint
	Prompts  []PromptEntry
	Drawings []DrawingEntry
	Guesses  []GuessEntry
	Votes    []VoteEntry
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
	PlayerID int
	Text     string
	DBID     uint
}

type VoteEntry struct {
	PlayerID  int
	GuessText string
	DBID      uint
}

func NewStore() *Store {
	return &Store{
		nextID:       1,
		nextPlayerID: 1,
		games:        make(map[string]*Game),
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

	player := Player{
		ID:   s.nextPlayerID,
		Name: name,
	}
	s.nextPlayerID++
	game.Players = append(game.Players, player)
	return game, &game.Players[len(game.Players)-1], nil
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
	templ.Handler(web.Home(flash, name)).ServeHTTP(w, r)
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
	prompts := promptsForPlayer(round, player.ID)
	if len(prompts) == 0 {
		writeError(w, http.StatusNotFound, "prompt not assigned")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"game_id":   game.ID,
		"player_id": player.ID,
		"prompts":   prompts,
	})
}

func (s *Server) handleGameSubroutes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if playerGameID, playerID, ok := parsePlayerPromptPath(r.URL.Path); ok {
			s.handlePlayerPrompt(w, r, playerGameID, playerID)
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

	game, player, err := s.store.AddPlayer(gameID, req.Name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	playerID, persistErr := s.persistPlayer(game, player)
	if persistErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to join game")
		return
	}

	resp := map[string]any{
		"game_id":   game.ID,
		"player":    req.Name,
		"join_code": game.JoinCode,
	}
	resp["player_id"] = playerID
	writeJSON(w, http.StatusOK, resp)
	log.Printf("player joined game_id=%s player_id=%d player_name=%s", game.ID, playerID, req.Name)

	if s.sessions != nil {
		s.sessions.SetName(w, r, req.Name)
	}

	s.broadcastGameUpdate(game)
}

func (s *Server) handleStartGame(w http.ResponseWriter, r *http.Request, gameID string) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseLobby {
			return errors.New("game already started")
		}
		game.Phase = phaseDrawings
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
}

type promptsRequest struct {
	PlayerID int      `json:"player_id"`
	Prompts  []string `json:"prompts"`
}

func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request, gameID string) {
	writeError(w, http.StatusGone, "prompt entry is disabled")
}

type drawingsRequest struct {
	PlayerID  int    `json:"player_id"`
	ImageData string `json:"image_data"`
	Prompt    string `json:"prompt"`
}

func (s *Server) handleDrawings(w http.ResponseWriter, r *http.Request, gameID string) {
	var req drawingsRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 || req.ImageData == "" {
		writeError(w, http.StatusBadRequest, "drawings are required")
		return
	}
	image, err := decodeImageData(req.ImageData)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image data")
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
		promptEntry, ok := findPromptForPlayer(round, player.ID, req.Prompt)
		if !ok {
			return errors.New("prompt not assigned")
		}
		round.Drawings = append(round.Drawings, DrawingEntry{
			PlayerID:  player.ID,
			ImageData: image,
			Prompt:    promptEntry.Text,
		})
		game.Drawings = append(game.Drawings, req.ImageData)
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
	if err := s.persistDrawing(game, req.PlayerID, image, req.Prompt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save drawings")
		return
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
	var req guessesRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 || strings.TrimSpace(req.Guess) == "" {
		writeError(w, http.StatusBadRequest, "guesses are required")
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
		round.Guesses = append(round.Guesses, GuessEntry{
			PlayerID: player.ID,
			Text:     req.Guess,
		})
		game.Guesses = append(game.Guesses, req.Guess)
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
	if err := s.persistGuess(game, req.PlayerID, req.Guess); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save guesses")
		return
	}
	log.Printf("guess submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

type votesRequest struct {
	PlayerID int    `json:"player_id"`
	Guess    string `json:"guess"`
}

func (s *Server) handleVotes(w http.ResponseWriter, r *http.Request, gameID string) {
	var req votesRequest
	if err := readJSON(r.Body, &req); err != nil || req.PlayerID <= 0 || strings.TrimSpace(req.Guess) == "" {
		writeError(w, http.StatusBadRequest, "votes are required")
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseVotes {
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
		round.Votes = append(round.Votes, VoteEntry{
			PlayerID:  player.ID,
			GuessText: req.Guess,
		})
		game.Votes = append(game.Votes, req.Guess)
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
	if err := s.persistVote(game, req.PlayerID, req.Guess); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save votes")
		return
	}
	log.Printf("vote submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	writeJSON(w, http.StatusOK, snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleAdvance(w http.ResponseWriter, r *http.Request, gameID string) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		next, ok := nextPhase(game.Phase)
		if !ok {
			return errors.New("no next phase")
		}
		game.Phase = next
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
		"players": extractPlayerNames(game.Players),
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
	s.ws.Add(gameID, conn)
	if game, ok := s.store.GetGame(gameID); ok {
		s.ws.Send(conn, snapshot(game))
	}
	go s.readWS(gameID, conn)
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

func promptsForPlayer(round *RoundState, playerID int) []string {
	if round == nil {
		return nil
	}
	var prompts []string
	for _, entry := range round.Prompts {
		if entry.PlayerID == playerID {
			prompts = append(prompts, entry.Text)
		}
	}
	return prompts
}

func findPromptForPlayer(round *RoundState, playerID int, promptText string) (PromptEntry, bool) {
	if round == nil {
		return PromptEntry{}, false
	}
	if promptText != "" {
		for _, entry := range round.Prompts {
			if entry.PlayerID == playerID && entry.Text == promptText {
				return entry, true
			}
		}
		return PromptEntry{}, false
	}
	for _, entry := range round.Prompts {
		if entry.PlayerID == playerID {
			return entry, true
		}
	}
	return PromptEntry{}, false
}

type wsHub struct {
	mu     sync.Mutex
	groups map[string]map[*websocket.Conn]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		groups: make(map[string]map[*websocket.Conn]struct{}),
	}
}

func (h *wsHub) Add(gameID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	group := h.groups[gameID]
	if group == nil {
		group = make(map[*websocket.Conn]struct{})
		h.groups[gameID] = group
	}
	group[conn] = struct{}{}
}

func (h *wsHub) Remove(gameID string, conn *websocket.Conn) {
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
			h.Remove(gameID, conn)
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
	if round != nil {
		roundNumber = round.Number
		drawingsCount = len(round.Drawings)
		guessesCount = len(round.Guesses)
		votesCount = len(round.Votes)
	}
	return map[string]any{
		"game_id":            game.ID,
		"join_code":          game.JoinCode,
		"phase":              game.Phase,
		"players":            extractPlayerNames(game.Players),
		"round_number":       roundNumber,
		"prompts_per_player": game.PromptsPerPlayer,
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

func currentRound(game *Game) *RoundState {
	if len(game.Rounds) == 0 {
		return nil
	}
	return &game.Rounds[len(game.Rounds)-1]
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

func (s *Server) persistPrompts(game *Game, playerID int, prompts []string) error {
	if s.db == nil {
		return s.persistEvent(game, "prompts_submitted", map[string]any{
			"player_id": playerID,
			"prompts":   prompts,
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
	for _, prompt := range prompts {
		record := db.Prompt{
			RoundID:  round.DBID,
			PlayerID: player.DBID,
			Text:     prompt,
		}
		if err := s.db.Create(&record).Error; err != nil {
			return err
		}
	}
	return s.persistEvent(game, "prompts_submitted", map[string]any{
		"player_id": playerID,
		"prompts":   prompts,
	})
}

func (s *Server) assignPrompts(game *Game) error {
	round := currentRound(game)
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Prompts) > 0 {
		return nil
	}
	count := game.PromptsPerPlayer
	if count <= 0 {
		count = 1
	}
	total := len(game.Players) * count
	if total == 0 {
		return errors.New("no players to assign prompts")
	}

	prompts, err := s.loadPromptLibrary(total)
	if err != nil {
		return err
	}
	if len(prompts) < total {
		return errors.New("not enough prompts available")
	}

	idx := 0
	for _, player := range game.Players {
		for i := 0; i < count; i++ {
			round.Prompts = append(round.Prompts, PromptEntry{
				PlayerID: player.ID,
				Text:     prompts[idx],
			})
			idx++
		}
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

func (s *Server) loadPromptLibrary(limit int) ([]string, error) {
	if s.db == nil {
		return fallbackPrompts(limit), nil
	}
	var records []db.PromptLibrary
	if err := s.db.Order("random()").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	prompts := make([]string, 0, len(records))
	for _, record := range records {
		prompts = append(prompts, record.Text)
	}
	return prompts, nil
}

func fallbackPrompts(limit int) []string {
	fallback := []string{
		"Draw a llama in a suit",
		"Draw a castle made of pancakes",
		"Draw a robot learning to dance",
		"Draw a pirate cat at a tea party",
		"Draw a rocket powered skateboard",
		"Draw a haunted treehouse",
		"Draw a snowy beach day",
		"Draw a giant sunflower city",
	}
	if limit >= len(fallback) {
		return fallback
	}
	return fallback[:limit]
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
	return s.persistEvent(game, "drawings_submitted", map[string]any{
		"player_id": playerID,
	})
}

func (s *Server) persistGuess(game *Game, playerID int, guess string) error {
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
	record := db.Guess{
		RoundID:   round.DBID,
		PlayerID:  player.DBID,
		DrawingID: 0,
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

func (s *Server) persistVote(game *Game, playerID int, guess string) error {
	if s.db == nil {
		return s.persistEvent(game, "votes_submitted", map[string]any{
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
	record := db.Vote{
		RoundID:  round.DBID,
		PlayerID: player.DBID,
		GuessID:  0,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	return s.persistEvent(game, "votes_submitted", map[string]any{
		"player_id": playerID,
		"guess":     guess,
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
}

func (s *Server) readWS(gameID string, conn *websocket.Conn) {
	defer s.ws.Remove(gameID, conn)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("ws disconnected game_id=%s error=%v", gameID, err)
			return
		}
	}
}
