package server

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestCreateGame(t *testing.T) {
	_, ts := newServerHarness(t)

	resp := doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	ensureAuthenticatedUser(t, ts)
	resp = doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
	createdResp := resp

	resp = doRequest(t, ts, http.MethodPost, "/api/games", map[string]any{
		"min_players": 2,
		"max_players": 11,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	body := decodeBody(t, createdResp)
	assertString(t, body["game_id"])
	assertString(t, body["join_code"])
	assertString(t, body["auth_token"])
	assertString(t, body["recovery_code"])
	if int(body["player_id"].(float64)) <= 0 {
		t.Fatal("expected creator player id")
	}
}

func TestJoinGameEnforcesTenPlayerCap(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	for i := 1; i <= 10; i++ {
		joinPlayer(t, ts, gameID, "Player"+strconv.Itoa(i))
	}

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Player11",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestRegisterLoginLogoutFlow(t *testing.T) {
	_, ts := newServerHarness(t)

	resp := doRequest(t, ts, http.MethodPost, "/api/auth/register", map[string]any{
		"email":    "host@example.com",
		"username": "Host",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/auth/logout", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    "host@example.com",
		"password": "wrong-password",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/auth/login", map[string]any{
		"email":    "host@example.com",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

func TestRegisterRejectsExistingEmail(t *testing.T) {
	_, ts := newServerHarness(t)
	payload := map[string]any{
		"email":    "host@example.com",
		"username": "Host",
		"password": "password123",
	}

	resp := doRequest(t, ts, http.MethodPost, "/api/auth/register", payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/auth/register", payload)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["error"] != "email is already registered" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestHomePage(t *testing.T) {
	_, ts := newServerHarness(t)

	resp := doRequest(t, ts, http.MethodGet, "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHomeGamesPartialExcludesCompleteGames(t *testing.T) {
	srv, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	if _, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		game.Phase = phaseComplete
		return nil
	}); err != nil {
		t.Fatalf("update game phase: %v", err)
	}

	resp := doRequest(t, ts, http.MethodGet, "/partials/home/games", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)
	if strings.Contains(body, gameID) {
		t.Fatalf("expected completed game %q to be hidden from active list", gameID)
	}
	if !strings.Contains(body, "No active games yet.") {
		t.Fatalf("expected no active games message, got %q", body)
	}
}

func TestGameView(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/games/"+gameID, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestDisplayView(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/display/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAudienceView(t *testing.T) {
	srv, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		game.AudienceEnabled = true
		return nil
	})
	if err != nil {
		t.Fatalf("enable audience: %v", err)
	}
	resp := doRequest(t, ts, http.MethodGet, "/audience/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAdminHomeView(t *testing.T) {
	srv, ts := newServerHarness(t)

	resp := doRequestNoRedirect(t, ts, http.MethodGet, "/admin", nil)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}

	ensureAuthenticatedUser(t, ts)
	promoteSessionUsersToAdmin(t, srv)
	resp = doRequest(t, ts, http.MethodGet, "/admin", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAdminPromptLibraryView(t *testing.T) {
	srv, ts := newServerHarness(t)

	ensureAuthenticatedUser(t, ts)
	promoteSessionUsersToAdmin(t, srv)
	resp := doRequest(t, ts, http.MethodGet, "/admin/prompts", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAdminGameView(t *testing.T) {
	srv, ts := newServerHarness(t)

	ensureAuthenticatedUser(t, ts)
	gameID := createGame(t, ts)
	promoteSessionUsersToAdmin(t, srv)
	resp := doRequest(t, ts, http.MethodGet, "/admin/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAdminResumeRequiresClaims(t *testing.T) {
	srv, ts := newServerHarness(t)

	ensureAuthenticatedUser(t, ts)
	gameID := createGame(t, ts)
	promoteSessionUsersToAdmin(t, srv)
	_ = joinPlayer(t, ts, gameID, "Ada")
	_ = joinPlayer(t, ts, gameID, "Bob")

	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		game.Phase = phasePaused
		game.PausedPhase = phaseDrawings
		for i := range game.Players {
			game.Players[i].Claimed = false
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update game: %v", err)
	}

	resp := doRequestNoRedirect(t, ts, http.MethodPost, "/admin/"+gameID+"/resume", nil)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	game, _ := srv.store.GetGame(gameID)
	if game.Phase != phasePaused {
		t.Fatalf("expected game to remain paused")
	}

	_, err = srv.store.UpdateGame(gameID, func(game *Game) error {
		for i := range game.Players {
			game.Players[i].Claimed = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("claim restored players: %v", err)
	}
	game, _ = srv.store.GetGame(gameID)
	if !allPlayersClaimed(game.Players) {
		t.Fatalf("expected all restored players to be claimed: %#v", game.Players)
	}

	resp = doRequestNoRedirect(t, ts, http.MethodPost, "/admin/"+gameID+"/resume", nil)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	game, _ = srv.store.GetGame(gameID)
	if game.Phase != phaseDrawings {
		t.Fatalf("expected game to resume to drawings, got %s paused=%s", game.Phase, game.PausedPhase)
	}
}

func TestJoinView(t *testing.T) {
	_, ts := newServerHarness(t)

	resp := doRequest(t, ts, http.MethodGet, "/join", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodGet, "/join/ABCD12", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestPlayerView(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name":        "Ada",
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	playerID := int(body["player_id"].(float64))

	resp = doRequest(t, ts, http.MethodGet, "/play/"+gameID+"/"+strconv.Itoa(playerID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestPlayerViewMissingRedirectsWithSession(t *testing.T) {
	_, ts := newServerHarness(t)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/play/game-1/1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if len(resp.Header.Values("Set-Cookie")) == 0 {
		t.Fatalf("expected session cookie to be set")
	}
}

func TestGetGame(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinGameByCode(t *testing.T) {
	_, ts := newServerHarness(t)

	_, joinCode := createGameWithCode(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+joinCode+"/join", map[string]string{
		"name":        "Ada",
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestJoinGame(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name":        "Ada",
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if _, ok := body["auth_token"].(string); !ok {
		t.Fatalf("expected auth_token in join response")
	}
}

func TestJoinGameWithoutAvatar(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ada",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestAvatarSaveIsImmutableInLobby(t *testing.T) {
	srv, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		game.AvatarsEnabled = true
		return nil
	})
	if err != nil {
		t.Fatalf("enable avatars: %v", err)
	}

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/avatar", map[string]any{
		"player_id":   playerID,
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/avatar", map[string]any{
		"player_id":   playerID,
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestJoinRejectsInvalidName(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name":        "<script>alert(1)</script>",
		"avatar_data": testAvatarData,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestGuessRejectsInvalidText(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	playerID := joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": playerID,
		"guess":     "<nope>",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestUpdateSettings(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id":    hostID,
		"rounds":       3,
		"lobby_locked": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["total_rounds"] != float64(3) {
		t.Fatalf("expected rounds 3, got %v", snapshot["total_rounds"])
	}
	if snapshot["max_players"] != float64(0) {
		t.Fatalf("expected max players unchanged, got %v", snapshot["max_players"])
	}
	if snapshot["lobby_locked"] != true {
		t.Fatalf("expected lobby locked, got %v", snapshot["lobby_locked"])
	}
}

func TestJoinGameLocked(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id":    hostID,
		"lobby_locked": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ben",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestKickPlayerBlocksRejoin(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	playerID := joinPlayer(t, ts, gameID, "Ben")

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/kick", map[string]any{
		"player_id": hostID,
		"target_id": playerID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{
		"name": "Ben",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestJoinSameNameRequiresRecovery(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	joinPlayer(t, ts, gameID, "Ada")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/join", map[string]string{"name": "Ada"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestStartGame(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestStartGameConflict(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestAdvanceGame(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestResults(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID+"/results", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestEventsUnavailableWithoutDB(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID := createGame(t, ts)
	resp := doRequest(t, ts, http.MethodGet, "/api/games/"+gameID+"/events", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}
