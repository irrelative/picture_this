package server

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"picture-this/internal/db"

	"github.com/gin-gonic/gin"
)

type settingsRequest struct {
	PlayerID       int    `json:"player_id" binding:"required,gt=0"`
	Rounds         int    `json:"rounds" binding:"min=0,max=10"`
	MaxPlayers     int    `json:"max_players" binding:"min=0,max=12"`
	LobbyLocked    bool   `json:"lobby_locked"`
}

type kickRequest struct {
	PlayerID int `json:"player_id" binding:"required,gt=0"`
	TargetID int `json:"target_id" binding:"required,gt=0"`
}

type playerPromptURI struct {
	GameID   string `uri:"gameID" binding:"required"`
	PlayerID int    `uri:"playerID" binding:"required,gt=0"`
}

type joinRequest struct {
	Name       string `json:"name" binding:"required,name"`
	AvatarData string `json:"avatar_data"`
}

type startRequest struct {
	PlayerID int `json:"player_id"`
}

type avatarRequest struct {
	PlayerID   int    `json:"player_id" binding:"required,gt=0"`
	AvatarData string `json:"avatar_data" binding:"required"`
}
type drawingsRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	ImageData string `json:"image_data" binding:"required"`
	Prompt    string `json:"prompt" binding:"required,prompt"`
}

type guessesRequest struct {
	PlayerID int    `json:"player_id" binding:"required,gt=0"`
	Guess    string `json:"guess" binding:"required,guess"`
}

type votesRequest struct {
	PlayerID int    `json:"player_id" binding:"required,gt=0"`
	Choice   string `json:"choice"`
	Guess    string `json:"guess"`
}

func (s *Server) handleCreateGame(c *gin.Context) {
	if !s.enforceRateLimit(c, "create") {
		return
	}
	game := s.store.CreateGame(s.cfg.PromptsPerPlayer)
	if err := s.persistGame(game); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create game"})
		return
	}
	log.Printf("game created game_id=%s join_code=%s", game.ID, game.JoinCode)
	resp := map[string]string{
		"game_id":   game.ID,
		"join_code": game.JoinCode,
	}
	c.JSON(http.StatusCreated, resp)
	s.broadcastHomeUpdate()
}

func (s *Server) handlePlayerPrompt(c *gin.Context) {
	var uri playerPromptURI
	if !bindURI(c, &uri) {
		c.Status(http.StatusNotFound)
		return
	}
	game, player, ok := s.store.GetPlayer(uri.GameID, uri.PlayerID)
	if !ok || game == nil || player == nil {
		c.Status(http.StatusNotFound)
		return
	}
	round := currentRound(game)
	if round == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "round not started"})
		return
	}
	prompt := promptForPlayer(round, player.ID)
	if prompt == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "prompt not assigned"})
		return
	}
	c.JSON(http.StatusOK, map[string]any{
		"game_id":   game.ID,
		"player_id": player.ID,
		"prompt":    prompt,
	})
}

func (s *Server) handleGetGame(c *gin.Context) {
	gameID := c.Param("gameID")
	game, ok := s.store.GetGame(gameID)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, s.snapshot(game))
}

func (s *Server) handleJoinGame(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "join") {
		return
	}
	var req joinRequest
	if !bindJSON(c, &req, bindMessages{
		"Name": {
			"required": "name is required",
			"name":     "name is invalid",
		},
	}, "name is required") {
		return
	}
	name := normalizeText(req.Name)
	avatar := []byte(nil)
	if strings.TrimSpace(req.AvatarData) != "" {
		decoded, err := decodeImageData(req.AvatarData)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "avatar image is required"})
			return
		}
		if len(decoded) > maxDrawingBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "avatar exceeds size limit"})
			return
		}
		avatar = decoded
	}

	game, player, err := s.store.AddPlayer(gameID, name, avatar)
	if err != nil {
		if err.Error() == "game not found" {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	playerID, persistErr := s.persistPlayer(game, player)
	if persistErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join game"})
		return
	}

	resp := map[string]any{
		"game_id":   game.ID,
		"player":    name,
		"join_code": game.JoinCode,
	}
	resp["player_id"] = playerID
	c.JSON(http.StatusOK, resp)
	log.Printf("player joined game_id=%s player_id=%d player_name=%s", game.ID, playerID, name)

	if s.sessions != nil {
		s.sessions.SetName(c.Writer, c.Request, name)
	}

	s.broadcastGameUpdate(game)
}

