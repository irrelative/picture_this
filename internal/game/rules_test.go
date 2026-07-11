package game

import "testing"

func TestScoresDrawful(t *testing.T) {
	state := State{Ruleset: RulesetDrawful, Players: []int{1, 2, 3}, Rounds: []Round{{
		Drawings: []Drawing{{ArtistID: 1}},
		Lies:     []Lie{{PlayerID: 2, DrawingIndex: 0, Text: "lie"}},
		Votes: []Vote{
			{PlayerID: 2, DrawingIndex: 0, Correct: true},
			{PlayerID: 3, DrawingIndex: 0, ChoiceText: "lie"},
		},
	}}}
	got := Scores(state)
	want := []Score{{PlayerID: 2, Points: 1500}, {PlayerID: 1, Points: 500}, {PlayerID: 3, Points: 0}}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("score %d: got %#v want %#v", i, got[i], want[i])
		}
	}
}
