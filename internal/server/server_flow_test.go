package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"picture-this/internal/config"
)

func TestAssignPromptsNoRepeat(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id": hostID,
		"rounds":    2,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})

	game, ok := srv.store.GetGame(gameID)
	if !ok {
		t.Fatalf("game not found")
	}
	firstRound := currentRound(game)
	if firstRound == nil || len(firstRound.Prompts) != 2 {
		t.Fatalf("expected prompts in first round")
	}
	firstPrompts := map[string]struct{}{}
	for _, prompt := range firstRound.Prompts {
		firstPrompts[prompt.Text] = struct{}{}
	}

	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		game.Rounds = append(game.Rounds, RoundState{Number: 2})
		return nil
	})
	if err != nil {
		t.Fatalf("update game: %v", err)
	}

	if err := srv.assignPrompts(game); err != nil {
		t.Fatalf("assign prompts: %v", err)
	}
	secondRound := currentRound(game)
	if secondRound == nil || len(secondRound.Prompts) != 2 {
		t.Fatalf("expected prompts in second round")
	}
	for _, prompt := range secondRound.Prompts {
		if _, found := firstPrompts[prompt.Text]; found {
			t.Fatalf("prompt repeated across rounds: %s", prompt.Text)
		}
	}
}

func TestSubmitDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{"player_id": hostID})

	prompt1 := fetchPrompt(t, ts, gameID, hostID)
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  hostID,
		"image_data": testAvatarData,
		"prompt":     prompt1,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{
		"player_id":  playerID2,
		"image_data": testAvatarData,
		"prompt":     prompt2,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
}

func TestHostActionsRequireValidAuthToken(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id":  hostID,
		"auth_token": "",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected auth failure conflict, got %d", resp.StatusCode)
	}

	token := getTestAuthToken(gameID, hostID)
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id":  hostID,
		"auth_token": token,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected start success with auth token, got %d", resp.StatusCode)
	}
}

func TestGuessAssignmentsRejectDuplicateLie(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, _ := setupThreePlayerRound(t, ts)
	snapshot := fetchSnapshot(t, ts, gameID)
	assignments := snapshot["guess_assignments"].([]any)
	if len(assignments) == 0 {
		t.Fatalf("expected guess assignments")
	}

	first := assignments[0].(map[string]any)
	firstPlayer := int(first["player_id"].(float64))
	firstDrawing := int(first["drawing_index"].(float64))
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": firstPlayer,
		"guess":     "shared-lie",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected first guess to pass, got %d", resp.StatusCode)
	}

	snapshot = fetchSnapshot(t, ts, gameID)
	assignments = snapshot["guess_assignments"].([]any)
	secondPlayer := 0
	for _, raw := range assignments {
		entry := raw.(map[string]any)
		if int(entry["drawing_index"].(float64)) != firstDrawing {
			continue
		}
		candidate := int(entry["player_id"].(float64))
		if candidate == firstPlayer {
			continue
		}
		secondPlayer = candidate
		break
	}
	if secondPlayer == 0 {
		t.Fatalf("expected another player assigned to same drawing")
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": secondPlayer,
		"guess":     "shared-lie",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate lie conflict, got %d", resp.StatusCode)
	}

	submitAllGuesses(t, ts, gameID)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	if raw, ok := snapshot["vote_assignments"].([]any); !ok || len(raw) == 0 {
		t.Fatalf("expected vote assignments, got %#v", snapshot["vote_assignments"])
	}
}

func TestVoteAssignmentsAdvanceToResults(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, hostID := setupThreePlayerRound(t, ts)
	submitAllGuesses(t, ts, gameID)

	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	submitAllVotes(t, ts, gameID)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	reveal, ok := snapshot["reveal"].(map[string]any)
	if !ok {
		t.Fatalf("expected reveal payload, got %#v", snapshot["reveal"])
	}
	if reveal["stage"] != "guesses" {
		t.Fatalf("expected initial reveal stage guesses, got %v", reveal["stage"])
	}

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected host advance success, got %d", resp.StatusCode)
	}
	snapshot = fetchSnapshot(t, ts, gameID)
	reveal = snapshot["reveal"].(map[string]any)
	if reveal["stage"] != "votes" {
		t.Fatalf("expected votes reveal stage, got %v", reveal["stage"])
	}
	if options, ok := reveal["options"].([]any); !ok || len(options) == 0 {
		t.Fatalf("expected reveal options during vote reveal")
	}
	if deltas, ok := reveal["score_deltas"].([]any); !ok || len(deltas) == 0 {
		t.Fatalf("expected reveal score deltas during vote reveal")
	}
}

