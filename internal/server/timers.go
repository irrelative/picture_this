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
			if s.cfg.RevealVotesSeconds > 0 {
				return time.Duration(s.cfg.RevealVotesSeconds) * time.Second
			}
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
	game, err := s.store.UpdateGame(gameID, func(game *Game) error {
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
	})
	if err != nil {
		return
	}
	for _, filled := range filledGuesses {
		if err := s.persistGuess(game, filled.PlayerID, filled.DrawingIndex, filled.Text); err != nil {
			log.Printf("auto-fill persist guess failed game_id=%s player_id=%d error=%v", game.ID, filled.PlayerID, err)
		}
	}
	for _, filled := range filledVotes {
		if err := s.persistVote(game, filled.PlayerID, filled.RoundNumber, filled.DrawingIndex, filled.ChoiceText, filled.ChoiceType); err != nil {
			log.Printf("auto-fill persist vote failed game_id=%s player_id=%d error=%v", game.ID, filled.PlayerID, err)
		}
	}
	if game.Phase == phaseDrawings && expectedPhase != phaseDrawings {
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
		if err := s.persistPhase(game, "game_advanced", EventPayload{Phase: game.Phase, Reason: "timeout"}); err != nil {
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
