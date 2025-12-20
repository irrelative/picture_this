package server

import (
	"bytes"
	"encoding/json"
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
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestStartGameConflict(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestSubmitPrompts(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/prompts", map[string]any{
		"player_id": 1,
		"prompts":   []string{"prompt-1"},
	})
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected status %d, got %d", http.StatusGone, resp.StatusCode)
	}
}

func TestSubmitDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{})
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID,
		"image_data": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII=",
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
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{})
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "guess-1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestSubmitVotes(t *testing.T) {
	srv := New(nil, config.Default())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{})
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{
		"player_id": playerID,
		"guess":     "guess-1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
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
