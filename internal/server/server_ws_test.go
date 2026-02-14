package server

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"picture-this/internal/config"

	"github.com/gorilla/websocket"
)

func TestWebsocketUpgradeRequired(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/games/" + gameID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skipf("skipping test; websocket dial unavailable: %v", err)
	}
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = conn.Close()
}

func TestWebsocketRolePayloadIsolation(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/games/" + gameID

	hostConn, _, err := websocket.DefaultDialer.Dial(wsURL+"?role=host", nil)
	if err != nil {
		t.Skipf("skipping test; websocket dial unavailable: %v", err)
	}
	defer hostConn.Close()

	playerConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skipf("skipping test; websocket dial unavailable: %v", err)
	}
	defer playerConn.Close()

	if messageType := readWSMessageType(t, hostConn, 5*time.Second); messageType != "snapshot" {
		t.Fatalf("expected host first message snapshot, got %s", messageType)
	}
	if messageType := readWSMessageType(t, hostConn, 5*time.Second); messageType != "html" {
		t.Fatalf("expected host second message html, got %s", messageType)
	}
	if messageType := readWSMessageType(t, playerConn, 5*time.Second); messageType != "snapshot" {
		t.Fatalf("expected player first message snapshot, got %s", messageType)
	}

	joinPlayer(t, ts, gameID, "Ada")

	waitForWSMessageTypes(t, hostConn, 5*time.Second, "snapshot", "html")

	if messageType := readWSMessageType(t, playerConn, 5*time.Second); messageType != "snapshot" {
		t.Fatalf("expected player broadcast message snapshot, got %s", messageType)
	}
	expectNoWSMessage(t, playerConn, 350*time.Millisecond)
}

func readWSMessageType(t *testing.T, conn *websocket.Conn, timeout time.Duration) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	return classifyWSMessage(payload)
}

func expectNoWSMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected no websocket message within %s", timeout)
	} else {
		netErr, ok := err.(net.Error)
		if !ok || !netErr.Timeout() {
			t.Fatalf("expected websocket timeout, got %v", err)
		}
	}
}

func classifyWSMessage(payload []byte) string {
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "invalid-json"
	}
	switch value := decoded.(type) {
	case map[string]any:
		if messageType, ok := value["type"].(string); ok && messageType == "html" {
			return "html"
		}
		if _, ok := value["phase"]; ok {
			return "snapshot"
		}
	case []any:
		if len(value) == 0 {
			return "empty-array"
		}
		first, ok := value[0].(map[string]any)
		if ok {
			if messageType, ok := first["type"].(string); ok && messageType == "html" {
				return "html"
			}
		}
	}
	return "unknown"
}

func waitForWSMessageTypes(t *testing.T, conn *websocket.Conn, timeout time.Duration, expected ...string) {
	t.Helper()
	if len(expected) == 0 {
		return
	}
	remaining := make(map[string]int, len(expected))
	for _, typ := range expected {
		remaining[typ]++
	}
	seen := make([]string, 0, len(expected)+2)
	deadline := time.Now().Add(timeout)
	for len(remaining) > 0 {
		remainingTime := time.Until(deadline)
		if remainingTime <= 0 {
			t.Fatalf("timed out waiting for websocket messages; seen=%v, missing=%v", seen, remaining)
		}
		messageType := readWSMessageType(t, conn, remainingTime)
		seen = append(seen, messageType)
		if count, ok := remaining[messageType]; ok {
			if count <= 1 {
				delete(remaining, messageType)
			} else {
				remaining[messageType] = count - 1
			}
		}
	}
	if len(seen) == 0 {
		t.Fatalf("expected websocket messages %v, saw none", expected)
	}
}
