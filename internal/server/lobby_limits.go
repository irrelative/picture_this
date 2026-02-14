package server

func effectiveMaxPlayers(maxPlayers int) int {
	if maxPlayers <= 0 || maxPlayers > maxLobbyPlayers {
		return maxLobbyPlayers
	}
	return maxPlayers
}