func TestSnapshotUsesAssignmentContract(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, _ := setupThreePlayerRound(t, ts)
	snapshot := fetchSnapshot(t, ts, gameID)

	if _, exists := snapshot["guess_turn"]; exists {
		t.Fatalf("unexpected legacy guess_turn in snapshot")
	}
	if _, exists := snapshot["vote_turn"]; exists {
		t.Fatalf("unexpected legacy vote_turn in snapshot")
	}
	if snapshot["guess_focus"] == nil {
		t.Fatalf("expected guess_focus in snapshot")
	}
	assignments, ok := snapshot["guess_assignments"].([]any)
	if !ok || len(assignments) == 0 {
		t.Fatalf("expected guess_assignments in snapshot")
	}
}

func TestResultsJokeStageAndHostAdvanceAuth(t *testing.T) {
	cfg := config.Default()
	srv := New(nil, cfg)
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/settings", map[string]any{
		"player_id": hostID,
		"rounds":    1,
	})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{"player_id": hostID})

	prompt1 := fetchPrompt(t, ts, gameID, hostID)
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{"player_id": hostID, "image_data": testAvatarData, "prompt": prompt1})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{"player_id": playerID2, "image_data": testAvatarData, "prompt": prompt2})

	submitAllGuesses(t, ts, gameID)
	submitAllVotes(t, ts, gameID)

	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		round := currentRound(game)
		if round == nil || round.RevealIndex < 0 || round.RevealIndex >= len(round.Drawings) {
			return errors.New("invalid reveal")
		}
		ownerID := round.Drawings[round.RevealIndex].PlayerID
		for i := range round.Prompts {
			if round.Prompts[i].PlayerID == ownerID {
				round.Prompts[i].Joke = "One-liner"
				round.Prompts[i].JokeAudioPath = "/static/audio/joke.mp3"
				return nil
			}
		}
		return errors.New("prompt not found")
	})
	if err != nil {
		t.Fatalf("update game: %v", err)
	}

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected bad request without player id, got %d", resp.StatusCode)
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{"player_id": playerID2})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected conflict for non-host advance, got %d", resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{"player_id": hostID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected host advance success, got %d", resp.StatusCode)
	}
	snapshot := fetchSnapshot(t, ts, gameID)
	reveal := snapshot["reveal"].(map[string]any)
	if reveal["stage"] != "votes" {
		t.Fatalf("expected votes reveal stage, got %v", reveal["stage"])
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{"player_id": hostID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected host advance success, got %d", resp.StatusCode)
	}
	snapshot = fetchSnapshot(t, ts, gameID)
	reveal = snapshot["reveal"].(map[string]any)
	if reveal["stage"] != "joke" {
		t.Fatalf("expected joke reveal stage, got %v", reveal["stage"])
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{"player_id": hostID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected host advance success, got %d", resp.StatusCode)
	}
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "complete" {
		t.Fatalf("expected complete phase, got %v", snapshot["phase"])
	}
}

func TestAudienceJoinAndVote(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, _ := setupThreePlayerRound(t, ts)
	submitAllGuesses(t, ts, gameID)
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}

	joinResp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience", map[string]any{
		"name": "Spectator",
	})
	if joinResp.StatusCode != http.StatusOK {
		t.Fatalf("expected audience join 200, got %d", joinResp.StatusCode)
	}
	joined := decodeBody(t, joinResp)
	audienceID := int(joined["audience_id"].(float64))
	token := joined["token"].(string)

	assignments, ok := snapshot["vote_assignments"].([]any)
	if !ok || len(assignments) == 0 {
		t.Fatalf("expected vote assignments")
	}
	first := assignments[0].(map[string]any)
	drawingIndex := int(first["drawing_index"].(float64))
	options := first["options"].([]any)
	firstOption := options[0].(map[string]any)
	choiceID := firstOption["id"].(string)
	choiceText := firstOption["text"].(string)

	voteResp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience/votes", map[string]any{
		"audience_id":   audienceID,
		"token":         token,
		"drawing_index": drawingIndex,
		"choice_id":     choiceID,
		"choice":        choiceText,
	})
	if voteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected audience vote 200, got %d", voteResp.StatusCode)
	}
	updated := fetchSnapshot(t, ts, gameID)
	results := updated["results"].([]any)
	entry := results[drawingIndex].(map[string]any)
	if raw, ok := entry["audience_votes"].([]any); !ok || len(raw) == 0 {
		t.Fatalf("expected audience vote breakdown in results")
	}
}

