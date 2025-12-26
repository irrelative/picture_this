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

func drawingIndexAtGuess(round *RoundState, index int) int {
	if round == nil || index < 0 || index >= len(round.GuessTurns) {
		return -1
	}
	return round.GuessTurns[index].DrawingIndex
}

func drawingIndexAtVote(round *RoundState, index int) int {
	if round == nil || index < 0 || index >= len(round.VoteTurns) {
		return -1
	}
	return round.VoteTurns[index].DrawingIndex
}

func endOfGuessBlock(round *RoundState, start int) int {
	if round == nil || start < 0 || start >= len(round.GuessTurns) {
		return start
	}
	drawingIndex := round.GuessTurns[start].DrawingIndex
	i := start
	for i < len(round.GuessTurns) && round.GuessTurns[i].DrawingIndex == drawingIndex {
		i++
	}
	return i
}

func endOfVoteBlock(round *RoundState, start int) int {
	if round == nil || start < 0 || start >= len(round.VoteTurns) {
		return start
	}
	drawingIndex := round.VoteTurns[start].DrawingIndex
	i := start
	for i < len(round.VoteTurns) && round.VoteTurns[i].DrawingIndex == drawingIndex {
		i++
	}
	return i
}

func voteTurnStartIndex(round *RoundState, drawingIndex int) int {
	if round == nil {
		return -1
	}
	for i, turn := range round.VoteTurns {
		if turn.DrawingIndex == drawingIndex {
			return i
		}
	}
	return -1
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
			if len(round.Drawings) == 0 {
				return "", errors.New("no drawings submitted")
			}
			current := round.CurrentGuess
			if current >= len(round.GuessTurns) {
				return "", errors.New("no active guess turn")
			}
			drawingIndex := drawingIndexAtGuess(round, current)
			if drawingIndex < 0 {
				return "", errors.New("invalid guess turn")
			}
			if mode != transitionPreview {
				round.CurrentGuess = endOfGuessBlock(round, current)
				if err := s.buildVoteTurns(game, round); err != nil {
					return "", err
				}
				start := voteTurnStartIndex(round, drawingIndex)
				if start < 0 {
					return "", errors.New("no vote turns for drawing")
				}
				if round.CurrentVote > start {
					return "", errors.New("vote turns out of sync")
				}
				if round.CurrentVote < start {
					round.CurrentVote = start
				}
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
			current := round.CurrentVote
			if current >= len(round.VoteTurns) {
				return "", errors.New("no active vote turn")
			}
			drawingIndex := drawingIndexAtVote(round, current)
			if drawingIndex < 0 {
				return "", errors.New("invalid vote turn")
			}
			if mode != transitionPreview {
				round.CurrentVote = endOfVoteBlock(round, current)
				initReveal(round, drawingIndex)
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
			if round.RevealStage == "" {
				if mode != transitionPreview {
					round.RevealStage = revealStageGuesses
				}
				applyPhase(game, phaseResults, mode, at)
				return phaseResults, nil
			}
			if round.RevealStage == revealStageGuesses {
				if mode != transitionPreview {
					round.RevealStage = revealStageVotes
				}
				applyPhase(game, phaseResults, mode, at)
				return phaseResults, nil
			}
			nextPhase := phaseComplete
			if round.CurrentGuess < len(round.GuessTurns) {
				nextPhase = phaseGuesses
			} else if round.Number < game.PromptsPerPlayer {
				nextPhase = phaseDrawings
			}
			if mode != transitionPreview {
				round.RevealStage = ""
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
	round.RevealIndex = drawingIndex
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
