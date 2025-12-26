package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

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

func TestDisplayView(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/display/"+gameID, nil)
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
		"name":        "Ada",
		"avatar_data": testAvatarData,
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
		"name":        "Ada",
		"avatar_data": testAvatarData,
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
		"name":        "Ada",
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinGameWithoutAvatar(t *testing.T) {
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
		"name":        "<script>alert(1)</script>",
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
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
