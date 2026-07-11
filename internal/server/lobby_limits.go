package server

func effectiveMaxPlayers(maxPlayers int) int {
	if maxPlayers <= 0 || maxPlayers > maxLobbyPlayers {
		return maxLobbyPlayers
	}
	return maxPlayers
}

func drawfulRoundsForPlayers(players int) int {
	if players >= 7 {
		return 1
	}
	return 2
}
