package server

import "testing"

func TestAddPlayerPausedRejectsNameTakeover(t *testing.T) {
	store := NewStore()
	game := store.CreateGame(2)
	game.Phase = phasePaused
	game.Players = append(game.Players, Player{ID: 1, Name: "Ada"})

	_, _, err := store.AddPlayer(game.ID, "Ada", nil, "hash")
	if err == nil || err.Error() != "player name already in use" {
		t.Fatalf("expected duplicate-name error, got %v", err)
	}

	_, _, err = store.AddPlayer(game.ID, "Bob", nil, "hash")
	if err == nil || err.Error() != "game is paused" {
		t.Fatalf("expected paused error, got %v", err)
	}
}
