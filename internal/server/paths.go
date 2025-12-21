package server

import (
	"strconv"
	"strings"
)

func parseGamePath(path string) (string, string, bool) {
	const prefix = "/api/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false
	}
	gameID := parts[0]
	if len(parts) == 1 {
		return gameID, "", true
	}
	if len(parts) == 2 {
		return gameID, parts[1], true
	}
	return "", "", false
}

func parseWebsocketPath(path string) (string, bool) {
	const prefix = "/ws/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return rest, true
}

func parseAudiencePath(path string) (string, string, bool) {
	const prefix = "/api/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 {
		return "", "", false
	}
	if parts[1] != "audience" {
		return "", "", false
	}
	if len(parts) == 2 {
		return parts[0], "", true
	}
	if len(parts) == 3 {
		return parts[0], parts[2], true
	}
	return "", "", false
}

func parseAudienceViewPath(path string) (string, int, bool) {
	const prefix = "/audience/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		return "", 0, false
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, false
	}
	if id <= 0 {
		return "", 0, false
	}
	return parts[0], id, true
}

func parseReplayPath(path string) (string, bool) {
	const prefix = "/replay/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(path, prefix)
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func parsePlayerPath(path string) (string, int, bool) {
	const prefix = "/play/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		return "", 0, false
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, false
	}
	if id <= 0 {
		return "", 0, false
	}
	return parts[0], id, true
}

func parsePlayerPromptPath(path string) (string, int, bool) {
	const prefix = "/api/games/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 4 {
		return "", 0, false
	}
	if parts[1] != "players" || parts[3] != "prompt" {
		return "", 0, false
	}
	playerID, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, false
	}
	if playerID <= 0 {
		return "", 0, false
	}
	return parts[0], playerID, true
}
