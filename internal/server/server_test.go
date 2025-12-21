package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"picture-this/internal/config"
)

func TestCreateGame(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp := doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	body := decodeBody(t, resp)
	assertString(t, body["game_id"])
	assertString(t, body["join_code"])
}

func TestHomePage(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp := doRequest(t, ts, http.MethodGet, "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestGameView(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/games/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinView(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp := doRequest(t, ts, http.MethodGet, "/join", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodGet, "/join/ABCD12", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestPlayerView(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ada",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	playerID := int(body["player_id"].(float64))

	resp = doRequest(t, ts, http.MethodGet, "/play/"+gameID+"/"+strconv.Itoa(playerID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestPlayerViewMissingRedirectsWithSession(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/play/game-1/1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if len(resp.Header.Values("Set-Cookie")) == 0 {
		t.Fatalf("expected session cookie to be set")
	}
}

func TestGetGame(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinGameByCode(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	_, joinCode := createGameWithCode(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+joinCode+"/join", map[string]string{
		"name": "Ada",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinGame(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ada",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinRejectsInvalidName(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "<script>alert(1)</script>",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestRateLimitJoin(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	var last *http.Response
	for i := 0; i < 11; i++ {
		last = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
			"name": fmt.Sprintf("Player%d", i),
		})
	}
	if last == nil {
		t.Fatalf("expected response, got nil")
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, last.StatusCode)
	}
}

func TestGuessRejectsInvalidText(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "<nope>",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateSettings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id":       hostID,
		"rounds":          3,
		"max_players":     4,
		"prompt_category": "animals",
		"lobby_locked":    true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["total_rounds"] != float64(3) {
		t.Fatalf("expected rounds 3, got %v", snapshot["total_rounds"])
	}
	if snapshot["max_players"] != float64(4) {
		t.Fatalf("expected max players 4, got %v", snapshot["max_players"])
	}
	if snapshot["prompt_category"] != "animals" {
		t.Fatalf("expected prompt category, got %v", snapshot["prompt_category"])
	}
	if snapshot["lobby_locked"] != true {
		t.Fatalf("expected lobby locked, got %v", snapshot["lobby_locked"])
	}
}

func TestJoinGameLocked(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id":    hostID,
		"lobby_locked": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ben",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestAssignPromptsNoRepeat(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
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

func TestKickPlayerBlocksRejoin(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	playerID := joinPlayer(t, ts, gameID, "Ben")

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/kick", map[string]any{
		"player_id": hostID,
		"target_id": playerID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ben",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestRenamePlayer(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/rename", map[string]any{
		"player_id": playerID,
		"name":      "Ada Prime",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	snapshot := fetchSnapshot(t, ts, gameID)
	players, ok := snapshot["players"].([]any)
	if !ok || len(players) != 1 {
		t.Fatalf("expected one player, got %#v", snapshot["players"])
	}
	if players[0].(string) != "Ada Prime" {
		t.Fatalf("expected renamed player, got %#v", players[0])
	}
}

func TestAudienceJoinAndVote(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id": hostID,
		"rounds":    1,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	prompt1 := fetchPrompt(t, ts, gameID, hostID)
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  hostID,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt1,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID2,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
		"prompt":     prompt2,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID2,
		"guess":     "guess-1",
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": hostID,
		"guess":     "guess-2",
	})

	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience", map[string]any{
		"name": "Viewer",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	audienceID := int(body["audience_id"].(float64))

	options := snapshot["audience_options"].([]any)
	first := options[0].(map[string]any)
	choices := first["options"].([]any)
	choice := choices[0].(string)
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience/votes", map[string]any{
		"audience_id":   audienceID,
		"drawing_index": int(first["drawing_index"].(float64)),
		"choice":        choice,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinSameNameReturnsSamePlayer(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	again := joinPlayer(t, ts, gameID, "Ada")
	if playerID != again {
		t.Fatalf("expected same player id, got %d and %d", playerID, again)
	}
}

func TestStartGame(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestStartGameConflict(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestSubmitDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
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
	ts := httptest.NewServer(srv.Handler())
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
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-2",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "drawings" {
		t.Fatalf("expected drawings phase, got %v", snapshot["phase"])
	}
}

func TestSubmitVotes(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
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
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID2,
		"guess":     "guess-1",
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-2",
	})
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
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-4",
	})
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	for i := 0; i < 4; i++ {
		snapshot = fetchSnapshot(t, ts, gameID)
		if snapshot["phase"] != "guesses-votes" {
			break
		}
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
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
}

func TestAdvanceGame(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestResults(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID+"/results", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestEventsUnavailableWithoutDB(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID+"/events", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestAutoAdvanceFromDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
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

func TestWebsocketUpgradeRequired(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/games/" + gameID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected websocket connection, got error: %v", err)
	}
	_ = conn.Close()
}

func createGame(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp := doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	return body["game_id"].(string)
}

func createGameWithCode(t *testing.T, ts *httptest.Server) (string, string) {
	t.Helper()
	resp := doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	return body["game_id"].(string), body["join_code"].(string)
}

func joinPlayer(t *testing.T, ts *httptest.Server, gameID, name string) int {
	t.Helper()
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": name,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	return int(body["player_id"].(float64))
}

func fetchPrompt(t *testing.T, ts *httptest.Server, gameID string, playerID int) string {
	t.Helper()
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID+"/players/"+strconv.Itoa(playerID)+"/prompt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if prompt, ok := body["prompt"].(string); ok {
		return prompt
	}
	t.Fatalf("expected prompt string, got %#v", body["prompt"])
	return ""
}

func fetchSnapshot(t *testing.T, ts *httptest.Server, gameID string) map[string]any {
	t.Helper()
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	return decodeBody(t, resp)
}

func doRequest(t *testing.T, ts *httptest.Server, method, path string, payload any) *http.Response {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(data)
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func assertString(t *testing.T, value any) {
	t.Helper()
	if _, ok := value.(string); !ok {
		t.Fatalf("expected string, got %T", value)
	}
}
