package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"

	"picture-this/internal/web"

	"github.com/gorilla/websocket"
)

type wsHub struct {
	mu     sync.Mutex
	groups map[string]map[*websocket.Conn]struct{}
	hosts  map[string]map[*websocket.Conn]struct{}
}

type homeHub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		groups: make(map[string]map[*websocket.Conn]struct{}),
		hosts:  make(map[string]map[*websocket.Conn]struct{}),
	}
}

func newHomeHub() *homeHub {
	return &homeHub{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

func (h *wsHub) Add(gameID string, conn *websocket.Conn, isHost bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	group := h.groups[gameID]
	if group == nil {
		group = make(map[*websocket.Conn]struct{})
		h.groups[gameID] = group
	}
	group[conn] = struct{}{}
	if isHost {
		hostGroup := h.hosts[gameID]
		if hostGroup == nil {
			hostGroup = make(map[*websocket.Conn]struct{})
			h.hosts[gameID] = hostGroup
		}
		hostGroup[conn] = struct{}{}
	}
}

func (h *homeHub) Add(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn] = struct{}{}
}

func (h *homeHub) Remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, conn)
	_ = conn.Close()
}

func (h *homeHub) Send(conn *websocket.Conn, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (h *homeHub) Broadcast(payload any) {
	h.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for conn := range h.conns {
		conns = append(conns, conn)
	}
	h.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.Remove(conn)
		}
	}
}

func (h *wsHub) Remove(gameID string, conn *websocket.Conn, isHost bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	group := h.groups[gameID]
	if group == nil {
		return
	}
	delete(group, conn)
	_ = conn.Close()
	if len(group) == 0 {
		delete(h.groups, gameID)
	}
	if isHost {
		hostGroup := h.hosts[gameID]
		if hostGroup != nil {
			delete(hostGroup, conn)
			if len(hostGroup) == 0 {
				delete(h.hosts, gameID)
			}
		}
	}
}

func (h *wsHub) Send(conn *websocket.Conn, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (h *wsHub) Broadcast(gameID string, payload any) {
	h.mu.Lock()
	group := h.groups[gameID]
	conns := make([]*websocket.Conn, 0, len(group))
	for conn := range group {
		conns = append(conns, conn)
	}
	h.mu.Unlock()

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			h.Remove(gameID, conn, false)
		}
	}
}

func (s *Server) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	gameID, ok := parseWebsocketPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, exists := s.store.GetGame(gameID); !exists {
		http.NotFound(w, r)
		return
	}
	role := r.URL.Query().Get("role")
	isHost := role == "host"
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	log.Printf("ws connected game_id=%s remote=%s", gameID, r.RemoteAddr)
	s.ws.Add(gameID, conn, isHost)
	if game, ok := s.store.GetGame(gameID); ok {
		s.ws.Send(conn, snapshot(game))
	}
	go s.readWS(gameID, conn, isHost)
}

func (s *Server) handleHomeWebsocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	log.Printf("ws connected home remote=%s", r.RemoteAddr)
	s.homeWS.Add(conn)
	s.homeWS.Send(conn, map[string]any{
		"games": s.homeSummaries(),
	})
	go s.readHomeWS(conn)
}

func (s *Server) readWS(gameID string, conn *websocket.Conn, isHost bool) {
	defer s.ws.Remove(gameID, conn, isHost)
	if isHost {
		defer s.endGameFromHost(gameID)
	}
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("ws disconnected game_id=%s error=%v", gameID, err)
			return
		}
	}
}

func (s *Server) readHomeWS(conn *websocket.Conn) {
	defer s.homeWS.Remove(conn)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			log.Printf("home ws disconnected error=%v", err)
			return
		}
	}
}

func (s *Server) endGameFromHost(gameID string) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase == phaseComplete {
			return errors.New("game already ended")
		}
		setPhase(game, phaseComplete)
		return nil
	})
	if err != nil {
		return
	}
	if err := s.persistPhase(game, "game_ended", map[string]any{"phase": game.Phase}); err != nil {
		return
	}
	log.Printf("game ended game_id=%s reason=host_disconnected", game.ID)
	s.broadcastGameUpdate(game)
	s.cancelPhaseTimer(game.ID)
}

func (s *Server) broadcastGameUpdate(game *Game) {
	if s.ws == nil {
		return
	}
	s.ws.Broadcast(game.ID, snapshot(game))
	s.broadcastHomeUpdate()
}

func (s *Server) broadcastHomeUpdate() {
	if s.homeWS == nil {
		return
	}
	s.homeWS.Broadcast(map[string]any{
		"games": s.homeSummaries(),
	})
}

func (s *Server) homeSummaries() []web.GameSummary {
	summaries := make([]web.GameSummary, 0)
	for _, game := range s.store.ListGameSummaries() {
		summaries = append(summaries, web.GameSummary{
			ID:       game.ID,
			JoinCode: game.JoinCode,
			Phase:    game.Phase,
			Players:  game.Players,
		})
	}
	return summaries
}
