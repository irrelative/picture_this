package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"picture-this/internal/db"

	"github.com/jackc/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

func (s *Server) persistGame(game *Game) error {
	if s.db == nil {
		return nil
	}
	record := db.Game{
		JoinCode:         game.JoinCode,
		Phase:            game.Phase,
		PromptsPerPlayer: game.PromptsPerPlayer,
		MaxPlayers:       game.MaxPlayers,
		LobbyLocked:      game.LobbyLocked,
	}
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error; err != nil {
		return err
	}
	game.DBID = record.ID
	newID := fmt.Sprintf("game-%d", record.ID)
	if game.ID != newID {
		s.store.UpdateGameID(game, newID)
	}
	return s.persistEvent(game, "game_created", EventPayload{
		GameID:   game.ID,
		JoinCode: game.JoinCode,
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
		GameID:      game.DBID,
		Name:        player.Name,
		AvatarImage: player.Avatar,
		Color:       player.Color,
		IsHost:      player.IsHost,
		JoinedAt:    time.Now().UTC(),
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
	if err := s.persistEvent(game, "player_joined", EventPayload{
		PlayerName: player.Name,
		PlayerID:   player.ID,
	}); err != nil {
		return player.ID, err
	}
	return player.ID, nil
}

func (s *Server) persistPlayerAvatar(game *Game, player *Player) error {
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
	if player.DBID == 0 {
		if existing, err := s.findPlayerDBID(game.DBID, player.Name); err == nil {
			player.DBID = existing
		}
	}
	if player.DBID == 0 {
		return errors.New("player not found")
	}
	return s.db.Model(&db.Player{}).
		Where("id = ?", player.DBID).
		Update("avatar_image", player.Avatar).Error
}

func (s *Server) persistPhase(game *Game, eventType string, payload EventPayload) error {
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
		"lobby_locked":       game.LobbyLocked,
	}
	if err := s.db.Model(&db.Game{}).Where("id = ?", game.DBID).Updates(updates).Error; err != nil {
		return err
	}
	return s.persistEvent(game, "settings_updated", EventPayload{
		PromptsPerPlayer: game.PromptsPerPlayer,
		MaxPlayers:       game.MaxPlayers,
		LobbyLocked:      game.LobbyLocked,
	})
}

func (s *Server) persistEvent(game *Game, eventType string, payload EventPayload) error {
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
		GameID:   game.DBID,
		RoundID:  s.resolveEventRoundID(game),
		PlayerID: s.resolveEventPlayerID(game, payload),
		Type:     eventType,
		Payload:  datatypes.JSON(data),
	}
	return s.db.Create(&event).Error
}

func (s *Server) resolveEventRoundID(game *Game) *uint {
	round := currentRound(game)
	if round == nil {
		return nil
	}
	if round.DBID == 0 {
		if err := s.persistRound(game); err != nil {
			return nil
		}
	}
	if round.DBID == 0 {
		return nil
	}
	id := round.DBID
	return &id
}

func (s *Server) resolveEventPlayerID(game *Game, payload EventPayload) *uint {
	if payload.PlayerID <= 0 {
		return nil
	}
	player, found := s.store.FindPlayer(game, payload.PlayerID)
	if found && player.DBID != 0 {
		value := player.DBID
		return &value
	}
	return nil
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
			RoundID:       round.DBID,
			PlayerID:      player.DBID,
			Text:          entry.Text,
			Joke:          entry.Joke,
			JokeAudioPath: entry.JokeAudioPath,
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
		return s.persistEvent(game, "drawings_submitted", EventPayload{
			PlayerID: playerID,
			Prompt:   promptText,
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
	return s.persistEvent(game, "drawings_submitted", EventPayload{
		PlayerID: playerID,
	})
}

func (s *Server) persistGuess(game *Game, playerID int, drawingIndex int, guess string) error {
	if s.db == nil {
		return s.persistEvent(game, "guesses_submitted", EventPayload{
			PlayerID: playerID,
			Guess:    guess,
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
	return s.persistEvent(game, "guesses_submitted", EventPayload{
		PlayerID: playerID,
		Guess:    guess,
	})
}

func (s *Server) persistVote(game *Game, playerID int, roundNumber int, drawingIndex int, choiceText string, choiceType string) error {
	if s.db == nil {
		return s.persistEvent(game, "votes_submitted", EventPayload{
			PlayerID: playerID,
			Choice:   choiceText,
		})
	}
	round := roundByNumber(game, roundNumber)
	if round == nil {
		return errors.New("round not started")
	}
	if round.DBID == 0 {
		return errors.New("round not persisted")
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
	return s.persistEvent(game, "votes_submitted", EventPayload{
		PlayerID: playerID,
		Choice:   choiceText,
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
