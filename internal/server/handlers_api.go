package server

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"picture-this/internal/db"

	"github.com/gin-gonic/gin"
)

type settingsRequest struct {
	PlayerID    int    `json:"player_id" binding:"required,gt=0"`
	AuthToken   string `json:"auth_token"`
	Rounds      int    `json:"rounds" binding:"min=0,max=10"`
	MaxPlayers  int    `json:"max_players" binding:"min=0,max=12"`
	LobbyLocked bool   `json:"lobby_locked"`
}

type kickRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	TargetID  int    `json:"target_id" binding:"required,gt=0"`
	AuthToken string `json:"auth_token"`
}

type playerPromptURI struct {
	GameID   string `uri:"gameID" binding:"required"`
	PlayerID int    `uri:"playerID" binding:"required,gt=0"`
}

type joinRequest struct {
	Name       string `json:"name" binding:"required,name"`
	AvatarData string `json:"avatar_data"`
}

type audienceJoinRequest struct {
	Name  string `json:"name" binding:"required,name"`
	Token string `json:"token"`
}

type audienceVoteRequest struct {
	AudienceID   int    `json:"audience_id" binding:"required,gt=0"`
	Token        string `json:"token" binding:"required"`
	DrawingIndex int    `json:"drawing_index"`
	Choice       string `json:"choice"`
	ChoiceID     string `json:"choice_id"`
}

type startRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	AuthToken string `json:"auth_token"`
}

type avatarRequest struct {
	PlayerID   int    `json:"player_id" binding:"required,gt=0"`
	AvatarData string `json:"avatar_data" binding:"required"`
	AuthToken  string `json:"auth_token"`
}
type drawingsRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	ImageData string `json:"image_data" binding:"required"`
	Prompt    string `json:"prompt" binding:"required,prompt"`
	AuthToken string `json:"auth_token"`
}

type guessesRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	Guess     string `json:"guess" binding:"required,guess"`
	AuthToken string `json:"auth_token"`
}

type votesRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	Choice    string `json:"choice"`
	ChoiceID  string `json:"choice_id"`
	Guess     string `json:"guess"`
	AuthToken string `json:"auth_token"`
}

type advanceRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	AuthToken string `json:"auth_token"`
}

type endRequest struct {
	PlayerID  int    `json:"player_id" binding:"required,gt=0"`
	AuthToken string `json:"auth_token"`
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
	if _, err := s.authenticatePlayerRequest(c, game, player.ID, c.Query("auth_token")); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
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
		game, ok = s.store.FindGameByJoinCode(gameID)
	}
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
		if err.Error() == "game is paused" {
			c.JSON(http.StatusConflict, gin.H{"error": "game is paused; enter an existing player name to claim your seat"})
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
	resp["auth_token"] = ensurePlayerAuthToken(game, playerID)
	c.JSON(http.StatusOK, resp)
	log.Printf("player joined game_id=%s player_id=%d player_name=%s", game.ID, playerID, name)

	if s.sessions != nil {
		s.sessions.SetName(c.Writer, c.Request, name)
	}

	s.broadcastGameUpdate(game)
}

func (s *Server) handleAudienceJoin(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "audience-join") {
		return
	}
	var req audienceJoinRequest
	if !bindJSON(c, &req, bindMessages{
		"Name": {
			"required": "name is required",
			"name":     "name is invalid",
		},
	}, "name is required") {
		return
	}
	name := normalizeText(req.Name)
	token := strings.TrimSpace(req.Token)
	var joined AudienceMember
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase == phaseComplete {
			return errors.New("game already ended")
		}
		if token != "" {
			for i := range game.Audience {
				existingToken := strings.TrimSpace(game.Audience[i].Token)
				if existingToken == "" {
					continue
				}
				if subtle.ConstantTimeCompare([]byte(existingToken), []byte(token)) != 1 {
					continue
				}
				if name != "" && game.Audience[i].Name != name {
					game.Audience[i].Name = name
				}
				joined = game.Audience[i]
				return nil
			}
		}
		nextID := 1
		for _, audience := range game.Audience {
			if audience.ID >= nextID {
				nextID = audience.ID + 1
			}
		}
		joined = AudienceMember{
			ID:    nextID,
			Name:  name,
			Token: newAuthToken(),
		}
		game.Audience = append(game.Audience, joined)
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
	c.JSON(http.StatusOK, map[string]any{
		"game_id":       game.ID,
		"audience_id":   joined.ID,
		"audience_name": joined.Name,
		"token":         joined.Token,
	})
	s.broadcastGameUpdate(game)
}

