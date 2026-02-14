package server

import (
	"fmt"
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
	playerAvatarLocks := extractPlayerAvatarLocks(game.Players)
	var guessFocus map[string]any
	var voteFocus map[string]any
	var guessAssignments []map[string]any
	var voteAssignments []map[string]any
	guessRequiredCount := 0
	guessSubmittedCount := 0
	voteRequiredCount := 0
	voteSubmittedCount := 0
	activeGuessDrawing := -1
	activeVoteDrawing := -1
	guessRemaining := map[int]int{}
	voteRemaining := map[int]int{}
	if round := currentRound(game); round != nil {
		guessRequiredCount = requiredGuessCount(game, round)
		guessSubmittedCount = len(round.Guesses)
		voteRequiredCount = requiredVoteCount(game, round)
		voteSubmittedCount = len(round.Votes)
		activeGuessDrawing = activeGuessDrawingIndex(game, round)
		activeVoteDrawing = activeVoteDrawingIndex(game, round)
		guessRemaining = guessRemainingByPlayer(game, round)
		voteRemaining = voteRemainingByPlayer(game, round)
		guessMap := buildGuessAssignments(game, round)
		voteMap := buildVoteAssignments(game, round)
		guessAssignments = guessAssignmentsPayload(game, round, guessMap)
		voteAssignments = voteAssignmentsPayload(game, round, voteMap)
		if guesserID, drawingIndex, ok := firstAssignmentByOrder(game, guessMap); ok {
			pending := pendingGuessersForDrawing(game, guessMap, drawingIndex)
			required := assignmentPlayerIDsByOrder(game, guessMap)
			guessFocus = map[string]any{
				"drawing_index":       drawingIndex,
				"guesser_id":          guesserID,
				"required_player_ids": required,
				"pending_player_ids":  pending,
				"required_count":      guessRequiredCount,
				"submitted_count":     guessSubmittedCount,
			}
			if drawingIndex >= 0 && drawingIndex < len(round.Drawings) {
				drawing := round.Drawings[drawingIndex]
				guessFocus["drawing_owner"] = drawing.PlayerID
				guessFocus["drawing_image"] = encodeImageData(drawing.ImageData)
			}
		}
		if voterID, drawingIndex, ok := firstAssignmentByOrder(game, voteMap); ok {
			pending := pendingVotersForDrawing(game, voteMap, drawingIndex)
			required := assignmentPlayerIDsByOrder(game, voteMap)
			voteFocus = map[string]any{
				"drawing_index":       drawingIndex,
				"voter_id":            voterID,
				"required_player_ids": required,
				"pending_player_ids":  pending,
				"required_count":      voteRequiredCount,
				"submitted_count":     voteSubmittedCount,
			}
			if drawingIndex >= 0 && drawingIndex < len(round.Drawings) {
				drawing := round.Drawings[drawingIndex]
				voteFocus["drawing_owner"] = drawing.PlayerID
				voteFocus["drawing_image"] = encodeImageData(drawing.ImageData)
				voteFocus["options"] = voteOptionsPayload(voteOptionEntries(round, drawingIndex))
			}
		}
	}
	phaseDuration := phaseDurationSeconds(cfg, game)
	phaseEndsAt := ""
	if !game.PhaseStartedAt.IsZero() && phaseDuration > 0 {
		phaseEndsAt = game.PhaseStartedAt.Add(time.Duration(phaseDuration) * time.Second).UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"game_id":               game.ID,
		"join_code":             game.JoinCode,
		"phase":                 game.Phase,
		"paused":                game.Phase == phasePaused,
		"paused_phase":          game.PausedPhase,
		"phase_started_at":      game.PhaseStartedAt,
		"phase_duration":        phaseDuration,
		"phase_ends_at":         phaseEndsAt,
		"players":               players,
		"player_ids":            playerIDs,
		"player_colors":         playerColors,
		"player_avatars":        playerAvatars,
		"player_avatar_locks":   playerAvatarLocks,
		"max_players":           game.MaxPlayers,
		"lobby_locked":          game.LobbyLocked,
		"host_id":               game.HostID,
		"scores":                scores,
		"results":               buildResults(game),
		"reveal":                reveal,
		"total_rounds":          game.PromptsPerPlayer,
		"current_round":         len(game.Rounds),
		"guess_focus":           guessFocus,
		"vote_focus":            voteFocus,
		"guess_assignments":     guessAssignments,
		"vote_assignments":      voteAssignments,
		"guess_required_count":  guessRequiredCount,
		"guess_submitted_count": guessSubmittedCount,
		"guess_active_drawing":  activeGuessDrawing,
		"guess_remaining":       guessRemaining,
		"vote_required_count":   voteRequiredCount,
		"vote_submitted_count":  voteSubmittedCount,
		"vote_active_drawing":   activeVoteDrawing,
		"vote_remaining":        voteRemaining,
		"prompts_per_player":    game.PromptsPerPlayer,
		"counts": map[string]int{
			"prompts":  promptsCount,
			"drawings": drawingsCount,
			"guesses":  guessesCount,
			"votes":    votesCount,
		},
		"can_join":       game.Phase == phaseLobby && !game.LobbyLocked && (game.MaxPlayers == 0 || len(game.Players) < game.MaxPlayers),
		"audience_count": len(game.Audience),
	}
}

