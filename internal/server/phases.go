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
			if mode == transitionAuto && len(round.Drawings) == 0 {
				applyPhase(game, phaseComplete, mode, at)
				return phaseComplete, nil
			}
			if mode != transitionPreview {
				if err := s.buildGuessTurns(game, round); err != nil {
					return "", err
				}
			} else if len(round.Drawings) == 0 {
				return "", errors.New("no drawings submitted")
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
			if mode != transitionPreview {
				round.CurrentGuess = len(round.GuessTurns)
				if err := s.buildVoteTurns(game, round); err != nil {
					return "", err
				}
			} else if len(round.Drawings) == 0 {
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
			if mode != transitionPreview {
				round.CurrentVote = len(round.VoteTurns)
			}
			if round.Number < game.PromptsPerPlayer {
				if mode != transitionPreview {
					game.Rounds = append(game.Rounds, RoundState{Number: len(game.Rounds) + 1})
				}
				applyPhase(game, phaseDrawings, mode, at)
				return phaseDrawings, nil
			}
			if mode != transitionPreview {
				initReveal(round)
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
			if mode == transitionPreview {
				return phaseComplete, nil
			}
			if len(round.Drawings) == 0 {
				applyPhase(game, phaseComplete, mode, at)
				return phaseComplete, nil
			}
			if round.RevealStage == "" {
				initReveal(round)
			} else if round.RevealStage == revealStageGuesses {
				round.RevealStage = revealStageVotes
			} else if round.RevealStage == revealStageVotes {
				round.RevealIndex++
				if round.RevealIndex >= len(round.Drawings) {
					applyPhase(game, phaseComplete, mode, at)
					return phaseComplete, nil
				}
				round.RevealStage = revealStageGuesses
			}
			applyPhase(game, phaseResults, mode, at)
			return phaseResults, nil
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