func TestAutoAdvanceFromDrawings(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	joinPlayer(t, ts, gameID, "Ben")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{
		"player_id": hostID,
	})

	_, err := srv.store.UpdateGame(gameID, func(game *Game) error {
		round := currentRound(game)
		if round == nil {
			return errors.New("round not started")
		}
		round.Drawings = append(round.Drawings, DrawingEntry{
			PlayerID:  hostID,
			ImageData: []byte{0x01},
			Prompt:    "Test prompt",
		})
		return nil
	})
	if err != nil {
		t.Fatalf("update game: %v", err)
	}

	srv.autoAdvancePhase(gameID, phaseDrawings)
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
	if raw, ok := snapshot["guess_assignments"].([]any); !ok || len(raw) == 0 {
		t.Fatalf("expected guess assignments after auto advance")
	}
}

func TestGuessAndVoteAssignmentsAreGlobalPerDrawing(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, _ := setupThreePlayerRound(t, ts)
	snapshot := fetchSnapshot(t, ts, gameID)
	assignments, ok := snapshot["guess_assignments"].([]any)
	if !ok || len(assignments) == 0 {
		t.Fatalf("expected guess assignments")
	}
	firstDrawing := int(assignments[0].(map[string]any)["drawing_index"].(float64))
	for _, raw := range assignments {
		entry := raw.(map[string]any)
		if int(entry["drawing_index"].(float64)) != firstDrawing {
			t.Fatalf("expected all guess assignments on the same drawing")
		}
	}
	for _, raw := range assignments {
		entry := raw.(map[string]any)
		playerID := int(entry["player_id"].(float64))
		resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
			"player_id": playerID,
			"guess":     fmt.Sprintf("global-guess-%d-%d", firstDrawing, playerID),
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected guess status 200, got %d", resp.StatusCode)
		}
	}

	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
	assignments, ok = snapshot["guess_assignments"].([]any)
	if !ok || len(assignments) == 0 {
		t.Fatalf("expected additional guess assignments")
	}
	nextDrawing := int(assignments[0].(map[string]any)["drawing_index"].(float64))
	if nextDrawing == firstDrawing {
		t.Fatalf("expected next guess assignments to advance drawings")
	}
	for _, raw := range assignments {
		entry := raw.(map[string]any)
		if int(entry["drawing_index"].(float64)) != nextDrawing {
			t.Fatalf("expected all guess assignments on the same drawing after advance")
		}
	}

	submitAllGuesses(t, ts, gameID)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}

	voteAssignments, ok := snapshot["vote_assignments"].([]any)
	if !ok || len(voteAssignments) == 0 {
		t.Fatalf("expected vote assignments")
	}
	voteDrawing := int(voteAssignments[0].(map[string]any)["drawing_index"].(float64))
	for _, raw := range voteAssignments {
		entry := raw.(map[string]any)
		if int(entry["drawing_index"].(float64)) != voteDrawing {
			t.Fatalf("expected all vote assignments on the same drawing")
		}
	}
}

func TestAutoAdvanceAutoFillsMissingGuessesAndVotes(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, _ := setupThreePlayerRound(t, ts)

	srv.autoAdvancePhase(gameID, phaseGuesses)
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase after auto-fill, got %v", snapshot["phase"])
	}

	game, ok := srv.store.GetGame(gameID)
	if !ok {
		t.Fatalf("game not found")
	}
	round := currentRound(game)
	if round == nil {
		t.Fatalf("round not found")
	}
	if len(round.Guesses) != requiredGuessCount(game, round) {
		t.Fatalf("expected auto-filled guesses to satisfy requirement, got %d", len(round.Guesses))
	}

	srv.autoAdvancePhase(gameID, phaseGuessVotes)
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase after auto-fill votes, got %v", snapshot["phase"])
	}

	game, ok = srv.store.GetGame(gameID)
	if !ok {
		t.Fatalf("game not found")
	}
	round = currentRound(game)
	if round == nil {
		t.Fatalf("round not found")
	}
	if len(round.Votes) != requiredVoteCount(game, round) {
		t.Fatalf("expected auto-filled votes to satisfy requirement, got %d", len(round.Votes))
	}
}

