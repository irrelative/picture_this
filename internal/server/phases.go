package server

import (
	"errors"
	"time"
)

type transitionMode int

const (
	transitionPreview transitionMode = iota
	transitionManual
	transitionAuto
)

type phaseTransition struct {
	advance func(s *Server, game *Game, mode transitionMode, at time.Time) (string, error)
}

var phaseTransitions = map[string]phaseTransition{
	phaseLobby: {
		advance: func(s *Server, game *Game, mode transitionMode, at time.Time) (string, error) {
			if mode != transitionPreview && len(game.Rounds) == 0 {
				game.Rounds = append(game.Rounds, RoundState{Number: 1})
			}
			applyPhase(game, phaseDrawings, mode, at)
			return phaseDrawings, nil
		},
	},
	phaseDrawings: {
		advance: func(s *Server, game *Game, mode transitionMode, at time.Time) (string, error) {
			round := currentRound(game)
			if round == nil {
				return "", errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				applyPhase(game, phaseComplete, mode, at)
				return phaseComplete, nil
			}
			applyPhase(game, phaseGuesses, mode, at)
			return phaseGuesses, nil
		},
	},
	phaseGuesses: {
		advance: func(s *Server, game *Game, mode transitionMode, at time.Time) (string, error) {
			round := currentRound(game)
			if round == nil {
				return "", errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				return "", errors.New("no drawings submitted")
			}
			applyPhase(game, phaseGuessVotes, mode, at)
			return phaseGuessVotes, nil
		},
	},
	phaseGuessVotes: {
		advance: func(s *Server, game *Game, mode transitionMode, at time.Time) (string, error) {
			round := currentRound(game)
			if round == nil {
				return "", errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				return "", errors.New("no drawings submitted")
			}
			if mode != transitionPreview {
				initReveal(round, 0)
			}
			applyPhase(game, phaseResults, mode, at)
			return phaseResults, nil
		},
	},
	phaseResults: {
		advance: func(s *Server, game *Game, mode transitionMode, at time.Time) (string, error) {
			round := currentRound(game)
			if round == nil {
				return "", errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				applyPhase(game, phaseComplete, mode, at)
				return phaseComplete, nil
			}

			revealIndex := round.RevealIndex
			revealStage := round.RevealStage
			if revealIndex < 0 || revealIndex >= len(round.Drawings) {
				revealIndex = 0
			}

			nextPhase := phaseResults
			nextRevealIndex := revealIndex
			nextRevealStage := revealStage

			switch revealStage {
			case "":
				nextRevealStage = revealStageGuesses
			case revealStageGuesses:
				nextRevealStage = revealStageVotes
			case revealStageVotes:
				if revealHasJoke(round) {
					nextRevealStage = revealStageJoke
					break
				}
				fallthrough
			case revealStageJoke:
				if revealIndex+1 < len(round.Drawings) {
					nextRevealIndex = revealIndex + 1
					nextRevealStage = revealStageGuesses
					break
				}
				nextRevealStage = ""
				nextRevealIndex = 0
				nextPhase = phaseComplete
				if round.Number < game.PromptsPerPlayer {
					nextPhase = phaseDrawings
				}
			default:
				nextRevealStage = revealStageGuesses
			}

			if mode != transitionPreview {
				round.RevealIndex = nextRevealIndex
				round.RevealStage = nextRevealStage
				if nextPhase == phaseDrawings {
					game.Rounds = append(game.Rounds, RoundState{Number: len(game.Rounds) + 1})
				}
			}
			applyPhase(game, nextPhase, mode, at)
			return nextPhase, nil
		},
	},
}

func (s *Server) nextPhase(game *Game) (string, bool, error) {
	next, err := s.advancePhase(game, transitionPreview, time.Time{})
	if err != nil || next == "" {
		return "", false, err
	}
	return next, true, nil
}

func (s *Server) advancePhase(game *Game, mode transitionMode, at time.Time) (string, error) {
	if game == nil {
		return "", errors.New("game not found")
	}
	transition, ok := phaseTransitions[game.Phase]
	if !ok {
		return "", errors.New("no next phase")
	}
	return transition.advance(s, game, mode, at)
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
	setPhaseAt(game, phase, time.Now().UTC())
}

func setPhaseAt(game *Game, phase string, at time.Time) {
	game.Phase = phase
	if at.IsZero() {
		at = time.Now().UTC()
	}
	game.PhaseStartedAt = at
}

func applyPhase(game *Game, phase string, mode transitionMode, at time.Time) {
	if mode == transitionPreview {
		return
	}
	setPhaseAt(game, phase, at)
}

func initReveal(round *RoundState, drawingIndex int) {
	if round == nil {
		return
	}
	if drawingIndex < 0 {
		drawingIndex = 0
	}
	if drawingIndex >= len(round.Drawings) {
		drawingIndex = 0
	}
	if len(round.Drawings) == 0 {
		round.RevealIndex = 0
		round.RevealStage = ""
		return
	}
	round.RevealIndex = drawingIndex
	round.RevealStage = revealStageGuesses
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
