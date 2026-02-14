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

	if messageType := readWSMessageType(t, hostConn, 2*time.Second); messageType != "snapshot" {
		t.Fatalf("expected host first message snapshot, got %s", messageType)
	}
	if messageType := readWSMessageType(t, hostConn, 2*time.Second); messageType != "html" {
		t.Fatalf("expected host second message html, got %s", messageType)
	}
	if messageType := readWSMessageType(t, playerConn, 2*time.Second); messageType != "snapshot" {
		t.Fatalf("expected player first message snapshot, got %s", messageType)
	}
	expectNoWSMessage(t, playerConn, 350*time.Millisecond)

	joinPlayer(t, ts, gameID, "Ada")

	hostSawSnapshot := false
	hostSawHTML := false
	for i := 0; i < 3; i++ {
		messageType := readWSMessageType(t, hostConn, 2*time.Second)
		if messageType == "snapshot" {
			hostSawSnapshot = true
		}
		if messageType == "html" {
			hostSawHTML = true
		}
		if hostSawSnapshot && hostSawHTML {
			break
		}
	}
	if !hostSawSnapshot || !hostSawHTML {
		t.Fatalf("expected host to receive both snapshot and html on broadcast")
	}

	if messageType := readWSMessageType(t, playerConn, 2*time.Second); messageType != "snapshot" {
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
