package server

import (
	"testing"

	"picture-this/internal/config"
)

func TestBuildDisplayStateSubmissionCounters(t *testing.T) {
	srv := New(nil, config.Default())
	game := &Game{
		ID:               "game-1",
		JoinCode:         "ABC123",
		Phase:            phaseGuesses,
		HostID:           1,
		Players:          []Player{{ID: 1, Name: "Ada"}, {ID: 2, Name: "Ben"}, {ID: 3, Name: "Cam"}},
		PromptsPerPlayer: 2,
		Rounds: []RoundState{
			{
				Number: 1,
				Drawings: []DrawingEntry{
					{PlayerID: 1},
					{PlayerID: 2},
				},
				Guesses: []GuessEntry{
					{PlayerID: 2, DrawingIndex: 0, Text: "guess-a"},
				},
				Votes: []VoteEntry{
					{PlayerID: 3, DrawingIndex: 0, ChoiceText: "guess-a", ChoiceType: voteChoiceGuess},
				},
			},
		},
	}

	guessState := srv.buildDisplayState(game)
	if guessState.DrawingSubmitted != 2 || guessState.DrawingRequired != 3 {
		t.Fatalf("unexpected drawing counters: got %d/%d", guessState.DrawingSubmitted, guessState.DrawingRequired)
	}
	if guessState.GuessSubmitted != 1 || guessState.GuessRequired != 2 {
		t.Fatalf("unexpected guess counters: got %d/%d", guessState.GuessSubmitted, guessState.GuessRequired)
	}

	game.Phase = phaseGuessVotes
	voteState := srv.buildDisplayState(game)
	if voteState.VoteSubmitted != 1 || voteState.VoteRequired != 2 {
		t.Fatalf("unexpected vote counters: got %d/%d", voteState.VoteSubmitted, voteState.VoteRequired)
	}
}
