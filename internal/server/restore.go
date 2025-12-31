package server

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"picture-this/internal/db"
)

func (s *Server) restoreGameFromDB(param string) (*Game, string, error) {
	if s.db == nil {
		return nil, "", errors.New("database not configured")
	}
	dbID, displayID, err := s.resolveAdminGameID(strings.TrimSpace(param))
	if err != nil {
		return nil, displayID, err
	}

	var record db.Game
	if err := s.db.First(&record, dbID).Error; err != nil {
		return nil, displayID, err
	}
	if record.Phase == phaseComplete {
		return nil, displayID, errors.New("game already complete")
	}

	if existing, ok := s.store.GetGame(fmt.Sprintf("game-%d", record.ID)); ok {
		return existing, displayID, nil
	}
	if existing, ok := s.store.FindGameByJoinCode(record.JoinCode); ok {
		return existing, displayID, nil
	}

	players, err := s.loadPlayers(record.ID)
	if err != nil {
		return nil, displayID, err
	}
	rounds, err := s.loadRounds(record.ID)
	if err != nil {
		return nil, displayID, err
	}

	roundIDs := make([]uint, 0, len(rounds))
	for _, round := range rounds {
		roundIDs = append(roundIDs, round.ID)
	}

	prompts, drawings, guesses, votes, err := s.loadRoundAssets(roundIDs)
	if err != nil {
		return nil, displayID, err
	}

	game := &Game{
		ID:               fmt.Sprintf("game-%d", record.ID),
		DBID:             record.ID,
		JoinCode:         record.JoinCode,
		Phase:            phasePaused,
		PausedPhase:      record.Phase,
		PhaseStartedAt:   time.Now().UTC(),
		MaxPlayers:       record.MaxPlayers,
		LobbyLocked:      record.LobbyLocked,
		UsedPrompts:      make(map[string]struct{}),
		KickedPlayers:    make(map[string]struct{}),
		PromptsPerPlayer: record.PromptsPerPlayer,
	}

	game.Players = buildPlayers(players, game)
	game.Rounds = buildRounds(rounds, prompts, drawings, guesses, votes)
	game.UsedPrompts = usedPrompts(game.Rounds)

	if round := currentRound(game); round != nil {
		if len(round.Drawings) > 0 {
			_ = s.buildGuessTurns(game, round)
		}
		if len(round.GuessTurns) > 0 {
			round.CurrentGuess = minInt(len(round.Guesses), len(round.GuessTurns))
		}
		if len(round.VoteTurns) > 0 {
			round.CurrentVote = minInt(len(round.Votes), len(round.VoteTurns))
		}
		if game.PausedPhase == phaseResults {
			if round.RevealStage == "" {
				round.RevealStage = revealStageGuesses
			}
			if round.RevealIndex < 0 {
				round.RevealIndex = 0
			}
		}
	}

	if err := s.store.RestoreGame(game); err != nil {
		return nil, displayID, err
	}
	return game, displayID, nil
}

func (s *Server) loadPlayers(gameID uint) ([]db.Player, error) {
	var players []db.Player
	if err := s.db.Where("game_id = ?", gameID).Order("joined_at asc").Find(&players).Error; err != nil {
		return nil, err
	}
	return players, nil
}

func (s *Server) loadRounds(gameID uint) ([]db.Round, error) {
	var rounds []db.Round
	if err := s.db.Where("game_id = ?", gameID).Order("number asc").Find(&rounds).Error; err != nil {
		return nil, err
	}
	return rounds, nil
}

func (s *Server) loadRoundAssets(roundIDs []uint) ([]db.Prompt, []db.Drawing, []db.Guess, []db.Vote, error) {
	if len(roundIDs) == 0 {
		return nil, nil, nil, nil, nil
	}
	var prompts []db.Prompt
	if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&prompts).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var drawings []db.Drawing
	if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&drawings).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var guesses []db.Guess
	if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&guesses).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var votes []db.Vote
	if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&votes).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	return prompts, drawings, guesses, votes, nil
}

