package server

import (
	"crypto/rand"
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
	actors       map[string]*gameActor
}

func NewStore() *Store {
	return &Store{
		nextID:       1,
		nextPlayerID: 1,
		games:        make(map[string]*Game),
		actors:       make(map[string]*gameActor),
	}
}

func (s *Store) CreateGame(promptsPerPlayer int) *Game {
	return s.CreateGameWithLimits(promptsPerPlayer, 3, 8)
}

func (s *Store) CreateGameWithLimits(promptsPerPlayer int, minPlayers int, maxPlayers int) *Game {
	s.mu.Lock()
	defer s.mu.Unlock()

	if minPlayers < 2 {
		minPlayers = 2
	}
	if maxPlayers < 0 {
		maxPlayers = 0
	}
	if minPlayers > maxLobbyPlayers {
		minPlayers = maxLobbyPlayers
	}
	if maxPlayers > maxLobbyPlayers {
		maxPlayers = maxLobbyPlayers
	}
	if maxPlayers > 0 && minPlayers > maxPlayers {
		minPlayers = maxPlayers
	}

	id := fmt.Sprintf("game-%d", s.nextID)
	s.nextID++
	game := &Game{
		ID:               id,
		JoinCode:         newJoinCode(),
		Phase:            phaseLobby,
		PhaseStartedAt:   timeNowUTC(),
		MinPlayers:       minPlayers,
		MaxPlayers:       maxPlayers,
		LobbyLocked:      false,
		UsedPrompts:      make(map[string]struct{}),
		KickedPlayers:    make(map[string]struct{}),
		PlayerAuthTokens: make(map[int]string),
		PromptsPerPlayer: promptsPerPlayer,
		Ruleset:          rulesetDrawful,
	}
	s.games[id] = game
	s.actors[id] = newGameActor(game)
	return game
}

func (s *Store) GetGame(id string) (*Game, bool) {
	s.mu.Lock()
	actor, ok := s.actors[id]
	s.mu.Unlock()
	if !ok || actor == nil {
		return nil, false
	}
	return actor.snapshot(), true
}

func (s *Store) DeleteGame(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.games, id)
	delete(s.actors, id)
}

func (s *Store) UpdateGame(id string, update func(game *Game) error) (*Game, error) {
	return s.updateGame(id, update, nil)
}

func (s *Store) UpdateGameDurably(id string, update, persist func(game *Game) error) (*Game, error) {
	return s.updateGame(id, update, persist)
}

func (s *Store) updateGame(id string, update, persist func(game *Game) error) (*Game, error) {
	s.mu.Lock()
	_, ok := s.games[id]
	actor := s.actors[id]
	s.mu.Unlock()
	if !ok {
		return nil, errors.New("game not found")
	}
	if actor == nil {
		return nil, errors.New("game actor not found")
	}
	var err error
	if persist == nil {
		err = actor.execute(update)
	} else {
		err = actor.executeDurably(update, persist)
	}
	if err != nil {
		return nil, err
	}
	return actor.snapshot(), nil
}

func (s *Store) ReplaceGameState(id string, state *Game) (*Game, error) {
	return s.UpdateGame(id, func(game *Game) error {
		replacement := cloneGame(state)
		version := game.Version
		*game = *replacement
		game.Version = version
		return nil
	})
}

func (s *Store) FindGameByJoinCode(code string) (*Game, bool) {
	s.mu.Lock()
	actors := make([]*gameActor, 0, len(s.actors))
	for _, actor := range s.actors {
		actors = append(actors, actor)
	}
	s.mu.Unlock()
	for _, actor := range actors {
		game := actor.snapshot()
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
	actor := s.actors[game.ID]
	delete(s.actors, game.ID)
	game.ID = newID
	s.games[newID] = game
	s.actors[newID] = actor
}

func (s *Store) AddPlayer(gameIDOrCode, name string, avatar []byte, recoveryHash string) (*Game, *Player, error) {
	return s.AddPlayerDurably(gameIDOrCode, name, avatar, recoveryHash, nil)
}

func (s *Store) AddPlayerDurably(gameIDOrCode, name string, avatar []byte, recoveryHash string, persist func(*Game, *Player) error) (*Game, *Player, error) {
	s.mu.Lock()
	_, ok := s.games[gameIDOrCode]
	actor := s.actors[gameIDOrCode]
	if !ok {
		for id, candidate := range s.games {
			if candidate.JoinCode == gameIDOrCode {
				actor = s.actors[id]
				ok = true
				break
			}
		}
	}
	if !ok {
		s.mu.Unlock()
		return nil, nil, errors.New("game not found")
	}
	s.mu.Unlock()

	var joined *Player
	apply := func(game *Game) error {
		for i := range game.Players {
			if strings.EqualFold(game.Players[i].Name, name) {
				return errors.New("player name already in use")
			}
		}
		if game.Phase == phasePaused {
			return errors.New("game is paused")
		}
		if game.Phase != phaseLobby {
			return errors.New("game already started")
		}
		if game.LobbyLocked {
			return errors.New("lobby locked")
		}
		if len(game.Players) >= effectiveMaxPlayers(game.MaxPlayers) {
			return errors.New("lobby full")
		}
		if game.KickedPlayers != nil {
			if _, kicked := game.KickedPlayers[strings.ToLower(name)]; kicked {
				return errors.New("player removed")
			}
		}
		s.mu.Lock()
		reservedPlayerID := s.nextPlayerID
		s.nextPlayerID++
		s.mu.Unlock()
		player := Player{ID: reservedPlayerID, Name: name, Avatar: avatar, IsHost: len(game.Players) == 0, Color: pickPlayerColor(len(game.Players)), Claimed: true, RecoveryHash: recoveryHash}
		game.Players = append(game.Players, player)
		if player.IsHost {
			game.HostID = player.ID
		}
		ensurePlayerAuthToken(game, player.ID)
		joined = &game.Players[len(game.Players)-1]
		return nil
	}
	var err error
	if persist == nil {
		err = actor.execute(apply)
	} else {
		err = actor.executeDurably(apply, func(game *Game) error {
			if joined == nil {
				return errors.New("joined player not found")
			}
			for i := range game.Players {
				if game.Players[i].ID == joined.ID {
					joined = &game.Players[i]
					return persist(game, joined)
				}
			}
			return errors.New("joined player not found")
		})
	}
	if err != nil {
		return nil, nil, err
	}
	result := actor.snapshot()
	for i := range result.Players {
		if result.Players[i].ID == joined.ID {
			return result, &result.Players[i], nil
		}
	}
	return nil, nil, errors.New("joined player not found")
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
	s.actors[game.ID] = newGameActor(game)
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
	actors := make([]*gameActor, 0, len(s.actors))
	for _, actor := range s.actors {
		actors = append(actors, actor)
	}
	s.mu.Unlock()
	list := make([]GameSummary, 0, len(actors))
	for _, actor := range actors {
		game := actor.snapshot()
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
	game, ok := s.GetGame(gameID)
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

func ensurePlayerAuthToken(game *Game, playerID int) string {
	if game == nil || playerID <= 0 {
		return ""
	}
	if game.PlayerAuthTokens == nil {
		game.PlayerAuthTokens = make(map[int]string)
	}
	if token := strings.TrimSpace(game.PlayerAuthTokens[playerID]); token != "" {
		return token
	}
	token := newAuthToken()
	game.PlayerAuthTokens[playerID] = token
	return token
}

func newAuthToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("tok-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buf)
}