func TestManualAdvanceAutoFillsMissingGuessesAndVotes(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, hostID := setupThreePlayerRound(t, ts)
	snapshot := fetchSnapshot(t, ts, gameID)
	assignments, ok := snapshot["guess_assignments"].([]any)
	if !ok || len(assignments) == 0 {
		t.Fatalf("expected guess assignments")
	}
	firstGuess := assignments[0].(map[string]any)
	guesserID := int(firstGuess["player_id"].(float64))
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": guesserID,
		"guess":     "manual-advance-guess",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected guess status 200, got %d", resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected manual advance status 200, got %d", resp.StatusCode)
	}
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses-votes" {
		t.Fatalf("expected guesses-votes phase, got %v", snapshot["phase"])
	}
	game, ok := srv.store.GetGame(gameID)
	if !ok {
		t.Fatalf("game not found")
	}
	round := currentRound(game)
	if round == nil {
		t.Fatalf("round not found")
	}
	if len(round.Guesses) != requiredGuessCount(game, round) {
		t.Fatalf("expected manual advance to auto-fill guesses, got %d", len(round.Guesses))
	}

	snapshot = fetchSnapshot(t, ts, gameID)
	voteAssignments, ok := snapshot["vote_assignments"].([]any)
	if !ok || len(voteAssignments) == 0 {
		t.Fatalf("expected vote assignments")
	}
	firstVote := voteAssignments[0].(map[string]any)
	voterID := int(firstVote["player_id"].(float64))
	optionsRaw, ok := firstVote["options"].([]any)
	if !ok || len(optionsRaw) == 0 {
		t.Fatalf("expected vote options")
	}
	var selected map[string]any
	for _, raw := range optionsRaw {
		option, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ownerID := int(option["owner_id"].(float64))
		optionType, _ := option["type"].(string)
		if optionType == "guess" && ownerID == voterID {
			continue
		}
		selected = option
		break
	}
	if selected == nil {
		t.Fatalf("expected at least one valid vote option")
	}
	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{
		"player_id": voterID,
		"choice_id": selected["id"],
		"choice":    selected["text"],
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected vote status 200, got %d", resp.StatusCode)
	}

	resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/advance", map[string]any{
		"player_id": hostID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected manual advance status 200, got %d", resp.StatusCode)
	}
	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "results" {
		t.Fatalf("expected results phase, got %v", snapshot["phase"])
	}
	game, ok = srv.store.GetGame(gameID)
	if !ok {
		t.Fatalf("game not found")
	}
	round = currentRound(game)
	if round == nil {
		t.Fatalf("round not found")
	}
	if len(round.Votes) != requiredVoteCount(game, round) {
		t.Fatalf("expected manual advance to auto-fill votes, got %d", len(round.Votes))
	}
}

func TestAudienceJoinUsesTokenIdentity(t *testing.T) {
	srv := New(nil, config.Default())
	ts := newTestServer(t, srv.Handler())
	t.Cleanup(ts.Close)

	gameID, _ := setupThreePlayerRound(t, ts)

	joinOneResp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience", map[string]any{
		"name": "Spectator",
	})
	if joinOneResp.StatusCode != http.StatusOK {
		t.Fatalf("expected first audience join 200, got %d", joinOneResp.StatusCode)
	}
	first := decodeBody(t, joinOneResp)
	firstID := int(first["audience_id"].(float64))
	firstToken := first["token"].(string)

	joinTwoResp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience", map[string]any{
		"name": "Spectator",
	})
	if joinTwoResp.StatusCode != http.StatusOK {
		t.Fatalf("expected second audience join 200, got %d", joinTwoResp.StatusCode)
	}
	second := decodeBody(t, joinTwoResp)
	secondID := int(second["audience_id"].(float64))
	if firstID == secondID {
		t.Fatalf("expected same-name join without token to create a new audience member")
	}

	joinWithTokenResp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/audience", map[string]any{
		"name":  "Spectator Prime",
		"token": firstToken,
	})
	if joinWithTokenResp.StatusCode != http.StatusOK {
		t.Fatalf("expected token audience join 200, got %d", joinWithTokenResp.StatusCode)
	}
	third := decodeBody(t, joinWithTokenResp)
	thirdID := int(third["audience_id"].(float64))
	if thirdID != firstID {
		t.Fatalf("expected token join to reclaim original audience member")
	}
	if third["audience_name"] != "Spectator Prime" {
		t.Fatalf("expected token join to update audience name, got %v", third["audience_name"])
	}

	snapshot := fetchSnapshot(t, ts, gameID)
	if count := int(snapshot["audience_count"].(float64)); count != 2 {
		t.Fatalf("expected audience_count=2, got %d", count)
	}
}

