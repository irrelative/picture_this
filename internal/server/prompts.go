package server

import (
	"errors"
	"log"
	"strings"

	"picture-this/internal/db"
)

func promptForPlayer(round *RoundState, playerID int) string {
	if round == nil {
		return ""
	}
	for _, entry := range round.Prompts {
		if entry.PlayerID == playerID {
			return entry.Text
		}
	}
	return ""
}

func findPromptForPlayer(round *RoundState, playerID int, promptText string) (PromptEntry, bool) {
	if round == nil {
		return PromptEntry{}, false
	}
	promptText = strings.TrimSpace(promptText)
	for _, entry := range round.Prompts {
		if entry.PlayerID != playerID {
			continue
		}
		if promptText == "" || entry.Text == promptText {
			return entry, true
		}
	}
	return PromptEntry{}, false
}

func drawingPrompt(round *RoundState, drawingIndex int) string {
	if round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return ""
	}
	return round.Drawings[drawingIndex].Prompt
}

func voteOptionsForDrawing(round *RoundState, drawingIndex int) []string {
	if round == nil || drawingIndex < 0 || drawingIndex >= len(round.Drawings) {
		return nil
	}
	options := make([]string, 0)
	seen := make(map[string]struct{})
	prompt := round.Drawings[drawingIndex].Prompt
	if prompt != "" {
		seen[prompt] = struct{}{}
		options = append(options, prompt)
	}
	for _, guess := range round.Guesses {
		if guess.DrawingIndex != drawingIndex {
			continue
		}
		if _, ok := seen[guess.Text]; ok {
			continue
		}
		seen[guess.Text] = struct{}{}
		options = append(options, guess.Text)
	}
	return options
}

func containsOption(options []string, choice string) bool {
	for _, option := range options {
		if option == choice {
			return true
		}
	}
	return false
}

func guessOwner(round *RoundState, drawingIndex int, text string) int {
	if round == nil {
		return 0
	}
	for _, guess := range round.Guesses {
		if guess.DrawingIndex == drawingIndex && guess.Text == text {
			return guess.PlayerID
		}
	}
	return 0
}

func (s *Server) assignPrompts(game *Game) error {
	round := currentRound(game)
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Prompts) > 0 {
		return nil
	}
	if game.UsedPrompts == nil {
		game.UsedPrompts = make(map[string]struct{})
	}
	total := len(game.Players)
	if total == 0 {
		return errors.New("no players to assign prompts")
	}

	prompts, err := s.loadPromptLibrary(total, game.UsedPrompts)
	if err != nil {
		return err
	}
	if len(prompts) < total {
		return errors.New("not enough prompts available")
	}

	idx := 0
	for _, player := range game.Players {
		prompt := prompts[idx]
		round.Prompts = append(round.Prompts, PromptEntry{
			PlayerID: player.ID,
			Text:     prompt.Text,
			Joke:     prompt.Joke,
		})
		game.UsedPrompts[prompt.Text] = struct{}{}
		idx++
	}
	if err := s.persistAssignedPrompts(game, round); err != nil {
		return err
	}
	if err := s.persistEvent(game, "prompts_assigned", EventPayload{Count: total}); err != nil {
		return err
	}
	log.Printf("prompts assigned game_id=%s count=%d", game.ID, total)
	return nil
}

func (s *Server) loadPromptLibrary(limit int, used map[string]struct{}) ([]db.PromptLibrary, error) {
	if s.db == nil {
		return selectPrompts(fallbackPromptsList(), limit, used), nil
	}
	var records []db.PromptLibrary
	query := s.db
	if len(used) > 0 {
		exclusions := make([]string, 0, len(used))
		for prompt := range used {
			exclusions = append(exclusions, prompt)
		}
		query = query.Where("text NOT IN ?", exclusions)
	}
	if err := query.Order("random()").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return selectPrompts(records, limit, used), nil
}

func fallbackPromptsList() []db.PromptLibrary {
	return []db.PromptLibrary{
		{Text: "A llama in a suit"},
		{Text: "A castle made of pancakes"},
		{Text: "A robot learning to dance"},
		{Text: "A pirate cat at a tea party"},
		{Text: "A rocket powered skateboard"},
		{Text: "A haunted treehouse"},
		{Text: "A snowy beach day"},
		{Text: "A giant sunflower city"},
	}
}

func selectPrompts(pool []db.PromptLibrary, limit int, used map[string]struct{}) []db.PromptLibrary {
	if limit <= 0 {
		return nil
	}
	selected := make([]db.PromptLibrary, 0, limit)
	for _, prompt := range pool {
		if len(selected) >= limit {
			break
		}
		if _, ok := used[prompt.Text]; ok {
			continue
		}
		selected = append(selected, prompt)
	}
	return selected
}
