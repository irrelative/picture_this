package server

import (
	"encoding/json"
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

func TestBuildDisplayStateRevealVoteSequence(t *testing.T) {
	srv := New(nil, config.Default())
	game := &Game{
		ID:               "game-2",
		JoinCode:         "XYZ999",
		Phase:            phaseResults,
		HostID:           1,
		Players:          []Player{{ID: 1, Name: "Ada"}, {ID: 2, Name: "Ben"}, {ID: 3, Name: "Cam"}, {ID: 4, Name: "Dia"}},
		PromptsPerPlayer: 1,
		Rounds: []RoundState{
			{
				Number:      1,
				RevealIndex: 0,
				RevealStage: revealStageVotes,
				Drawings: []DrawingEntry{
					{PlayerID: 1, Prompt: "Real prompt"},
				},
				Guesses: []GuessEntry{
					{PlayerID: 2, DrawingIndex: 0, Text: "lie-b"},
					{PlayerID: 3, DrawingIndex: 0, Text: "lie-c"},
					{PlayerID: 4, DrawingIndex: 0, Text: "lie-d"},
				},
				Votes: []VoteEntry{
					{PlayerID: 2, DrawingIndex: 0, ChoiceText: "lie-c", ChoiceType: voteChoiceGuess},
					{PlayerID: 3, DrawingIndex: 0, ChoiceText: "lie-c", ChoiceType: voteChoiceGuess},
					{PlayerID: 4, DrawingIndex: 0, ChoiceText: "lie-b", ChoiceType: voteChoiceGuess},
				},
			},
		},
	}

	state := srv.buildDisplayState(game)
	var sequence []map[string]any
	if err := json.Unmarshal([]byte(state.RevealVoteSequence), &sequence); err != nil {
		t.Fatalf("unmarshal reveal vote sequence: %v", err)
	}
	if len(sequence) != 3 {
		t.Fatalf("expected 3 reveal entries (2 lies with votes + prompt), got %d", len(sequence))
	}
	if sequence[0]["text"] != "lie-b" || sequence[1]["text"] != "lie-c" {
		t.Fatalf("expected lies ordered by vote count asc, got %#v", sequence)
	}
	if sequence[2]["type"] != voteChoicePrompt || sequence[2]["text"] != "Real prompt" {
		t.Fatalf("expected prompt last, got %#v", sequence[2])
	}
}