func (s *Server) handleAvatar(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "avatar") {
		return
	}
	var req avatarRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "player_id is required",
			"gt":       "player_id is required",
		},
		"AvatarData": {
			"required": "avatar image is required",
		},
	}, "avatar image is required") {
		return
	}
	avatar, err := decodeImageData(req.AvatarData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar image is required"})
		return
	}
	if len(avatar) > maxDrawingBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar exceeds size limit"})
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseLobby {
			return errors.New("avatars only available in lobby")
		}
		player, ok := s.store.FindPlayer(game, req.PlayerID)
		if !ok {
			return errors.New("player not found")
		}
		player.Avatar = avatar
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	player, ok := s.store.FindPlayer(game, req.PlayerID)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"error": "player not found"})
		return
	}
	if err := s.persistPlayerAvatar(game, player); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save avatar"})
		return
	}
	if err := s.persistEvent(game, "avatar_updated", EventPayload{
		PlayerName: player.Name,
		PlayerID:   player.ID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save avatar event"})
		return
	}
	log.Printf("player avatar updated game_id=%s player_id=%d", game.ID, req.PlayerID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleEvents(c *gin.Context) {
	gameID := c.Param("gameID")
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "events not available"})
		return
	}
	game, ok := s.store.GetGame(gameID)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	if game.DBID == 0 {
		if err := s.ensureGameDBID(game); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load game"})
			return
		}
	}
	var records []db.Event
	if err := s.db.Where("game_id = ?", game.DBID).Order("created_at asc").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load events"})
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
	c.JSON(http.StatusOK, map[string]any{
		"game_id": game.ID,
		"events":  events,
	})
}

func (s *Server) handleSettings(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "settings") {
		return
	}
	var req settingsRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "player_id is required",
			"gt":       "player_id is required",
		},
		"Rounds": {
			"max": "rounds exceeds maximum",
		},
		"MaxPlayers": {
			"max": "max players exceeds maximum",
		},
	}, "invalid settings") {
		return
	}
	if req.MaxPlayers < 0 || req.Rounds < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings"})
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
		game.LobbyLocked = req.LobbyLocked
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err := s.persistSettings(game); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
		return
	}
	log.Printf("settings updated game_id=%s rounds=%d max_players=%d locked=%t", game.ID, game.PromptsPerPlayer, game.MaxPlayers, game.LobbyLocked)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleKick(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "kick") {
		return
	}
	var req kickRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "player_id and target_id are required",
			"gt":       "player_id and target_id are required",
		},
		"TargetID": {
			"required": "player_id and target_id are required",
			"gt":       "player_id and target_id are required",
		},
	}, "player_id and target_id are required") {
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
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	log.Printf("player removed game_id=%s target_id=%d", game.ID, req.TargetID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleStartGame(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "start") {
		return
	}
	var req startRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
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
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err := s.persistPhase(game, "game_started", EventPayload{Phase: game.Phase}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start game"})
		return
	}
	if err := s.persistRound(game); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create round"})
		return
	}
	if err := s.assignPrompts(game); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign prompts"})
		return
	}
	log.Printf("game started game_id=%s phase=%s", game.ID, game.Phase)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

