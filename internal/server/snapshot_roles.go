package server

// snapshotForPublic contains only state that is safe for an unauthenticated
// display client. Interactive assignments are always delivered by an
// authenticated role-specific endpoint.
func (s *Server) snapshotForPublic(game *Game) map[string]any {
	snapshot := s.snapshot(game)
	delete(snapshot, "guess_focus")
	delete(snapshot, "vote_focus")
	delete(snapshot, "guess_assignments")
	delete(snapshot, "vote_assignments")
	delete(snapshot, "guess_remaining")
	delete(snapshot, "vote_remaining")
	if game.Phase != phaseComplete {
		delete(snapshot, "results")
	}
	return snapshot
}

func (s *Server) snapshotForPlayer(game *Game, playerID int) map[string]any {
	snapshot := s.snapshot(game)
	snapshot["guess_assignments"] = filterAssignments(snapshot["guess_assignments"], playerID)
	snapshot["vote_assignments"] = filterAssignments(snapshot["vote_assignments"], playerID)
	delete(snapshot, "guess_focus")
	delete(snapshot, "vote_focus")
	delete(snapshot, "guess_remaining")
	delete(snapshot, "vote_remaining")
	if game.Phase != phaseComplete {
		delete(snapshot, "results")
	}
	return snapshot
}

func (s *Server) snapshotForAudience(game *Game) map[string]any {
	snapshot := s.snapshotForPublic(game)
	delete(snapshot, "player_avatars")
	delete(snapshot, "player_avatar_locks")
	return snapshot
}

func filterAssignments(raw any, playerID int) []map[string]any {
	assignments, ok := raw.([]map[string]any)
	if !ok {
		return nil
	}
	for _, assignment := range assignments {
		if id, _ := assignment["player_id"].(int); id == playerID {
			return []map[string]any{assignment}
		}
	}
	return nil
}

type stateChangedMessage struct {
	Type    string `json:"type"`
	Version int64  `json:"version,omitempty"`
}