func (s *Server) handleAudienceVote(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "audience-vote") {
		return
	}
	var req audienceVoteRequest
	if !bindJSON(c, &req, bindMessages{
		"AudienceID": {
			"required": "audience_id is required",
			"gt":       "audience_id is required",
		},
		"Token": {
			"required": "token is required",
		},
	}, "invalid audience vote") {
		return
	}
	choiceID := strings.TrimSpace(req.ChoiceID)
	choiceText := normalizeText(req.Choice)
	if choiceID == "" && choiceText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "votes are required"})
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseGuessVotes {
			return errors.New("votes not accepted in this phase")
		}
		var audience *AudienceMember
		for i := range game.Audience {
			if game.Audience[i].ID == req.AudienceID {
				audience = &game.Audience[i]
				break
			}
		}
		if audience == nil {
			return errors.New("audience member not found")
		}
		if strings.TrimSpace(audience.Token) == "" || subtle.ConstantTimeCompare([]byte(audience.Token), []byte(strings.TrimSpace(req.Token))) != 1 {
			return errors.New("invalid audience authentication")
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		drawingIndex := req.DrawingIndex
		if drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
			_, fallback, ok := firstAssignmentByOrder(game, buildVoteAssignments(game, round))
			if !ok {
				return errors.New("no active vote drawing")
			}
			drawingIndex = fallback
		}
		for _, vote := range round.AudienceVotes {
			if vote.AudienceID == req.AudienceID && vote.DrawingIndex == drawingIndex {
				return errors.New("vote already submitted")
			}
		}
		options := voteOptionEntries(round, drawingIndex)
		selected, ok := selectVoteOption(options, choiceID, choiceText)
		if !ok {
			return errors.New("invalid vote option")
		}
		round.AudienceVotes = append(round.AudienceVotes, AudienceVoteEntry{
			AudienceID:   audience.ID,
			AudienceName: audience.Name,
			DrawingIndex: drawingIndex,
			ChoiceID:     selected.ID,
			ChoiceText:   selected.Text,
			ChoiceType:   selected.Type,
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
	c.JSON(http.StatusOK, s.snapshot(game))
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
		player, err := s.authenticatePlayerRequest(c, game, req.PlayerID, req.AuthToken)
		if err != nil {
			return err
		}
		if player.AvatarLocked {
			return errors.New("avatar is already locked for this game")
		}
		player.Avatar = avatar
		player.AvatarLocked = true
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
		if _, err := s.authenticateHostRequest(c, game, req.PlayerID, req.AuthToken); err != nil {
			return err
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
		if _, err := s.authenticateHostRequest(c, game, req.PlayerID, req.AuthToken); err != nil {
			return err
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
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "player_id is required",
			"gt":       "player_id is required",
		},
	}, "player_id is required") {
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if len(game.Players) < 2 {
			return errors.New("not enough players")
		}
		if _, err := s.authenticateHostRequest(c, game, req.PlayerID, req.AuthToken); err != nil {
			return err
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
		player, err := s.authenticatePlayerRequest(c, game, req.PlayerID, req.AuthToken)
		if err != nil {
			return err
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
	drawingIndex := -1
	phaseAdvanced := false
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseGuesses {
			return errors.New("guesses not accepted in this phase")
		}
		player, err := s.authenticatePlayerRequest(c, game, req.PlayerID, req.AuthToken)
		if err != nil {
			return err
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		assignedDrawing, ok := nextGuessAssignment(game, round, player.ID)
		if !ok {
			return errors.New("no guess assignment")
		}
		if hasGuessForPlayer(round, assignedDrawing, player.ID) {
			return errors.New("guess already submitted")
		}
		if hasGuessText(round, assignedDrawing, guessText) {
			return errors.New("guess already used for this drawing")
		}
		drawingIndex = assignedDrawing
		round.Guesses = append(round.Guesses, GuessEntry{
			PlayerID:     player.ID,
			DrawingIndex: assignedDrawing,
			Text:         guessText,
		})
		if activeGuessDrawingIndex(game, round) < 0 {
			setPhase(game, phaseGuessVotes)
			phaseAdvanced = true
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
	if err := s.persistGuess(game, req.PlayerID, drawingIndex, guessText); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save guesses"})
		return
	}
	if phaseAdvanced && game.Phase == phaseGuessVotes {
		if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("guess submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
	if phaseAdvanced {
		s.schedulePhaseTimer(game)
	}
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
	choiceID := strings.TrimSpace(req.ChoiceID)
	if choiceID == "" && choiceText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "votes are required"})
		return
	}
	voteRoundNumber := 0
	voteDrawingIndex := -1
	voteChoiceText := ""
	voteChoiceType := voteChoiceGuess
	phaseAdvanced := false
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseGuessVotes {
			return errors.New("votes not accepted in this phase")
		}
		player, err := s.authenticatePlayerRequest(c, game, req.PlayerID, req.AuthToken)
		if err != nil {
			return err
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		activeDrawing, ok := nextVoteAssignment(game, round, player.ID)
		if !ok {
			return errors.New("no vote assignment")
		}
		if hasVoteForPlayer(round, activeDrawing, player.ID) {
			return errors.New("vote already submitted")
		}
		options := voteOptionEntries(round, activeDrawing)
		if len(options) == 0 {
			return errors.New("invalid vote option")
		}
		selected, ok := selectVoteOption(options, choiceID, choiceText)
		if !ok {
			return errors.New("invalid vote option")
		}
		if selected.Type == voteChoiceGuess && selected.OwnerID == player.ID {
			return errors.New("cannot vote for your own lie")
		}
		voteRoundNumber = round.Number
		voteDrawingIndex = activeDrawing
		voteChoiceText = selected.Text
		voteChoiceType = selected.Type
		round.Votes = append(round.Votes, VoteEntry{
			PlayerID:     player.ID,
			DrawingIndex: activeDrawing,
			ChoiceText:   selected.Text,
			ChoiceType:   selected.Type,
		})
		if activeVoteDrawingIndex(game, round) < 0 {
			setPhase(game, phaseResults)
			initReveal(round, activeDrawing)
			phaseAdvanced = true
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
	if err := s.persistVote(game, req.PlayerID, voteRoundNumber, voteDrawingIndex, voteChoiceText, voteChoiceType); err != nil {
		log.Printf("persist vote failed game_id=%s player_id=%d error=%v", game.ID, req.PlayerID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save votes"})
		return
	}
	if phaseAdvanced && game.Phase == phaseResults {
		if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to advance game"})
			return
		}
		log.Printf("game advanced game_id=%s phase=%s", game.ID, game.Phase)
	}
	log.Printf("vote submitted game_id=%s player_id=%d", game.ID, req.PlayerID)
	c.JSON(http.StatusOK, s.snapshot(game))
	s.broadcastGameUpdate(game)
	if phaseAdvanced {
		s.schedulePhaseTimer(game)
	}
}

func (s *Server) handleAdvance(c *gin.Context) {
	gameID := c.Param("gameID")
	if !s.enforceRateLimit(c, "advance") {
		return
	}
	var req advanceRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "player_id is required",
			"gt":       "player_id is required",
		},
	}, "player_id is required") {
		return
	}
	prevPhase := ""
	filledGuesses := make([]autoFilledGuess, 0)
	filledVotes := make([]autoFilledVote, 0)
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if _, err := s.authenticateHostRequest(c, game, req.PlayerID, req.AuthToken); err != nil {
			return err
		}
		prevPhase = game.Phase
		if game.Phase == phaseGuesses {
			filledGuesses = append(filledGuesses, autoFillMissingGuesses(game)...)
		}
		if game.Phase == phaseGuessVotes {
			filledVotes = append(filledVotes, autoFillMissingVotes(game)...)
		}
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
	for _, filled := range filledGuesses {
		if err := s.persistGuess(game, filled.PlayerID, filled.DrawingIndex, filled.Text); err != nil {
			log.Printf("manual advance persist guess failed game_id=%s player_id=%d error=%v", game.ID, filled.PlayerID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save guesses"})
			return
		}
	}
	for _, filled := range filledVotes {
		if err := s.persistVote(game, filled.PlayerID, filled.RoundNumber, filled.DrawingIndex, filled.ChoiceText, filled.ChoiceType); err != nil {
			log.Printf("manual advance persist vote failed game_id=%s player_id=%d error=%v", game.ID, filled.PlayerID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save votes"})
			return
		}
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
	var req endRequest
	if !bindJSON(c, &req, bindMessages{
		"PlayerID": {
			"required": "player_id is required",
			"gt":       "player_id is required",
		},
	}, "player_id is required") {
		return
	}
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if _, err := s.authenticateHostRequest(c, game, req.PlayerID, req.AuthToken); err != nil {
			return err
		}
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
