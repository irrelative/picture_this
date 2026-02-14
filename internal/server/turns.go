package server

import (
	"sort"
	"strings"
)

func hasGuessForPlayer(round *RoundState, drawingIndex int, playerID int) bool {
	if round == nil {
		return false
	}
	for _, guess := range round.Guesses {
		if guess.DrawingIndex == drawingIndex && guess.PlayerID == playerID {
			return true
		}
	}
	return false
}

func hasVoteForPlayer(round *RoundState, drawingIndex int, playerID int) bool {
	if round == nil {
		return false
	}
	for _, vote := range round.Votes {
		if vote.DrawingIndex == drawingIndex && vote.PlayerID == playerID {
			return true
		}
	}
	return false
}

func hasGuessText(round *RoundState, drawingIndex int, text string) bool {
	if round == nil {
		return false
	}
	for _, guess := range round.Guesses {
		if guess.DrawingIndex == drawingIndex && strings.EqualFold(guess.Text, text) {
			return true
		}
	}
	return false
}

func nextGuessAssignment(game *Game, round *RoundState, playerID int) (int, bool) {
	active := activeGuessDrawingIndex(game, round)
	if active < 0 {
		return -1, false
	}
	if !playerHasID(game, playerID) {
		return -1, false
	}
	drawing := round.Drawings[active]
	if drawing.PlayerID == playerID {
		return -1, false
	}
	if hasGuessForPlayer(round, active, playerID) {
		return -1, false
	}
	return active, true
}

func nextVoteAssignment(game *Game, round *RoundState, playerID int) (int, bool) {
	active := activeVoteDrawingIndex(game, round)
	if active < 0 {
		return -1, false
	}
	if !playerHasID(game, playerID) {
		return -1, false
	}
	drawing := round.Drawings[active]
	if drawing.PlayerID == playerID {
		return -1, false
	}
	if hasVoteForPlayer(round, active, playerID) {
		return -1, false
	}
	return active, true
}

func buildGuessAssignments(game *Game, round *RoundState) map[int]int {
	assignments := make(map[int]int)
	active := activeGuessDrawingIndex(game, round)
	if active < 0 {
		return assignments
	}
	for _, playerID := range pendingGuessersForIndex(game, round, active) {
		assignments[playerID] = active
	}
	return assignments
}

func buildVoteAssignments(game *Game, round *RoundState) map[int]int {
	assignments := make(map[int]int)
	active := activeVoteDrawingIndex(game, round)
	if active < 0 {
		return assignments
	}
	for _, playerID := range pendingVotersForIndex(game, round, active) {
		assignments[playerID] = active
	}
	return assignments
}

func playerHasID(game *Game, playerID int) bool {
	if game == nil {
		return false
	}
	for _, player := range game.Players {
		if player.ID == playerID {
			return true
		}
	}
	return false
}

func pendingGuessersForIndex(game *Game, round *RoundState, drawingIndex int) []int {
	if game == nil || round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return nil
	}
	drawingOwner := round.Drawings[drawingIndex].PlayerID
	pending := make([]int, 0, len(game.Players))
	for _, player := range game.Players {
		if player.ID == drawingOwner {
			continue
		}
		if hasGuessForPlayer(round, drawingIndex, player.ID) {
			continue
		}
		pending = append(pending, player.ID)
	}
	return pending
}

func pendingVotersForIndex(game *Game, round *RoundState, drawingIndex int) []int {
	if game == nil || round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return nil
	}
	drawingOwner := round.Drawings[drawingIndex].PlayerID
	pending := make([]int, 0, len(game.Players))
	for _, player := range game.Players {
		if player.ID == drawingOwner {
			continue
		}
		if hasVoteForPlayer(round, drawingIndex, player.ID) {
			continue
		}
		pending = append(pending, player.ID)
	}
	return pending
}

func activeGuessDrawingIndex(game *Game, round *RoundState) int {
	if game == nil || round == nil || len(round.Drawings) == 0 {
		return -1
	}
	drawingIndex := normalizeDrawingIndex(round)
	if len(pendingGuessersForIndex(game, round, drawingIndex)) > 0 {
		return drawingIndex
	}
	return -1
}

