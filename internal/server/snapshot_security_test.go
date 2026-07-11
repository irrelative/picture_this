package server

import "testing"

func TestPublicSnapshotOmitsInteractiveSecrets(t *testing.T) {
	game := securityTestGame()
	srv := &Server{}
	snapshot := srv.snapshotForPublic(game)
	for _, key := range []string{"results", "guess_focus", "vote_focus", "guess_assignments", "vote_assignments", "guess_remaining", "vote_remaining"} {
		if _, exists := snapshot[key]; exists {
			t.Errorf("public snapshot contains secret field %q", key)
		}
	}
}

func TestPlayerVoteOptionsDoNotIdentifyDecoys(t *testing.T) {
	game := securityTestGame()
	srv := &Server{}
	snapshot := srv.snapshotForPlayer(game, 1)
	assignments, _ := snapshot["vote_assignments"].([]map[string]any)
	for _, assignment := range assignments {
		options, _ := assignment["options"].([]map[string]any)
		for _, option := range options {
			for _, forbidden := range []string{"type", "owner_id", "is_decoy"} {
				if _, exists := option[forbidden]; exists {
					t.Errorf("player vote option contains %q", forbidden)
				}
			}
		}
	}
}

func securityTestGame() *Game {
	return &Game{
		ID: "game-1", Phase: phaseGuessVotes, Ruleset: rulesetDrawful,
		Players: []Player{{ID: 1, Name: "Ada"}, {ID: 2, Name: "Ben"}, {ID: 3, Name: "Cam"}},
		Rounds: []RoundState{{Number: 1,
			Drawings: []DrawingEntry{{PlayerID: 3, Prompt: "secret", ImageData: []byte("image")}},
			Guesses:  []GuessEntry{{PlayerID: 1, DrawingIndex: 0, Text: "lie one"}, {PlayerID: 2, DrawingIndex: 0, Text: "lie two"}},
		}},
	}
}
