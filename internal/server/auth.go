package server

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) authenticatePlayerRequest(c *gin.Context, game *Game, playerID int, authToken string) (*Player, error) {
	if game == nil {
		return nil, errors.New("game not found")
	}
	if playerID <= 0 {
		return nil, errors.New("player_id is required")
	}
	player, ok := s.store.FindPlayer(game, playerID)
	if !ok {
		return nil, errors.New("player not found")
	}
	expected := ensurePlayerAuthToken(game, playerID)
	provided := strings.TrimSpace(authToken)
	if provided != "" {
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1 {
			return player, nil
		}
		return nil, errors.New("invalid player authentication")
	}
	if s.sessions != nil {
		sessionName := normalizeText(s.sessions.GetName(c.Writer, c.Request))
		if sessionName != "" && strings.EqualFold(sessionName, player.Name) {
			return player, nil
		}
	}
	return nil, errors.New("authentication required")
}

func (s *Server) authenticateHostRequest(c *gin.Context, game *Game, playerID int, authToken string) (*Player, error) {
	player, err := s.authenticatePlayerRequest(c, game, playerID, authToken)
	if err != nil {
		return nil, err
	}
	if game.HostID == 0 || player.ID != game.HostID {
		return nil, errors.New("only host can perform this action")
	}
	return player, nil
}