func activeVoteDrawingIndex(game *Game, round *RoundState) int {
	if game == nil || round == nil || len(round.Drawings) == 0 {
		return -1
	}
	drawingIndex := normalizeDrawingIndex(round)
	if len(pendingVotersForIndex(game, round, drawingIndex)) > 0 {
		return drawingIndex
	}
	return -1
}

func assignmentPlayerIDsByOrder(game *Game, assignments map[int]int) []int {
	if game == nil || len(assignments) == 0 {
		return nil
	}
	ids := make([]int, 0, len(assignments))
	for _, player := range game.Players {
		if _, ok := assignments[player.ID]; ok {
			ids = append(ids, player.ID)
		}
	}
	return ids
}

func firstAssignmentByOrder(game *Game, assignments map[int]int) (int, int, bool) {
	if game == nil || len(assignments) == 0 {
		return 0, -1, false
	}
	for _, player := range game.Players {
		if drawingIndex, ok := assignments[player.ID]; ok {
			return player.ID, drawingIndex, true
		}
	}
	return 0, -1, false
}

func requiredGuessCount(game *Game, round *RoundState) int {
	if game == nil || round == nil || len(round.Drawings) == 0 {
		return 0
	}
	count := 0
	for _, drawing := range round.Drawings {
		for _, player := range game.Players {
			if player.ID == drawing.PlayerID {
				continue
			}
			count++
		}
	}
	return count
}

func requiredGuessCountForDrawing(game *Game, round *RoundState, drawingIndex int) int {
	if game == nil || round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return 0
	}
	total := 0
	ownerID := round.Drawings[drawingIndex].PlayerID
	for _, player := range game.Players {
		if player.ID == ownerID {
			continue
		}
		total++
	}
	return total
}

func requiredVoteCount(game *Game, round *RoundState) int {
	if game == nil || round == nil || len(round.Drawings) == 0 {
		return 0
	}
	count := 0
	for _, drawing := range round.Drawings {
		for _, player := range game.Players {
			if player.ID == drawing.PlayerID {
				continue
			}
			count++
		}
	}
	return count
}

func requiredVoteCountForDrawing(game *Game, round *RoundState, drawingIndex int) int {
	if game == nil || round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return 0
	}
	total := 0
	ownerID := round.Drawings[drawingIndex].PlayerID
	for _, player := range game.Players {
		if player.ID == ownerID {
			continue
		}
		total++
	}
	return total
}

func remainingGuessesForPlayer(game *Game, round *RoundState, playerID int) int {
	if game == nil || round == nil {
		return 0
	}
	total := 0
	for drawingIndex, drawing := range round.Drawings {
		if drawing.PlayerID == playerID {
			continue
		}
		total++
		if hasGuessForPlayer(round, drawingIndex, playerID) {
			total--
		}
	}
	if total < 0 {
		return 0
	}
	return total
}

func remainingVotesForPlayer(game *Game, round *RoundState, playerID int) int {
	if game == nil || round == nil {
		return 0
	}
	total := 0
	for drawingIndex, drawing := range round.Drawings {
		if drawing.PlayerID == playerID {
			continue
		}
		total++
		if hasVoteForPlayer(round, drawingIndex, playerID) {
			total--
		}
	}
	if total < 0 {
		return 0
	}
	return total
}

func guessRemainingByPlayer(game *Game, round *RoundState) map[int]int {
	result := make(map[int]int)
	if game == nil || round == nil {
		return result
	}
	drawingIndex := activeGuessDrawingIndex(game, round)
	for _, player := range game.Players {
		if drawingIndex < 0 {
			result[player.ID] = 0
			continue
		}
		if round.Drawings[drawingIndex].PlayerID == player.ID || hasGuessForPlayer(round, drawingIndex, player.ID) {
			result[player.ID] = 0
			continue
		}
		result[player.ID] = 1
	}
	return result
}

