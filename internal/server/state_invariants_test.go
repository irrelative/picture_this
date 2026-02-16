package server

import (
	"net/http"
	"strconv"
	"testing"
)

func TestGuessSnapshotAssignmentInvariants(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID, _ := setupThreePlayerRound(t, ts)
	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != phaseGuesses {
		t.Fatalf("expected phase %q, got %v", phaseGuesses, snapshot["phase"])
	}
	assertAssignmentSnapshotInvariants(t, snapshot, "guess_assignments", "guess_active_drawing", "guess_required_count", "guess_submitted_count", "guess_remaining", "guess_focus")

	assignments := mustAssignmentList(t, snapshot, "guess_assignments")
	if len(assignments) == 0 {
		t.Fatalf("expected guess assignments")
	}
	first := assignments[0]
	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/guesses", map[string]any{
		"player_id": asInt(first["player_id"]),
		"guess":     "invariant-guess",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] == phaseGuesses {
		assertAssignmentSnapshotInvariants(t, snapshot, "guess_assignments", "guess_active_drawing", "guess_required_count", "guess_submitted_count", "guess_remaining", "guess_focus")
	}
}

func TestVoteSnapshotAssignmentInvariants(t *testing.T) {
	_, ts := newServerHarness(t)

	gameID, _ := setupThreePlayerRound(t, ts)
	submitAllGuesses(t, ts, gameID)

	snapshot := fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] != phaseGuessVotes {
		t.Fatalf("expected phase %q, got %v", phaseGuessVotes, snapshot["phase"])
	}
	assertAssignmentSnapshotInvariants(t, snapshot, "vote_assignments", "vote_active_drawing", "vote_required_count", "vote_submitted_count", "vote_remaining", "vote_focus")

	assignments := mustAssignmentList(t, snapshot, "vote_assignments")
	if len(assignments) == 0 {
		t.Fatalf("expected vote assignments")
	}
	first := assignments[0]
	playerID := asInt(first["player_id"])
	choiceID, choiceText := firstValidVoteChoice(t, first, playerID)

	resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/votes", map[string]any{
		"player_id": playerID,
		"choice_id": choiceID,
		"choice":    choiceText,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	snapshot = fetchSnapshot(t, ts, gameID)
	if snapshot["phase"] == phaseGuessVotes {
		assertAssignmentSnapshotInvariants(t, snapshot, "vote_assignments", "vote_active_drawing", "vote_required_count", "vote_submitted_count", "vote_remaining", "vote_focus")
	}
}