func buildPlayers(records []db.Player, game *Game) []Player {
	players := make([]Player, 0, len(records))
	for _, record := range records {
		player := Player{
			ID:      int(record.ID),
			DBID:    record.ID,
			Name:    record.Name,
			Avatar:  record.AvatarImage,
			IsHost:  record.IsHost,
			Color:   record.Color,
			Claimed: false,
		}
		players = append(players, player)
		if record.IsHost {
			game.HostID = player.ID
		}
	}
	return players
}

func buildRounds(rounds []db.Round, prompts []db.Prompt, drawings []db.Drawing, guesses []db.Guess, votes []db.Vote) []RoundState {
	promptsByRound := map[uint][]db.Prompt{}
	for _, prompt := range prompts {
		promptsByRound[prompt.RoundID] = append(promptsByRound[prompt.RoundID], prompt)
	}
	drawingsByRound := map[uint][]db.Drawing{}
	for _, drawing := range drawings {
		drawingsByRound[drawing.RoundID] = append(drawingsByRound[drawing.RoundID], drawing)
	}
	guessesByRound := map[uint][]db.Guess{}
	for _, guess := range guesses {
		guessesByRound[guess.RoundID] = append(guessesByRound[guess.RoundID], guess)
	}
	votesByRound := map[uint][]db.Vote{}
	for _, vote := range votes {
		votesByRound[vote.RoundID] = append(votesByRound[vote.RoundID], vote)
	}

	states := make([]RoundState, 0, len(rounds))
	for _, round := range rounds {
		state := RoundState{Number: round.Number, DBID: round.ID}
		promptRecords := promptsByRound[round.ID]
		promptTextByID := map[uint]string{}
		for _, prompt := range promptRecords {
			state.Prompts = append(state.Prompts, PromptEntry{
				PlayerID: int(prompt.PlayerID),
				Text:     prompt.Text,
				Joke:     prompt.Joke,
				DBID:     prompt.ID,
			})
			promptTextByID[prompt.ID] = prompt.Text
		}

		drawingRecords := drawingsByRound[round.ID]
		sort.SliceStable(drawingRecords, func(i, j int) bool {
			return drawingRecords[i].CreatedAt.Before(drawingRecords[j].CreatedAt)
		})
		drawingIndexByID := map[uint]int{}
		for _, drawing := range drawingRecords {
			promptText := promptTextByID[drawing.PromptID]
			state.Drawings = append(state.Drawings, DrawingEntry{
				PlayerID:  int(drawing.PlayerID),
				ImageData: drawing.ImageData,
				Prompt:    promptText,
				DBID:      drawing.ID,
			})
			drawingIndexByID[drawing.ID] = len(state.Drawings) - 1
		}

		guessRecords := guessesByRound[round.ID]
		sort.SliceStable(guessRecords, func(i, j int) bool {
			return guessRecords[i].CreatedAt.Before(guessRecords[j].CreatedAt)
		})
		for _, guess := range guessRecords {
			index, ok := drawingIndexByID[guess.DrawingID]
			if !ok {
				continue
			}
			state.Guesses = append(state.Guesses, GuessEntry{
				PlayerID:     int(guess.PlayerID),
				DrawingIndex: index,
				Text:         guess.Text,
				DBID:         guess.ID,
			})
		}

		voteRecords := votesByRound[round.ID]
		sort.SliceStable(voteRecords, func(i, j int) bool {
			return voteRecords[i].CreatedAt.Before(voteRecords[j].CreatedAt)
		})
		for _, vote := range voteRecords {
			index, ok := drawingIndexByID[vote.DrawingID]
			if !ok {
				continue
			}
			state.Votes = append(state.Votes, VoteEntry{
				PlayerID:     int(vote.PlayerID),
				DrawingIndex: index,
				ChoiceText:   vote.ChoiceText,
				ChoiceType:   vote.ChoiceType,
				DBID:         vote.ID,
			})
		}

		states = append(states, state)
	}
	return states
}

func usedPrompts(rounds []RoundState) map[string]struct{} {
	used := make(map[string]struct{})
	for _, round := range rounds {
		for _, prompt := range round.Prompts {
			used[prompt.Text] = struct{}{}
		}
	}
	return used
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func countClaimed(players []Player) int {
	count := 0
	for _, player := range players {
		if player.Claimed {
			count++
		}
	}
	return count
}

func allPlayersClaimed(players []Player) bool {
	return len(players) > 0 && countClaimed(players) == len(players)
}
