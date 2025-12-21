package server

import (
	"errors"
	"log"
	"time"
)

func (s *Server) schedulePhaseTimer(game *Game) {
	duration := s.phaseDuration(game.Phase)
	if duration <= 0 {
		s.cancelPhaseTimer(game.ID)
		return
	}
	s.timersMu.Lock()
	if existing, ok := s.timers[game.ID]; ok {
		existing.Stop()
	}
	timer := time.AfterFunc(duration, func() {
		s.autoAdvancePhase(game.ID, game.Phase)
	})
	s.timers[game.ID] = timer
	s.timersMu.Unlock()
}

func (s *Server) cancelPhaseTimer(gameID string) {
	s.timersMu.Lock()
	defer s.timersMu.Unlock()
	if timer, ok := s.timers[gameID]; ok {
		timer.Stop()
		delete(s.timers, gameID)
	}
}

func (s *Server) phaseDuration(phase string) time.Duration {
	switch phase {
	case phaseDrawings:
		return time.Duration(s.cfg.DrawDurationSeconds) * time.Second
	case phaseGuesses:
		return time.Duration(s.cfg.GuessDurationSeconds) * time.Second
	case phaseGuessVotes:
		return time.Duration(s.cfg.VoteDurationSeconds) * time.Second
	case phaseResults:
		return time.Duration(s.cfg.RevealDurationSeconds) * time.Second
	default:
		return 0
	}
}

func (s *Server) autoAdvancePhase(gameID string, expectedPhase string) {
	now := time.Now().UTC()
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
		if game.Phase != expectedPhase {
			return errors.New("phase changed")
		}
		switch expectedPhase {
		case phaseDrawings:
			round := currentRound(game)
			if round == nil {
				return errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				game.Phase = phaseComplete
				game.PhaseStartedAt = now
				return nil
			}
			if err := s.buildGuessTurns(game, round); err != nil {
				return err
			}
			game.Phase = phaseGuesses
			game.PhaseStartedAt = now
	case phaseGuesses:
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		round.CurrentGuess = len(round.GuessTurns)
		if err := s.buildVoteTurns(game, round); err != nil {
			return err
		}
		game.Phase = phaseGuessVotes
		game.PhaseStartedAt = now
	case phaseGuessVotes:
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		round.CurrentVote = len(round.VoteTurns)
		if round.Number < game.PromptsPerPlayer {
			game.Phase = phaseDrawings
			game.PhaseStartedAt = now
			game.Rounds = append(game.Rounds, RoundState{
				Number: len(game.Rounds) + 1,
			})
		} else {
			game.Phase = phaseResults
			game.PhaseStartedAt = now
			initReveal(round)
		}
		case phaseResults:
			round := currentRound(game)
			if round == nil {
				return errors.New("round not started")
			}
			if len(round.Drawings) == 0 {
				game.Phase = phaseComplete
				game.PhaseStartedAt = now
				return nil
			}
			if round.RevealStage == "" {
				initReveal(round)
			} else if round.RevealStage == revealStageGuesses {
				round.RevealStage = revealStageVotes
			} else if round.RevealStage == revealStageVotes {
				round.RevealIndex++
				if round.RevealIndex >= len(round.Drawings) {
					game.Phase = phaseComplete
					game.PhaseStartedAt = now
					return nil
				}
				round.RevealStage = revealStageGuesses
			}
		default:
			return errors.New("phase not timed")
		}
		return nil
	})
	if err != nil {
		return
	}
	if game.Phase == phaseDrawings && (expectedPhase == phaseGuesses || expectedPhase == phaseGuessVotes) {
		if err := s.persistRound(game); err != nil {
			log.Printf("auto-advance persist round failed game_id=%s error=%v", game.ID, err)
			return
		}
		if err := s.assignPrompts(game); err != nil {
			log.Printf("auto-advance assign prompts failed game_id=%s error=%v", game.ID, err)
			return
		}
	}
	if game.Phase != expectedPhase {
		if err := s.persistPhase(game, "game_advanced", map[string]any{"phase": game.Phase, "reason": "timeout"}); err != nil {
			log.Printf("auto-advance persist phase failed game_id=%s error=%v", game.ID, err)
			return
		}
		log.Printf("game auto-advanced game_id=%s from=%s to=%s", game.ID, expectedPhase, game.Phase)
	}
	if game.Phase == phaseComplete {
		s.cancelPhaseTimer(game.ID)
	} else {
		s.schedulePhaseTimer(game)
	}
	s.broadcastGameUpdate(game)
}
