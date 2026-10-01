package server

import (
	"encoding/base64"
	"net/http"
	"reflect"
	"testing"
)

func TestVoteOptionIDsAreOpaqueStableAndScoped(t *testing.T) {
	game := securityTestGame()
	round := currentRound(game)
	options := voteOptionEntries(round, 0)
	seen := map[string]bool{}
	for _, option := range options {
		raw, err := base64.RawURLEncoding.DecodeString(option.ID)
		if err != nil || len(raw) != 32 || seen[option.ID] {
			t.Fatalf("invalid opaque ID: %q", option.ID)
		}
		seen[option.ID] = true
	}
	if !reflect.DeepEqual(options, voteOptionEntries(currentRound(cloneGame(game)), 0)) {
		t.Fatal("IDs changed on snapshot/reconnect")
	}
	next := cloneGame(game)
	currentRound(next).Number++
	for _, option := range voteOptionEntries(currentRound(next), 0) {
		if seen[option.ID] {
			t.Fatal("option ID reused in another round")
		}
	}
	round.Drawings = append(round.Drawings, round.Drawings[0])
	for _, guess := range append([]GuessEntry(nil), round.Guesses...) {
		guess.DrawingIndex = 1
		round.Guesses = append(round.Guesses, guess)
	}
	for _, option := range voteOptionEntries(round, 1) {
		if seen[option.ID] {
			t.Fatal("option ID reused for another drawing")
		}
	}
	snapshot := (&Server{}).snapshotForPlayer(game, 1)
	assignments := snapshot["vote_assignments"].([]map[string]any)
	if len(assignments) != 1 {
		t.Fatal("missing player assignment")
	}
	for _, option := range assignments[0]["options"].([]map[string]any) {
		for _, key := range []string{"type", "owner_id"} {
			if _, ok := option[key]; ok {
				t.Fatalf("option leaks %s", key)
			}
		}
	}
}

func TestVotesRejectSemanticIDsAndAcceptOpaqueIDs(t *testing.T) {
	srv, ts := newServerHarness(t)
	gameID, _ := setupThreePlayerRound(t, ts)
	submitAllGuesses(t, ts, gameID)
	game, _ := srv.store.GetGame(gameID)
	round := currentRound(game)
	voterID := pendingVotersForIndex(game, round, 0)[0]
	for _, id := range []string{"prompt", "guess:1"} {
		resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{"player_id": voterID, "choice_id": id})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("semantic ID %q accepted: %d", id, resp.StatusCode)
		}
	}
	var correctID string
	for _, option := range voteOptionEntries(round, 0) {
		if option.Type == voteChoicePrompt {
			correctID = option.ID
		}
	}
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{"player_id": voterID, "choice_id": correctID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("opaque ID rejected: %d", resp.StatusCode)
	}
	game, _ = srv.store.GetGame(gameID)
	votes := currentRound(game).Votes
	if len(votes) != 1 || votes[0].ChoiceType != voteChoicePrompt {
		t.Fatalf("wrong vote resolution: %#v", votes)
	}
}

func TestOpaqueVoteIDsPreserveRevealVotesAndLikes(t *testing.T) {
	game := securityTestGame()
	round := currentRound(game)
	round.Votes = []VoteEntry{{PlayerID: 1, DrawingIndex: 0, ChoiceText: "secret", ChoiceType: voteChoicePrompt}, {PlayerID: 2, DrawingIndex: 0, ChoiceText: "lie one", ChoiceType: voteChoiceGuess}}
	options := voteOptionEntries(round, 0)
	for _, option := range options {
		if option.Text == "lie one" {
			round.AudienceVotes = []AudienceVoteEntry{{AudienceID: 1, DrawingIndex: 0, ChoiceID: option.ID, ChoiceText: option.Text, ChoiceType: option.Type}}
		}
	}
	round.Likes = []LikeEntry{{PlayerID: 2, DrawingIndex: 0, GuessOwnerID: 1}}
	for _, option := range revealOptionsPayload(round, 0, buildNameMap(game.Players)) {
		switch option["text"] {
		case "secret":
			if option["player_vote_count"] != 1 {
				t.Fatal("correct vote missing from reveal")
			}
		case "lie one":
			if option["player_vote_count"] != 1 || option["audience_count"] != 1 || option["like_count"] != 1 {
				t.Fatalf("decoy statistics lost: %#v", option)
			}
		}
	}
}
