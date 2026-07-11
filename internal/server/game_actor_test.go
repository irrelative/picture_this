package server

import (
	"errors"
	"testing"
)

func TestDurableActorCommandPublishesOnlyAfterPersistence(t *testing.T) {
	store := NewStore()
	game := store.CreateGame(2)
	before := game.Version
	_, err := store.UpdateGameDurably(game.ID, func(candidate *Game) error {
		candidate.Phase = phaseDrawings
		return nil
	}, func(*Game) error {
		return errors.New("database unavailable")
	})
	if err == nil {
		t.Fatal("expected persistence error")
	}
	after, _ := store.GetGame(game.ID)
	if after.Phase != phaseLobby || after.Version != before {
		t.Fatalf("failed durable command leaked state: phase=%s version=%d", after.Phase, after.Version)
	}
}
