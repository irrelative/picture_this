package server

import (
	"errors"
	"log"
	"time"
)

func (s *Server) schedulePhaseTimer(game *Game) {
	duration := s.phaseDuration(game)
	if duration <= 0 {
		s.cancelPhaseTimer(game.ID)
		return
	}
	s.timersMu.Lock()
	s.timerGeneration[game.ID]++
	generation := s.timerGeneration[game.ID]
	if existing, ok := s.timers[game.ID]; ok {
		existing.Stop()
	}
	gameID := game.ID
	expectedPhase := game.Phase
	timer := time.AfterFunc(duration, func() {
		s.timersMu.Lock()
		current := s.timerGeneration[gameID]
		s.timersMu.Unlock()
		if current != generation {
			return
		}
		s.autoAdvancePhase(gameID, expectedPhase)
	})
	s.timers[game.ID] = timer
	s.timersMu.Unlock()
}

func (s *Server) cancelPhaseTimer(gameID string) {
	s.timersMu.Lock()
	defer s.timersMu.Unlock()
	s.timerGeneration[gameID]++
	if timer, ok := s.timers[gameID]; ok {
		timer.Stop()
		delete(s.timers, gameID)
	}
}

func (s *Server) phaseDuration(game *Game) time.Duration {
	if game == nil {
		return 0
	}
	switch game.Phase {
	case phaseDrawings:
		return time.Duration(s.cfg.DrawDurationSeconds) * time.Second
	case phaseGuesses:
		return time.Duration(s.cfg.GuessDurationSeconds) * time.Second
	case phaseGuessVotes:
		return time.Duration(s.cfg.VoteDurationSeconds) * time.Second
	case phaseResults:
		round := currentRound(game)
		if round == nil {
			return time.Duration(s.cfg.RevealDurationSeconds) * time.Second
		}
		switch round.RevealStage {
		case revealStageVotes:
			base := s.cfg.RevealVotesSeconds
			if base <= 0 {
				base = s.cfg.RevealDurationSeconds
			}
			return time.Duration(revealVotesStageDurationSeconds(base, round)) * time.Second
		case revealStageJoke:
			if s.cfg.RevealJokeSeconds > 0 {
				return time.Duration(s.cfg.RevealJokeSeconds) * time.Second
			}
		default:
			if s.cfg.RevealGuessesSeconds > 0 {
				return time.Duration(s.cfg.RevealGuessesSeconds) * time.Second
			}
		}
		return time.Duration(s.cfg.RevealDurationSeconds) * time.Second
	default:
		return 0
	}
}

func (s *Server) autoAdvancePhase(gameID string, expectedPhase string) {
	now := time.Now().UTC()
	filledGuesses := make([]autoFilledGuess, 0)
	filledVotes := make([]autoFilledVote, 0)
	game, err := s.store.UpdateGameDurably(gameID, func(game *Game) error {
		if game.Phase != expectedPhase {
			return errors.New("phase changed")
		}
		if expectedPhase == phaseGuesses {
			filledGuesses = append(filledGuesses, autoFillMissingGuesses(game)...)
		}
		if expectedPhase == phaseGuessVotes {
			filledVotes = append(filledVotes, autoFillMissingVotes(game)...)
		}
		_, err := s.advancePhase(game, transitionAuto, now)
		return err
	}, func(game *Game) error {
		return s.persistAdvanceTransaction(game, filledGuesses, filledVotes, expectedPhase, "timeout")
	})
	if err != nil {
		log.Printf("game auto-advance failed game_id=%s phase=%s error=%v", gameID, expectedPhase, err)
		if s.db != nil {
			s.schedulePhaseRetry(gameID, expectedPhase)
		}
		return
	}
	if game.Phase != expectedPhase {
		log.Printf("game auto-advanced game_id=%s from=%s to=%s", game.ID, expectedPhase, game.Phase)
	}
	if game.Phase == phaseComplete {
		s.cancelPhaseTimer(game.ID)
	} else {
		s.schedulePhaseTimer(game)
	}
	s.broadcastGameUpdate(game)
}

func (s *Server) schedulePhaseRetry(gameID, expectedPhase string) {
	s.timersMu.Lock()
	s.timerGeneration[gameID]++
	generation := s.timerGeneration[gameID]
	timer := time.AfterFunc(2*time.Second, func() {
		s.timersMu.Lock()
		current := s.timerGeneration[gameID]
		s.timersMu.Unlock()
		if current == generation {
			s.autoAdvancePhase(gameID, expectedPhase)
		}
	})
	if existing := s.timers[gameID]; existing != nil {
		existing.Stop()
	}
	s.timers[gameID] = timer
	s.timersMu.Unlock()
}
