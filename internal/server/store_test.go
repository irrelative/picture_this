package server

import (
	"errors"
	"testing"
)

func TestAddPlayerDurablyRollsBackPersistenceFailure(t *testing.T) {
	store := NewStore()
	game := store.CreateGame(0)
	_, _, err := store.AddPlayerDurably(game.ID, "Ada", nil, "hash", func(*Game, *Player) error {
		return errors.New("database unavailable")
	})
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	got, ok := store.GetGame(game.ID)
	if !ok {
		t.Fatal("game disappeared")
	}
	if len(got.Players) != 0 || got.HostID != 0 {
		t.Fatalf("failed join leaked state: players=%d host=%d", len(got.Players), got.HostID)
	}
	if _, player, retryErr := store.AddPlayer(game.ID, "Ada", nil, "hash"); retryErr != nil || !player.IsHost {
		t.Fatalf("retry failed: player=%+v err=%v", player, retryErr)
	}
}

func TestDurableKickStyleMutationRollsBackPersistenceFailure(t *testing.T) {
	store := NewStore()
	game := store.CreateGame(0)
	_, _, _ = store.AddPlayer(game.ID, "Host", nil, "hash")
	_, _, _ = store.AddPlayer(game.ID, "Ada", nil, "hash")
	_, err := store.UpdateGameDurably(game.ID, func(candidate *Game) error {
		candidate.Players = candidate.Players[:1]
		return nil
	}, func(*Game) error { return errors.New("database unavailable") })
	if err == nil {
		t.Fatal("expected persistence failure")
	}
	got, _ := store.GetGame(game.ID)
	if len(got.Players) != 2 || got.Players[1].Name != "Ada" {
		t.Fatalf("failed kick leaked state: %+v", got.Players)
	}
}

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
