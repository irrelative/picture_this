package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const testAvatarData = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMBAp4pWZkAAAAASUVORK5CYII="

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
		"name":        name,
		"avatar_data": testAvatarData,
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

func doRequestNoRedirect(t *testing.T, ts *httptest.Server, method, path string, payload any) *http.Response {
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
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
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
