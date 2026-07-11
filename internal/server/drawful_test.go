package server

import "testing"

func TestDrawfulAdaptiveRounds(t *testing.T) {
	for players, want := range map[int]int{3: 2, 6: 2, 7: 1, 8: 1} {
		if got := drawfulRoundsForPlayers(players); got != want {
			t.Fatalf("players=%d: got %d rounds, want %d", players, got, want)
		}
	}
}

func TestDrawfulScoringRewardsCorrectGuessersAndArtist(t *testing.T) {
	game := &Game{Ruleset: rulesetDrawful, Players: []Player{{ID: 1, Name: "Artist"}, {ID: 2, Name: "Correct"}, {ID: 3, Name: "Fooled"}}, Rounds: []RoundState{{
		Drawings: []DrawingEntry{{PlayerID: 1, Prompt: "real"}},
		Guesses:  []GuessEntry{{PlayerID: 2, DrawingIndex: 0, Text: "lie"}},
		Votes: []VoteEntry{
			{PlayerID: 2, DrawingIndex: 0, ChoiceText: "real", ChoiceType: voteChoicePrompt},
			{PlayerID: 3, DrawingIndex: 0, ChoiceText: "lie", ChoiceType: voteChoiceGuess},
		},
	}}}
	scores := buildScores(game)
	got := map[int]int{}
	for _, score := range scores {
		got[score["player_id"].(int)] = score["score"].(int)
	}
	if got[1] != 500 || got[2] != 1500 || got[3] != 0 {
		t.Fatalf("unexpected Drawful scores: %#v", got)
	}
}