func phaseDurationSeconds(cfg config.Config, game *Game) int {
	if game == nil {
		return 0
	}
	switch game.Phase {
	case phaseDrawings:
		return cfg.DrawDurationSeconds
	case phaseGuesses:
		return cfg.GuessDurationSeconds
	case phaseGuessVotes:
		return cfg.VoteDurationSeconds
	case phaseResults:
		round := currentRound(game)
		if round == nil {
			return cfg.RevealDurationSeconds
		}
		switch round.RevealStage {
		case revealStageVotes:
			if cfg.RevealVotesSeconds > 0 {
				return cfg.RevealVotesSeconds
			}
		case revealStageJoke:
			if cfg.RevealJokeSeconds > 0 {
				return cfg.RevealJokeSeconds
			}
		default:
			if cfg.RevealGuessesSeconds > 0 {
				return cfg.RevealGuessesSeconds
			}
		}
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

func extractPlayerAvatarLocks(players []Player) map[int]bool {
	locks := make(map[int]bool, len(players))
	for _, player := range players {
		if player.AvatarLocked {
			locks[player.ID] = true
		}
	}
	return locks
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
	promptJokes := map[int]string{}
	promptJokeAudio := map[int]string{}
	for _, prompt := range round.Prompts {
		if prompt.Joke != "" {
			promptJokes[prompt.PlayerID] = prompt.Joke
		}
		if prompt.JokeAudioPath != "" {
			promptJokeAudio[prompt.PlayerID] = prompt.JokeAudioPath
		}
	}
	results := make([]map[string]any, 0, len(round.Drawings))
	for drawingIndex, drawing := range round.Drawings {
		joke := promptJokes[drawing.PlayerID]
		jokeAudio := promptJokeAudio[drawing.PlayerID]
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
		audienceVotes := audienceVoteBreakdown(round, drawingIndex)
		options := revealOptionsPayload(round, drawingIndex, playerNames)
		scoreDeltas := drawingScoreDeltas(game, round, drawingIndex, playerNames)
		results = append(results, map[string]any{
			"drawing_index":      drawingIndex,
			"drawing_owner":      drawing.PlayerID,
			"drawing_owner_name": playerNames[drawing.PlayerID],
			"drawing_image":      encodeImageData(drawing.ImageData),
			"prompt":             drawing.Prompt,
			"joke":               joke,
			"joke_audio":         jokeAudio,
			"guesses":            guesses,
			"votes":              votes,
			"audience_votes":     audienceVotes,
			"options":            options,
			"score_deltas":       scoreDeltas,
		})
	}
	return results
}

func buildScores(game *Game) []map[string]any {
	scores := map[int]int{}
	for _, player := range game.Players {
		scores[player.ID] = 0
	}
	if game == nil {
		return nil
	}
	for i := range game.Rounds {
		round := &game.Rounds[i]
		if len(round.Drawings) == 0 {
			continue
		}
		for drawingIndex, drawing := range round.Drawings {
			fooledVotes := 0
			for _, vote := range round.Votes {
				if vote.DrawingIndex != drawingIndex {
					continue
				}
				if vote.ChoiceType == voteChoicePrompt {
					scores[vote.PlayerID] += 1000
				} else if vote.ChoiceType == voteChoiceGuess {
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
	joke := ""
	jokeAudio := ""
	for _, prompt := range round.Prompts {
		if prompt.PlayerID != drawing.PlayerID {
			continue
		}
		joke = prompt.Joke
		jokeAudio = prompt.JokeAudioPath
		break
	}
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
	} else if round.RevealStage == revealStageVotes || round.RevealStage == revealStageJoke {
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
		payload["audience_votes"] = audienceVoteBreakdown(round, round.RevealIndex)
		payload["options"] = revealOptionsPayload(round, round.RevealIndex, playerNames)
		payload["score_deltas"] = drawingScoreDeltas(game, round, round.RevealIndex, playerNames)
		if round.RevealStage == revealStageJoke {
			payload["joke"] = joke
			payload["joke_audio"] = jokeAudio
		}
	}
	return payload
}

func revealOptionsPayload(round *RoundState, drawingIndex int, playerNames map[int]string) []map[string]any {
	if round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return nil
	}
	options := voteOptionEntries(round, drawingIndex)
	if len(options) == 0 {
		return nil
	}
	type optionStats struct {
		base         map[string]any
		playerVotes  []map[string]any
		playerCount  int
		audience     int
		totalVoteCnt int
	}
	statsByID := make(map[string]*optionStats, len(options))
	for _, option := range options {
		ownerName := playerNames[option.OwnerID]
		statsByID[option.ID] = &optionStats{
			base: map[string]any{
				"id":         option.ID,
				"text":       option.Text,
				"type":       option.Type,
				"owner_id":   option.OwnerID,
				"owner_name": ownerName,
			},
			playerVotes: make([]map[string]any, 0),
		}
	}

	resolveOptionID := func(choiceType, choiceText, choiceID string) string {
		if choiceID != "" {
			if _, ok := statsByID[choiceID]; ok {
				return choiceID
			}
		}
		if choiceType == voteChoicePrompt {
			if _, ok := statsByID[voteOptionIDPrompt]; ok {
				return voteOptionIDPrompt
			}
		}
		if choiceType == voteChoiceGuess {
			ownerID := guessOwner(round, drawingIndex, choiceText)
			if ownerID > 0 {
				id := fmt.Sprintf("%s%d", voteOptionIDGuess, ownerID)
				if _, ok := statsByID[id]; ok {
					return id
				}
			}
		}
		for _, option := range options {
			if option.Text == choiceText {
				return option.ID
			}
		}
		return ""
	}

	for _, vote := range round.Votes {
		if vote.DrawingIndex != drawingIndex {
			continue
		}
		optionID := resolveOptionID(vote.ChoiceType, vote.ChoiceText, "")
		if optionID == "" {
			continue
		}
		stats := statsByID[optionID]
		stats.playerCount++
		stats.totalVoteCnt++
		stats.playerVotes = append(stats.playerVotes, map[string]any{
			"player_id":   vote.PlayerID,
			"player_name": playerNames[vote.PlayerID],
		})
	}

	for _, vote := range round.AudienceVotes {
		if vote.DrawingIndex != drawingIndex {
			continue
		}
		optionID := resolveOptionID(vote.ChoiceType, vote.ChoiceText, vote.ChoiceID)
		if optionID == "" {
			continue
		}
		stats := statsByID[optionID]
		stats.audience++
		stats.totalVoteCnt++
	}

	result := make([]map[string]any, 0, len(options))
	for _, option := range options {
		stats := statsByID[option.ID]
		entry := map[string]any{
			"id":                stats.base["id"],
			"text":              stats.base["text"],
			"type":              stats.base["type"],
			"owner_id":          stats.base["owner_id"],
			"owner_name":        stats.base["owner_name"],
			"player_votes":      stats.playerVotes,
			"player_vote_count": stats.playerCount,
			"audience_count":    stats.audience,
			"total_count":       stats.totalVoteCnt,
		}
		result = append(result, entry)
	}
	return result
}

func drawingScoreDeltas(game *Game, round *RoundState, drawingIndex int, playerNames map[int]string) []map[string]any {
	if game == nil || round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return nil
	}
	type scoreDelta struct {
		PlayerID int
		Delta    int
		Reasons  []string
	}
	entries := make(map[int]*scoreDelta)
	addDelta := func(playerID int, delta int, reason string) {
		if playerID <= 0 || delta == 0 {
			return
		}
		entry, ok := entries[playerID]
		if !ok {
			entry = &scoreDelta{PlayerID: playerID}
			entries[playerID] = entry
		}
		entry.Delta += delta
		if reason != "" {
			entry.Reasons = append(entry.Reasons, reason)
		}
	}

	fooledVotes := 0
	for _, vote := range round.Votes {
		if vote.DrawingIndex != drawingIndex {
			continue
		}
		if vote.ChoiceType == voteChoicePrompt {
			addDelta(vote.PlayerID, 1000, "Correct vote")
			continue
		}
		if vote.ChoiceType == voteChoiceGuess {
			ownerID := guessOwner(round, drawingIndex, vote.ChoiceText)
			if ownerID != 0 {
				fooledVotes++
				fooledName := playerNames[vote.PlayerID]
				if fooledName == "" {
					fooledName = fmt.Sprintf("Player %d", vote.PlayerID)
				}
				addDelta(ownerID, 500, "Fooled "+fooledName)
			}
		}
	}

	drawingOwner := round.Drawings[drawingIndex].PlayerID
	if fooledVotes == 0 {
		addDelta(drawingOwner, 1000, "No one picked a lie")
	} else {
		addDelta(drawingOwner, 500*fooledVotes, fmt.Sprintf("%d players picked lies", fooledVotes))
	}

	ordered := make([]*scoreDelta, 0, len(entries))
	for _, entry := range entries {
		ordered = append(ordered, entry)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Delta == ordered[j].Delta {
			return ordered[i].PlayerID < ordered[j].PlayerID
		}
		return ordered[i].Delta > ordered[j].Delta
	})

	result := make([]map[string]any, 0, len(ordered))
	for _, entry := range ordered {
		result = append(result, map[string]any{
			"player_id":   entry.PlayerID,
			"player_name": playerNames[entry.PlayerID],
			"delta":       entry.Delta,
			"reasons":     entry.Reasons,
		})
	}
	return result
}

func audienceVoteBreakdown(round *RoundState, drawingIndex int) []map[string]any {
	if round == nil {
		return nil
	}
	counts := make(map[string]map[string]any)
	order := make([]string, 0)
	for _, vote := range round.AudienceVotes {
		if vote.DrawingIndex != drawingIndex {
			continue
		}
		key := vote.ChoiceType + "|" + vote.ChoiceText
		entry, ok := counts[key]
		if !ok {
			entry = map[string]any{
				"text":  vote.ChoiceText,
				"type":  vote.ChoiceType,
				"count": 0,
			}
			counts[key] = entry
			order = append(order, key)
		}
		entry["count"] = entry["count"].(int) + 1
	}
	result := make([]map[string]any, 0, len(order))
	for _, key := range order {
		result = append(result, counts[key])
	}
	return result
}
