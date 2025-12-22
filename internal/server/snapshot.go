package server

import (
	"sort"
	"time"

	"picture-this/internal/config"
)

func snapshotWithConfig(game *Game, cfg config.Config) map[string]any {
	players := extractPlayerNames(game.Players)
	playerIDs := extractPlayerIDs(game.Players)
	scores := buildScores(game)
	reveal := buildReveal(game)
	promptsCount := 0
	drawingsCount := 0
	guessesCount := 0
	votesCount := 0
	if round := currentRound(game); round != nil {
		promptsCount = len(round.Prompts)
		drawingsCount = len(round.Drawings)
		guessesCount = len(round.Guesses)
		votesCount = len(round.Votes)
	}
	playerColors := extractPlayerColors(game.Players)
	playerAvatars := extractPlayerAvatars(game.Players)
	var guessTurn map[string]any
	var voteTurn map[string]any
	if round := currentRound(game); round != nil {
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
	}
	phaseDuration := phaseDurationSeconds(cfg, game.Phase)
	phaseEndsAt := ""
	if !game.PhaseStartedAt.IsZero() && phaseDuration > 0 {
		phaseEndsAt = game.PhaseStartedAt.Add(time.Duration(phaseDuration) * time.Second).UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"game_id":            game.ID,
		"join_code":          game.JoinCode,
		"phase":              game.Phase,
		"phase_started_at":   game.PhaseStartedAt,
		"phase_duration":     phaseDuration,
		"phase_ends_at":      phaseEndsAt,
		"players":            players,
		"player_ids":         playerIDs,
		"player_colors":      playerColors,
		"player_avatars":     playerAvatars,
		"prompt_category":    game.PromptCategory,
		"max_players":        game.MaxPlayers,
		"lobby_locked":       game.LobbyLocked,
		"host_id":            game.HostID,
		"scores":             scores,
		"results":            buildResults(game),
		"reveal":             reveal,
		"total_rounds":       game.PromptsPerPlayer,
		"current_round":      len(game.Rounds),
		"guess_turn":         guessTurn,
		"vote_turn":          voteTurn,
		"prompts_per_player": game.PromptsPerPlayer,
		"audience_count":     len(game.Audience),
		"audience_options":   buildAudienceOptions(game),
		"counts": map[string]int{
			"prompts":  promptsCount,
			"drawings": drawingsCount,
			"guesses":  guessesCount,
			"votes":    votesCount,
		},
		"can_join": game.Phase == phaseLobby && !game.LobbyLocked && (game.MaxPlayers == 0 || len(game.Players) < game.MaxPlayers),
	}
}

func phaseDurationSeconds(cfg config.Config, phase string) int {
	switch phase {
	case phaseDrawings:
		return cfg.DrawDurationSeconds
	case phaseGuesses:
		return cfg.GuessDurationSeconds
	case phaseGuessVotes:
		return cfg.VoteDurationSeconds
	case phaseResults:
		return cfg.RevealDurationSeconds
	default:
		return 0
	}
}

func extractPlayerNames(players []Player) []string {
	list := make([]string, 0, len(players))
	for _, player := range players {
		list = append(list, player.Name)
	}
	return list
}

func extractPlayerColors(players []Player) map[int]string {
	colors := make(map[int]string, len(players))
	for _, player := range players {
		colors[player.ID] = player.Color
	}
	return colors
}

func extractPlayerAvatars(players []Player) map[int]string {
	avatars := make(map[int]string, len(players))
	for _, player := range players {
		if len(player.Avatar) == 0 {
			continue
		}
		avatars[player.ID] = encodeImageData(player.Avatar)
	}
	return avatars
}

func extractPlayerIDs(players []Player) []int {
	list := make([]int, 0, len(players))
	for _, player := range players {
		list = append(list, player.ID)
	}
	return list
}

func buildResults(game *Game) []map[string]any {
	round := currentRound(game)
	if round == nil {
		return nil
	}
	playerNames := map[int]string{}
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
	scores := map[int]int{}
	for _, player := range game.Players {
		scores[player.ID] = 0
	}
	for drawingIndex, drawing := range round.Drawings {
		fooledVotes := 0
		for _, vote := range round.Votes {
			if vote.DrawingIndex != drawingIndex {
				continue
			}
			if vote.ChoiceType == "prompt" {
				scores[vote.PlayerID] += 1000
			} else if vote.ChoiceType == "guess" {
				if ownerID := guessOwner(round, drawingIndex, vote.ChoiceText); ownerID != 0 {
					scores[ownerID] += 500
					fooledVotes++
				}
			}
		}
		if fooledVotes == 0 {
			scores[drawing.PlayerID] += 1000
		} else {
			scores[drawing.PlayerID] += 500 * fooledVotes
		}
	}
	results := make([]map[string]any, 0, len(scores))
	for _, player := range game.Players {
		results = append(results, map[string]any{
			"player_id":   player.ID,
			"player_name": player.Name,
			"score":       scores[player.ID],
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i]["score"].(int) > results[j]["score"].(int)
	})
	return results
}

func buildReveal(game *Game) map[string]any {
	round := currentRound(game)
	if round == nil || game.Phase != phaseResults {
		return nil
	}
	if round.RevealIndex >= len(round.Drawings) {
		return nil
	}
	playerNames := map[int]string{}
	for _, player := range game.Players {
		playerNames[player.ID] = player.Name
	}
	drawing := round.Drawings[round.RevealIndex]
	payload := map[string]any{
		"drawing_index":      round.RevealIndex,
		"drawing_owner":      drawing.PlayerID,
		"drawing_owner_name": playerNames[drawing.PlayerID],
		"drawing_image":      encodeImageData(drawing.ImageData),
		"stage":              round.RevealStage,
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
