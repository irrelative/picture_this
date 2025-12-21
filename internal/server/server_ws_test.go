package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"picture-this/internal/config"

	"github.com/gorilla/websocket"
)

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
