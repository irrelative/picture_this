package server

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu           sync.Mutex
	nextID       int
	nextPlayerID int
	games        map[string]*Game
}

func NewStore() *Store {
	return &Store{
		nextID:       1,
		nextPlayerID: 1,
		games:        make(map[string]*Game),
	}
}

func (s *Store) CreateGame(promptsPerPlayer int) *Game {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("game-%d", s.nextID)
	s.nextID++
	game := &Game{
		ID:               id,
		JoinCode:         newJoinCode(),
		Phase:            phaseLobby,
		PhaseStartedAt:   timeNowUTC(),
		MaxPlayers:       0,
		LobbyLocked:      false,
		UsedPrompts:      make(map[string]struct{}),
		KickedPlayers:    make(map[string]struct{}),
		PromptsPerPlayer: promptsPerPlayer,
	}
	s.games[id] = game
	return game
}

func (s *Store) GetGame(id string) (*Game, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[id]
	return game, ok
}

func (s *Store) UpdateGame(id string, update func(game *Game) error) (*Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[id]
	if !ok {
		return nil, errors.New("game not found")
	}
	if err := update(game); err != nil {
		return nil, err
	}
	return game, nil
}

func (s *Store) FindGameByJoinCode(code string) (*Game, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, game := range s.games {
		if game.JoinCode == code {
			return game, true
		}
	}
	return nil, false
}

func (s *Store) UpdateGameID(game *Game, newID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if game.ID == newID {
		return
	}
	delete(s.games, game.ID)
	game.ID = newID
	s.games[newID] = game
}

func (s *Store) AddPlayer(gameIDOrCode, name string, avatar []byte) (*Game, *Player, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, ok := s.games[gameIDOrCode]
	if !ok {
		for _, candidate := range s.games {
			if candidate.JoinCode == gameIDOrCode {
				game = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, nil, errors.New("game not found")
	}

	for i := range game.Players {
		if game.Players[i].Name == name {
			if len(avatar) > 0 {
				game.Players[i].Avatar = avatar
			}
			game.Players[i].Claimed = true
			return game, &game.Players[i], nil
		}
	}
	if game.Phase == phasePaused {
		return nil, nil, errors.New("game is paused")
	}
	if game.Phase != phaseLobby {
		return nil, nil, errors.New("game already started")
	}
	if game.LobbyLocked {
		return nil, nil, errors.New("lobby locked")
	}
	if game.MaxPlayers > 0 && len(game.Players) >= game.MaxPlayers {
		return nil, nil, errors.New("lobby full")
	}
	if game.KickedPlayers != nil {
		if _, kicked := game.KickedPlayers[strings.ToLower(name)]; kicked {
			return nil, nil, errors.New("player removed")
		}
	}

	player := Player{
		ID:      s.nextPlayerID,
		Name:    name,
		Avatar:  avatar,
		IsHost:  len(game.Players) == 0,
		Color:   pickPlayerColor(len(game.Players)),
		Claimed: true,
	}
	s.nextPlayerID++
	game.Players = append(game.Players, player)
	if player.IsHost {
		game.HostID = player.ID
	}
	return game, &game.Players[len(game.Players)-1], nil
}

func (s *Store) RestoreGame(game *Game) error {
	if game == nil {
		return errors.New("game is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.games[game.ID]; ok {
		return errors.New("game already running")
	}
	for _, existing := range s.games {
		if existing.JoinCode == game.JoinCode {
			return errors.New("game already running")
		}
	}
	s.games[game.ID] = game
	if id := gameSortKey(game.ID); id >= s.nextID {
		s.nextID = id + 1
	}
	maxPlayerID := 0
	for _, player := range game.Players {
		if player.ID > maxPlayerID {
			maxPlayerID = player.ID
		}
	}
	if maxPlayerID >= s.nextPlayerID {
		s.nextPlayerID = maxPlayerID + 1
	}
	return nil
}

func (s *Store) ListGameSummaries() []GameSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]GameSummary, 0, len(s.games))
	for _, game := range s.games {
		list = append(list, GameSummary{
			ID:       game.ID,
			JoinCode: game.JoinCode,
			Phase:    game.Phase,
			Players:  len(game.Players),
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return gameSortKey(list[i].ID) < gameSortKey(list[j].ID)
	})
	return list
}

func gameSortKey(id string) int {
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		return 0
	}
	value, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return value
}

func (s *Store) GetPlayer(gameID string, playerID int) (*Game, *Player, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[gameID]
	if !ok {
		return nil, nil, false
	}
	for i := range game.Players {
		if game.Players[i].ID == playerID {
			return game, &game.Players[i], true
		}
	}
	return game, nil, false
}

func (s *Store) FindPlayer(game *Game, playerID int) (*Player, bool) {
	for i := range game.Players {
		if game.Players[i].ID == playerID {
			return &game.Players[i], true
		}
	}
	return nil, false
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}