func setupThreePlayerRound(t *testing.T, ts *httptest.Server) (string, int) {
	t.Helper()
	gameID := createGame(t, ts)
	hostID := joinPlayer(t, ts, gameID, "Ada")
	playerID2 := joinPlayer(t, ts, gameID, "Ben")
	playerID3 := joinPlayer(t, ts, gameID, "Cam")
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{"player_id": hostID})

	prompt1 := fetchPrompt(t, ts, gameID, hostID)
	prompt2 := fetchPrompt(t, ts, gameID, playerID2)
	prompt3 := fetchPrompt(t, ts, gameID, playerID3)
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{"player_id": hostID, "image_data": testAvatarData, "prompt": prompt1})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{"player_id": playerID2, "image_data": testAvatarData, "prompt": prompt2})
	doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/drawings", map[string]any{"player_id": playerID3, "image_data": testAvatarData, "prompt": prompt3})

	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != "guesses" {
		t.Fatalf("expected guesses phase, got %v", snapshot["phase"])
	}
	return gameID, hostID
}

func submitAllGuesses(t *testing.T, ts *httptest.Server, gameID string) {
	t.Helper()
	for guard := 0; guard < 12; guard++ {
		snapshot := fetchSnapshot(t, ts, gameID)
		if snapshot["phase"] != "guesses" {
			return
		}
		assignmentsRaw, ok := snapshot["guess_assignments"].([]any)
		if !ok || len(assignmentsRaw) == 0 {
			continue
		}
		for _, raw := range assignmentsRaw {
			entry := raw.(map[string]any)
			playerID := int(entry["player_id"].(float64))
			drawingIndex := int(entry["drawing_index"].(float64))
			resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
				"player_id": playerID,
				"guess":     fmt.Sprintf("guess-%d-%d-%d", drawingIndex, playerID, guard),
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected guess status 200, got %d", resp.StatusCode)
			}
		}
	}
	t.Fatalf("guess phase did not complete")
}

func submitAllVotes(t *testing.T, ts *httptest.Server, gameID string) {
	t.Helper()
	for guard := 0; guard < 12; guard++ {
		snapshot := fetchSnapshot(t, ts, gameID)
		if snapshot["phase"] != "guesses-votes" {
			return
		}
		assignmentsRaw, ok := snapshot["vote_assignments"].([]any)
		if !ok || len(assignmentsRaw) == 0 {
			continue
		}
		for _, raw := range assignmentsRaw {
			entry := raw.(map[string]any)
			playerID := int(entry["player_id"].(float64))
			optionsRaw, ok := entry["options"].([]any)
			if !ok || len(optionsRaw) == 0 {
				t.Fatalf("expected vote options")
			}
			var selected map[string]any
			for _, optionRaw := range optionsRaw {
				option, ok := optionRaw.(map[string]any)
				if !ok {
					continue
				}
				ownerID := int(option["owner_id"].(float64))
				optionType, _ := option["type"].(string)
				if optionType == "guess" && ownerID == playerID {
					continue
				}
				selected = option
				break
			}
			if selected == nil {
				t.Fatalf("expected at least one valid vote option for player %d", playerID)
			}
			choiceID, _ := selected["id"].(string)
			choiceText, _ := selected["text"].(string)
			resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{
				"player_id": playerID,
				"choice_id": choiceID,
				"choice":    choiceText,
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected vote status 200, got %d", resp.StatusCode)
			}
		}
	}
	t.Fatalf("vote phase did not complete")
}
