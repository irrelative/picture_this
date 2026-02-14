package server

import (
	"fmt"
	"strconv"
)

type autoFilledGuess struct {
	PlayerID     int
	DrawingIndex int
	Text         string
}

type autoFilledVote struct {
	PlayerID     int
	RoundNumber  int
	DrawingIndex int
	ChoiceText   string
	ChoiceType   string
}

func autoFillMissingGuesses(game *Game) []autoFilledGuess {
	if game == nil {
		return nil
	}
	round := currentRound(game)
	if round == nil {
		return nil
	}
	filled := make([]autoFilledGuess, 0)
	for drawingIndex := range round.Drawings {
		pending := pendingGuessersForIndex(game, round, drawingIndex)
		for _, playerID := range pending {
			text := autoGuessText(round, drawingIndex, playerID)
			round.Guesses = append(round.Guesses, GuessEntry{
				PlayerID:     playerID,
				DrawingIndex: drawingIndex,
				Text:         text,
			})
			filled = append(filled, autoFilledGuess{
				PlayerID:     playerID,
				DrawingIndex: drawingIndex,
				Text:         text,
			})
		}
	}
	return filled
}

func autoFillMissingVotes(game *Game) []autoFilledVote {
	if game == nil {
		return nil
	}
	round := currentRound(game)
	if round == nil {
		return nil
	}
	filled := make([]autoFilledVote, 0)
	for drawingIndex := range round.Drawings {
		pending := pendingVotersForIndex(game, round, drawingIndex)
		if len(pending) == 0 {
			continue
		}
		choiceText := round.Drawings[drawingIndex].Prompt
		if choiceText == "" {
			choiceText = fmt.Sprintf("Prompt %d", drawingIndex+1)
		}
		for _, playerID := range pending {
			round.Votes = append(round.Votes, VoteEntry{
				PlayerID:     playerID,
				DrawingIndex: drawingIndex,
				ChoiceText:   choiceText,
				ChoiceType:   voteChoicePrompt,
			})
			filled = append(filled, autoFilledVote{
				PlayerID:     playerID,
				RoundNumber:  round.Number,
				DrawingIndex: drawingIndex,
				ChoiceText:   choiceText,
				ChoiceType:   voteChoicePrompt,
			})
		}
	}
	return filled
}

func autoGuessText(round *RoundState, drawingIndex int, playerID int) string {
	base := "Auto guess " + strconv.Itoa(playerID)
	if !hasGuessText(round, drawingIndex, base) {
		return base
	}
	for suffix := 2; suffix < 100; suffix++ {
		candidate := fmt.Sprintf("%s #%d", base, suffix)
		if !hasGuessText(round, drawingIndex, candidate) {
			return candidate
		}
	}
	return base + " fallback"
}