func voteRemainingByPlayer(game *Game, round *RoundState) map[int]int {
	result := make(map[int]int)
	if game == nil || round == nil {
		return result
	}
	drawingIndex := activeVoteDrawingIndex(game, round)
	for _, player := range game.Players {
		if drawingIndex < 0 {
			result[player.ID] = 0
			continue
		}
		if round.Drawings[drawingIndex].PlayerID == player.ID || hasVoteForPlayer(round, drawingIndex, player.ID) {
			result[player.ID] = 0
			continue
		}
		result[player.ID] = 1
	}
	return result
}

func pendingGuessersForDrawing(game *Game, assignments map[int]int, drawingIndex int) []int {
	if game == nil || len(assignments) == 0 {
		return nil
	}
	ids := make([]int, 0)
	for _, player := range game.Players {
		if assigned, ok := assignments[player.ID]; ok && assigned == drawingIndex {
			ids = append(ids, player.ID)
		}
	}
	return ids
}

func pendingVotersForDrawing(game *Game, assignments map[int]int, drawingIndex int) []int {
	if game == nil || len(assignments) == 0 {
		return nil
	}
	ids := make([]int, 0)
	for _, player := range game.Players {
		if assigned, ok := assignments[player.ID]; ok && assigned == drawingIndex {
			ids = append(ids, player.ID)
		}
	}
	return ids
}

func guessAssignmentsPayload(game *Game, round *RoundState, assignments map[int]int) []map[string]any {
	if game == nil || round == nil || len(assignments) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(assignments))
	for _, player := range game.Players {
		drawingIndex, ok := assignments[player.ID]
		if !ok {
			continue
		}
		if drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
			continue
		}
		drawing := round.Drawings[drawingIndex]
		payload = append(payload, map[string]any{
			"player_id":       player.ID,
			"drawing_index":   drawingIndex,
			"drawing_owner":   drawing.PlayerID,
			"drawing_image":   encodeImageData(drawing.ImageData),
			"pending_for_one": pendingGuessersForDrawing(game, assignments, drawingIndex),
		})
	}
	return payload
}

func voteAssignmentsPayload(game *Game, round *RoundState, assignments map[int]int) []map[string]any {
	if game == nil || round == nil || len(assignments) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(assignments))
	for _, player := range game.Players {
		drawingIndex, ok := assignments[player.ID]
		if !ok {
			continue
		}
		if drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
			continue
		}
		drawing := round.Drawings[drawingIndex]
		payload = append(payload, map[string]any{
			"player_id":       player.ID,
			"drawing_index":   drawingIndex,
			"drawing_owner":   drawing.PlayerID,
			"drawing_image":   encodeImageData(drawing.ImageData),
			"options":         voteOptionsPayload(voteOptionEntries(round, drawingIndex)),
			"pending_for_one": pendingVotersForDrawing(game, assignments, drawingIndex),
		})
	}
	return payload
}

func voteOptionsPayload(options []VoteOption) []map[string]any {
	if len(options) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(options))
	for _, option := range options {
		result = append(result, map[string]any{
			"id":       option.ID,
			"text":     option.Text,
			"type":     option.Type,
			"owner_id": option.OwnerID,
		})
	}
	return result
}

func sortedUniquePlayerIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		set[id] = struct{}{}
	}
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func revealHasJoke(round *RoundState) bool {
	if round == nil || round.RevealIndex < 0 || round.RevealIndex >= len(round.Drawings) {
		return false
	}
	drawingOwner := round.Drawings[round.RevealIndex].PlayerID
	for _, prompt := range round.Prompts {
		if prompt.PlayerID == drawingOwner && prompt.Joke != "" {
			return true
		}
	}
	return false
}

func normalizeDrawingIndex(round *RoundState) int {
	if round == nil || len(round.Drawings) == 0 {
		return -1
	}
	if round.RevealIndex < 0 || round.RevealIndex >= len(round.Drawings) {
		return 0
	}
	return round.RevealIndex
}
