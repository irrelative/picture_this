package server

import "picture-this/internal/game"

func effectiveMaxPlayers(maxPlayers int) int {
	if maxPlayers <= 0 || maxPlayers > maxLobbyPlayers {
		return maxLobbyPlayers
	}
	return maxPlayers
}

func drawfulRoundsForPlayers(players int) int {
	return game.AdaptiveRounds(players)
}
