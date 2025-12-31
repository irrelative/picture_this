package server

import (
	"errors"
	"net/http"
	"testing"

	"picture-this/internal/config"
)

func TestAssignPromptsNoRepeat(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id": hostID,
		"rounds":    2,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})

	game, ok := srv.store.GetGame(gameID)
	if !ok {
		t.Fatalf("game not found")
	}
	firstRound := currentRound(game)
	if firstRound == nil || len(firstRound.Prompts) != 2 {
		t.Fatalf("expected prompts in first round")
	}
	firstPrompts := map[string]struct{}{}
	for _, prompt := range firstRound.Prompts {
		firstPrompts[prompt.Text] = struct{}{}
	}

	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		game.Rounds = append(game.Rounds, RoundState{Number: 2})
		return nil
	})
	if err != nil {
		t.Fatalf("update game: %v", err)
	}

	if err := srv.assignPrompts(game); err != nil {
		t.Fatalf("assign prompts: %v", err)
	}
	secondRound := currentRound(game)
	if secondRound == nil || len(secondRound.Prompts) != 2 {
		t.Fatalf("expected prompts in second round")
	}
	for _, prompt := range secondRound.Prompts {
		if _, found := firstPrompts[prompt.Text]; found {
			t.Fatalf("prompt repeated across rounds: %s", prompt.Text)
		}
	}
}

func TestSubmitDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": playerID,
	})
	prompt := fetchPrompt(t, ts, gameID, playerID)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID2,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt2,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestSubmitGuesses(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": playerID,
	})
	prompt := fetchPrompt(t, ts, gameID, playerID)
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID2,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt2,
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-1",
	})
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-OK for wrong turn, got %d", resp.StatusCode)
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID2,
		"guess":     "guess-1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
}

func TestSubmitVotes(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": playerID,
	})
	prompt := fetchPrompt(t, ts, gameID, playerID)
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID2,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt2,
	})
	submitVote := func() {
		snapshot := fetchSnapshot(t, ts, gameID)
		turn, ok := snapshot["vote_turn"].(map[string]any)
		if !ok {
			t.Fatalf("expected vote turn, got %#v", snapshot["vote_turn"])
		}
		voterID := int(turn["voter_id"].(float64))
		optionsRaw, ok := turn["options"].([]any)
		if !ok || len(optionsRaw) == 0 {
			t.Fatalf("expected vote options, got %#v", turn["options"])
		}
		choice, ok := optionsRaw[0].(string)
		if !ok {
			t.Fatalf("expected option string, got %#v", optionsRaw[0])
		}
		resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{
			"player_id": voterID,
			"choice":    choice,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}
	}

	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID2,
		"guess":     "guess-1",
	})
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	submitVote()
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-2",
	})
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	submitVote()
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "drawings" {
		t.Fatalf("expected drawings phase, got %v", snapshot["phase"])
	}
	prompt = fetchPrompt(t, ts, gameID, playerID)
	prompt2 = fetchPrompt(t, ts, gameID, playerID2)
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID2,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt2,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID2,
		"guess":     "guess-3",
	})
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	submitVote()
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-4",
	})
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	submitVote()
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", nil)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "complete" {
		t.Fatalf("expected complete phase, got %v", snapshot["phase"])
	}
}

func TestAutoAdvanceFromDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})

	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		round.Drawings = append(round.Drawings, DrawingEntry{
			PlayerID:  hostID,
			ImageData: []byte{0x01},
			Prompt:    "Test prompt",
		})
		return nil
	})
	if err != nil {
		t.Fatalf("update game: %v", err)
	}

	srv.autoAdvancePhase(gameID, phaseDrawings)
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
	if snapshot["guess_turn"] == nil {
		t.Fatalf("expected guess turn after auto advance")
	}
}
