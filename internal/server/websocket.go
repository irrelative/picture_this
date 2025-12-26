package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"picture-this/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type gameWSURI struct {
	GameID string `uri:"gameID" binding:"required"`
}

type wsQuery struct {
	Role string `form:"role"`
}

type wsHTMLMessage struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Swap   string `json:"swap"`
	HTML   string `json:"html"`
}

func htmlMessage(target, swap, html string) wsHTMLMessage {
	return wsHTMLMessage{
		Type:   "html",
		Target: target,
		Swap:   swap,
		HTML:   html,
	}
}

type wsHub struct {
	mu      sync.Mutex
	groups  map[string]map[*websocket.Conn]struct{}
	hosts   map[string]map[*websocket.Conn]struct{}
	display map[string]map[*websocket.Conn]struct{}
}

type homeHub struct {
	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		groups:  make(map[string]map[*websocket.Conn]struct{}),
		hosts:   make(map[string]map[*websocket.Conn]struct{}),
		display: make(map[string]map[*websocket.Conn]struct{}),
	}
}

func newHomeHub() *homeHub {
	return &homeHub{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

func (h *wsHub) Add(gameID string, conn *websocket.Conn, role string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if role == "display" {
		group := h.display[gameID]
		if group == nil {
			group = make(map[*websocket.Conn]struct{})
			h.display[gameID] = group
		}
		group[conn] = struct{}{}
		return
	}
	group := h.groups[gameID]
	if group == nil {
		group = make(map[*websocket.Conn]struct{})
		h.groups[gameID] = group
	}
	group[conn] = struct{}{}
	if role == "host" {
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

func (h *wsHub) Remove(gameID string, conn *websocket.Conn, role string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if role == "display" {
		group := h.display[gameID]
		if group == nil {
			return
		}
		delete(group, conn)
		_ = conn.Close()
		if len(group) == 0 {
			delete(h.display, gameID)
		}
		return
	}
	group := h.groups[gameID]
	if group == nil {
		return
	}
	delete(group, conn)
	_ = conn.Close()
	if len(group) == 0 {
		delete(h.groups, gameID)
	}
	if role == "host" {
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

func (h *wsHub) SendHTML(conn *websocket.Conn, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, data)
}

func (h *wsHub) SendDisplay(conn *websocket.Conn, payload any) {
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
			h.Remove(gameID, conn, "player")
		}
	}
}

func (h *wsHub) BroadcastHTML(gameID string, payload any) {
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
			h.Remove(gameID, conn, "player")
		}
	}
}

func (h *wsHub) BroadcastDisplay(gameID string, payload any) {
	h.mu.Lock()
	group := h.display[gameID]
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
			h.Remove(gameID, conn, "display")
		}
	}
}

func (s *Server) handleWebsocket(c *gin.Context) {
	var uri gameWSURI
	if !bindURI(c, &uri) {
		return
	}
	if _, exists := s.store.GetGame(uri.GameID); !exists {
		c.Status(http.StatusNotFound)
		return
	}
	var query wsQuery
	_ = bindQuery(c, &query)
	role := query.Role
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	log.Printf("ws connected game_id=%s remote=%s", uri.GameID, c.Request.RemoteAddr)
	s.ws.Add(uri.GameID, conn, role)
	if game, ok := s.store.GetGame(uri.GameID); ok {
		if role == "display" {
			s.ws.SendDisplay(conn, htmlMessage("#displayContent", "outer", s.renderDisplayHTML(game)))
		} else {
			s.ws.Send(conn, s.snapshot(game))
			s.ws.SendHTML(conn, s.renderGameHTMLMessages(game))
		}
	}
	go s.readWS(uri.GameID, conn, role)
}

func (s *Server) handleHomeWebsocket(c *gin.Context) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	log.Printf("ws connected home remote=%s", c.Request.RemoteAddr)
	s.homeWS.Add(conn)
	s.homeWS.Send(conn, htmlMessage("#activeGamesContent", "inner", s.renderHomeGamesHTML()))
	go s.readHomeWS(conn)
}

func (s *Server) readWS(gameID string, conn *websocket.Conn, role string) {
	defer s.ws.Remove(gameID, conn, role)
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

func (s *Server) broadcastGameUpdate(game *Game) {
	if s.ws == nil {
		return
	}
	s.ws.Broadcast(game.ID, s.snapshot(game))
	s.ws.BroadcastHTML(game.ID, s.renderGameHTMLMessages(game))
	s.ws.BroadcastDisplay(game.ID, htmlMessage("#displayContent", "outer", s.renderDisplayHTML(game)))
	s.broadcastHomeUpdate()
}

func (s *Server) broadcastHomeUpdate() {
	if s.homeWS == nil {
		return
	}
	s.homeWS.Broadcast(htmlMessage("#activeGamesContent", "inner", s.renderHomeGamesHTML()))
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
