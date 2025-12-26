package server

import (
	"bytes"
	"context"
	"html"
	"strconv"

	"picture-this/internal/web"
)

func (s *Server) renderGameHTMLMessages(game *Game) []wsHTMLMessage {
	items := buildPlayerListItems(game)
	return []wsHTMLMessage{
		htmlMessage("#joinCode", "inner", escapeHTML(game.JoinCode)),
		htmlMessage("#gameStatus", "inner", escapeHTML(game.Phase)),
		htmlMessage("#playerList", "inner", s.renderPlayerListHTML(items)),
		htmlMessage("#playerActions", "inner", s.renderPlayerActionsHTML(items, game.Phase == phaseLobby)),
		htmlMessage("#lobbyStatus", "inner", escapeHTML(buildLobbyStatus(game))),
	}
}

func (s *Server) renderPlayerListHTML(items []web.PlayerListItem) string {
	var buf bytes.Buffer
	if err := web.PlayerList(items).Render(context.Background(), &buf); err != nil {
		return ""
	}
	return buf.String()
}

func (s *Server) renderPlayerActionsHTML(items []web.PlayerListItem, actionsEnabled bool) string {
	var buf bytes.Buffer
	if err := web.GamePlayerActions(items, actionsEnabled).Render(context.Background(), &buf); err != nil {
		return ""
	}
	return buf.String()
}

func buildPlayerListItems(game *Game) []web.PlayerListItem {
	items := make([]web.PlayerListItem, 0, len(game.Players))
	for _, player := range game.Players {
		items = append(items, web.PlayerListItem{
			ID:     player.ID,
			Name:   player.Name,
			Avatar: encodeImageData(player.Avatar),
			Color:  player.Color,
			IsHost: player.ID == game.HostID,
		})
	}
	return items
}

func buildLobbyStatus(game *Game) string {
	maxPlayers := "∞"
	if game.MaxPlayers > 0 {
		maxPlayers = strconv.Itoa(game.MaxPlayers)
	}
	locked := "Open"
	if game.LobbyLocked {
		locked = "Locked"
	}
	return "Players: " + strconv.Itoa(len(game.Players)) + "/" + maxPlayers + ". " + locked + " lobby."
}

func escapeHTML(value string) string {
	if value == "" {
		return ""
	}
	return html.EscapeString(value)
}
