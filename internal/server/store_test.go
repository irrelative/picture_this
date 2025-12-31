package server

import "testing"

func TestAddPlayerPausedClaimsExisting(t *testing.T) {
	store := NewStore()
	game := store.CreateGame(2)
	game.Phase = phasePaused
	game.Players = append(game.Players, Player{ID: 1, Name: "Ada"})

	_, player, err := store.AddPlayer(game.ID, "Ada", nil)
	if err != nil {
		t.Fatalf("expected claim to succeed, got %v", err)
	}
	if player == nil || player.ID != 1 {
		t.Fatalf("expected existing player, got %#v", player)
	}

	_, _, err = store.AddPlayer(game.ID, "Bob", nil)
	if err == nil || err.Error() != "game is paused" {
		t.Fatalf("expected paused error, got %v", err)
	}
}
