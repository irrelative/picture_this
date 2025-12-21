package server

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"picture-this/internal/db"
)

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

type drawingsRequest struct {
	PlayerID  int    `json:"player_id"`
	ImageData string `json:"image_data"`
	Prompt    string `json:"prompt"`
}

type guessesRequest struct {
	PlayerID int    `json:"player_id"`
	Guess    string `json:"guess"`
}

type votesRequest struct {
	PlayerID int    `json:"player_id"`
	Choice   string `json:"choice"`
	Guess    string `json:"guess"`
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

func (s *Server) handlePlayerPrompt(w http.ResponseWriter, r *http.Request, gameID string, playerID int) {
	game, player, ok := s.store.GetPlayer(gameID, playerID)
	if !ok || game == nil || player == nil {
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
	if action == "" {
		s.handleGetGame(w, r, gameID)
		return
	}
	if action == "events" {
		s.handleEvents(w, r, gameID)
		return
	}

	switch r.Method {
	case http.MethodGet:
		switch action {
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
