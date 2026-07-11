package server

import (
	"net/http"
	"testing"
)

func TestDuplicateNameRequiresRecoveryCode(t *testing.T) {
	_, ts := newServerHarness(t)
	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{"name": "Ada"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("join: %d", resp.StatusCode)
	}
	joined := decodeBody(t, resp)
	recoveryCode, _ := joined["recovery_code"].(string)
	if recoveryCode == "" {
		t.Fatal("join did not return a recovery code")
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{"name": "ada"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate join: got %d", resp.StatusCode)
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/players/recover", map[string]string{"name": "Ada", "recovery_code": recoveryCode})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("recover: got %d", resp.StatusCode)
	}
	recovered := decodeBody(t, resp)
	if recovered["recovery_code"] == recoveryCode || recovered["auth_token"] == joined["auth_token"] {
		t.Fatal("recovery did not rotate credentials")
	}
}
