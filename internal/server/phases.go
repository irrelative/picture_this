package server

import (
	"errors"
	"time"
)

func nextPhase(current string) (string, bool) {
	for i := range phaseOrder {
		if phaseOrder[i] == current {
			if i+1 < len(phaseOrder) {
				return phaseOrder[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

func currentRound(game *Game) *RoundState {
	if len(game.Rounds) == 0 {
		return nil
	}
	return &game.Rounds[len(game.Rounds)-1]
}

func roundByNumber(game *Game, number int) *RoundState {
	if game == nil || number <= 0 {
		return nil
	}
	for i := range game.Rounds {
		if game.Rounds[i].Number == number {
			return &game.Rounds[i]
		}
	}
	return nil
}

func setPhase(game *Game, phase string) {
	game.Phase = phase
	game.PhaseStartedAt = time.Now().UTC()
}

func initReveal(round *RoundState) {
	if round == nil {
		return
	}
	round.RevealIndex = 0
	round.RevealStage = revealStageGuesses
}

func (s *Server) buildGuessTurns(game *Game, round *RoundState) error {
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Drawings) == 0 {
		return errors.New("no drawings submitted")
	}
	if len(round.GuessTurns) > 0 {
		return nil
	}
	round.GuessTurns = nil
	round.CurrentGuess = 0
	for drawingIndex, drawing := range round.Drawings {
		for _, player := range game.Players {
			if player.ID == drawing.PlayerID {
				continue
			}
			round.GuessTurns = append(round.GuessTurns, GuessTurn{
				DrawingIndex: drawingIndex,
				GuesserID:    player.ID,
			})
		}
	}
	if len(round.GuessTurns) == 0 {
		return errors.New("no guess turns available")
	}
	return nil
}

func (s *Server) buildVoteTurns(game *Game, round *RoundState) error {
	if round == nil {
		return errors.New("round not started")
	}
	if len(round.Drawings) == 0 {
		return errors.New("no drawings submitted")
	}
	if len(round.VoteTurns) > 0 {
		return nil
	}
	round.VoteTurns = nil
	round.CurrentVote = 0
	for drawingIndex, drawing := range round.Drawings {
		for _, player := range game.Players {
			if player.ID == drawing.PlayerID {
				continue
			}
			round.VoteTurns = append(round.VoteTurns, VoteTurn{
				DrawingIndex: drawingIndex,
				VoterID:      player.ID,
			})
		}
	}
	if len(round.VoteTurns) == 0 {
		return errors.New("no vote turns available")
	}
	return nil
}

func drawingsComplete(game *Game) bool {
	if game == nil {
		return false
	}
	round := currentRound(game)
	if round == nil {
		return false
	}
	return len(round.Drawings) >= len(game.Players) && len(game.Players) > 0
}

func (s *Server) tryAdvanceToGuesses(gameID string) (bool, *Game, error) {
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != phaseDrawings {
			return nil
		}
		if !drawingsComplete(game) {
			return nil
		}
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		if err := s.buildGuessTurns(game, round); err != nil {
			return err
		}
		setPhase(game, phaseGuesses)
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	if game.Phase != phaseGuesses {
		return false, game, nil
	}
	s.schedulePhaseTimer(game)
	return true, game, nil
}
