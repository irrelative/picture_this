package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
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
	return nil, errors.New("authentication required")
}

func newRecoveryCredential() (string, string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	return code, string(hash), err
}

func verifyRecoveryCredential(hash, code string) bool {
	return hash != "" && code != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
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