func (s *Server) handleDrawings(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "drawings") {
		return
	}
	var req drawingsRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "drawings are required",
			"gt":       "drawings are required",
		},
		"ImageData": {
			"required": "drawings are required",
		},
		"Prompt": {
			"required": "drawings are required",
			"prompt":   "prompt is invalid",
		},
	}, "drawings are required") {
		return
	}
	promptText := strings.TrimSpace(req.Prompt)
	image, err := decodeImageData(req.ImageData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image data"})
		return
	}
	if len(image) > maxDrawingBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "drawing exceeds size limit"})
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
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err := s.persistDrawing(game, req.PlayerID, image, promptText); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save drawings"})
		return
	}
	advanced, updated, err := s.tryAdvanceToGuesses(gameID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
		return
	}
	if advanced {
		game = updated
		if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("drawing submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleGuesses(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "guesses") {
		return
	}
	var req guessesRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "guesses are required",
			"gt":       "guesses are required",
		},
		"Guess": {
			"required": "guesses are required",
			"guess":    "guess is invalid",
		},
	}, "guesses are required") {
		return
	}
	guessText := normalizeText(req.Guess)
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
		if round.CurrentGuess >= len(round.GuessTurns) || round.GuessTurns[round.CurrentGuess].DrawingIndex != turn.DrawingIndex {
			if err := s.buildVoteTurns(game, round); err != nil {
				return err
			}
			start := voteTurnStartIndex(round, turn.DrawingIndex)
			if start < 0 {
				return errors.New("no vote turns for drawing")
			}
			if round.CurrentVote > start {
				return errors.New("vote turns out of sync")
			}
			if round.CurrentVote < start {
				round.CurrentVote = start
			}
			setPhase(game, phaseGuessVotes)
		}
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save guesses"})
		return
	}
	if game.Phase == phaseGuessVotes {
		if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("guess submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

func (s *Server) handleVotes(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "votes") {
		return
	}
	var req votesRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "votes are required",
			"gt":       "votes are required",
		},
	}, "votes are required") {
		return
	}
	choiceText := normalizeText(req.Choice)
	if choiceText == "" {
		choiceText = normalizeText(req.Guess)
	}
	if choiceText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "votes are required"})
		return
	}
	choiceText, err := validateChoice(choiceText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	voteRoundNumber := 0
	voteDrawingIndex := -1
	voteChoiceType := "guess"
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
		voteRoundNumber = round.Number
		voteDrawingIndex = turn.DrawingIndex
		voteChoiceType = choiceType
		round.Votes = append(round.Votes, VoteEntry{
			PlayerID:     player.ID,
			DrawingIndex: turn.DrawingIndex,
			ChoiceText:   choiceText,
			ChoiceType:   choiceType,
		})
		round.CurrentVote++
		if round.CurrentVote >= len(round.VoteTurns) || round.VoteTurns[round.CurrentVote].DrawingIndex != turn.DrawingIndex {
			setPhase(game, phaseResults)
			initReveal(round, turn.DrawingIndex)
		}
		return nil
	})
	if err != nil {
		if err.Error() == "game not found" {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err := s.persistVote(game, req.PlayerID, voteRoundNumber, voteDrawingIndex, choiceText, voteChoiceType); err != nil {
		log.Printf("persist vote failed game_id=%s player_id=%d error=%v", game.ID, req.PlayerID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save votes"})
		return
	}
	if game.Phase == phaseResults {
		if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("vote submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

func (s *Server) handleAdvance(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "advance") {
		return
	}
	prevPhase := ""
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		prevPhase = game.Phase
		_, err := s.advancePhase(game, transitionManual, time.Time{})
		return err
	})
	if err != nil {
		if err.Error() == "game not found" {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if game.Phase == phaseDrawings && prevPhase != phaseDrawings {
		if err := s.persistRound(game); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create round"})
			return
		}
		if len(game.Players) > 0 {
			if err := s.assignPrompts(game); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign prompts"})
				return
			}
		}
	}
	if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
		return
	}
	log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
	s.schedulePhaseTimer(game)
}

func (s *Server) handleEndGame(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "end") {
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
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err := s.persistPhase(game, "game_ended", EventPayload{Phase: game.Phase}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to end game"})
		return
	}
	log.Printf("game ended game_id=%s", game.ID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
}

func (s *Server) handleResults(c *gin.Context) {
	gameID := c.Param("gameID")
	game, ok := s.store.GetGame(gameID)
	if !ok {
		c.Status(http.StatusNotFound)
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
	c.JSON(http.StatusOK, map[string]any{
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