func assertAssignmentSnapshotInvariants(
	t *testing.T,
	snapshot map[string]any,
	assignmentsKey string,
	activeKey string,
	requiredKey string,
	submittedKey string,
	remainingKey string,
	focusKey string,
) {
	t.Helper()

	assignments := mustAssignmentList(t, snapshot, assignmentsKey)
	remaining := mustMap(t, snapshot[remainingKey], remainingKey)
	active := asInt(snapshot[activeKey])
	required := asInt(snapshot[requiredKey])
	submitted := asInt(snapshot[submittedKey])

	if len(assignments) == 0 {
		if active != -1 {
			t.Fatalf("expected %s=-1 without assignments, got %d", activeKey, active)
		}
		if required != 0 || submitted != 0 {
			t.Fatalf("expected %s/%s=0/0 without assignments, got %d/%d", requiredKey, submittedKey, required, submitted)
		}
		for rawPlayerID, rawRemaining := range remaining {
			if asInt(rawRemaining) != 0 {
				t.Fatalf("expected %s[%s]=0 without assignments, got %v", remainingKey, rawPlayerID, rawRemaining)
			}
		}
		return
	}

	drawingIndex := asInt(assignments[0]["drawing_index"])
	pendingByPlayer := make(map[int]struct{}, len(assignments))
	for _, entry := range assignments {
		playerID := asInt(entry["player_id"])
		entryDrawing := asInt(entry["drawing_index"])
		if entryDrawing != drawingIndex {
			t.Fatalf("expected all %s entries on drawing %d, got %d", assignmentsKey, drawingIndex, entryDrawing)
		}
		if playerID <= 0 {
			t.Fatalf("expected positive player_id, got %d", playerID)
		}
		pendingByPlayer[playerID] = struct{}{}
	}

	if active != drawingIndex {
		t.Fatalf("expected %s=%d, got %d", activeKey, drawingIndex, active)
	}
	if required != submitted+len(assignments) {
		t.Fatalf("expected %s == %s + pending; got %d != %d + %d", requiredKey, submittedKey, required, submitted, len(assignments))
	}
	if submitted < 0 || submitted > required {
		t.Fatalf("expected %s in [0,%s], got %d/%d", submittedKey, requiredKey, submitted, required)
	}

	pendingCount := 0
	for rawPlayerID, rawRemaining := range remaining {
		playerID, err := strconv.Atoi(rawPlayerID)
		if err != nil {
			t.Fatalf("expected numeric player id key in %s, got %q", remainingKey, rawPlayerID)
		}
		remainingValue := asInt(rawRemaining)
		if remainingValue != 0 && remainingValue != 1 {
			t.Fatalf("expected %s[%d] to be 0 or 1, got %d", remainingKey, playerID, remainingValue)
		}
		if remainingValue == 1 {
			pendingCount++
			if _, ok := pendingByPlayer[playerID]; !ok {
				t.Fatalf("%s[%d]=1 but player is not in %s", remainingKey, playerID, assignmentsKey)
			}
		}
	}
	if pendingCount != len(assignments) {
		t.Fatalf("expected pending count %d from %s, got %d", len(assignments), remainingKey, pendingCount)
	}

	for playerID := range pendingByPlayer {
		if value := asInt(remaining[strconv.Itoa(playerID)]); value != 1 {
			t.Fatalf("expected %s[%d]=1 for pending player, got %d", remainingKey, playerID, value)
		}
	}

	if rawFocus, ok := snapshot[focusKey]; ok && rawFocus != nil {
		focus := mustMap(t, rawFocus, focusKey)
		if asInt(focus["drawing_index"]) != drawingIndex {
			t.Fatalf("expected %s.drawing_index=%d", focusKey, drawingIndex)
		}
		if asInt(focus["required_count"]) != required {
			t.Fatalf("expected %s.required_count=%d", focusKey, required)
		}
		if asInt(focus["submitted_count"]) != submitted {
			t.Fatalf("expected %s.submitted_count=%d", focusKey, submitted)
		}
	}
}

func mustAssignmentList(t *testing.T, snapshot map[string]any, key string) []map[string]any {
	t.Helper()
	rawList, ok := snapshot[key].([]any)
	if !ok {
		t.Fatalf("expected %s list, got %T", key, snapshot[key])
	}
	result := make([]map[string]any, 0, len(rawList))
	for _, raw := range rawList {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected %s item to be object, got %T", key, raw)
		}
		result = append(result, entry)
	}
	return result
}

func mustMap(t *testing.T, value any, key string) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected %s object, got %T", key, value)
	}
	return result
}

func asInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	case nil:
		return 0
	default:
		return 0
	}
}

func firstValidVoteChoice(t *testing.T, assignment map[string]any, playerID int) (string, string) {
	t.Helper()
	rawOptions, ok := assignment["options"].([]any)
	if !ok || len(rawOptions) == 0 {
		t.Fatalf("expected vote options")
	}
	for _, raw := range rawOptions {
		option, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ownerID := asInt(option["owner_id"])
		optionType, _ := option["type"].(string)
		if optionType == voteChoiceGuess && ownerID == playerID {
			continue
		}
		choiceID, _ := option["id"].(string)
		choiceText, _ := option["text"].(string)
		if choiceID == "" || choiceText == "" {
			continue
		}
		return choiceID, choiceText
	}
	t.Fatalf("no valid vote option for player %d in assignment %+v", playerID, assignment)
	return "", ""
}
