package server

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"picture-this/internal/web"
)

func (s *Server) buildDisplayState(game *Game) web.DisplayState {
	phase := game.Phase
	stageTitle, stageStatus, stageImage, options := buildDisplayStage(game)
	revealStage := ""
	revealJokeAudio := ""
	revealDrawingIndex := -1
	if reveal := buildReveal(game); reveal != nil {
		if value, ok := reveal["stage"].(string); ok {
			revealStage = value
		}
		if value, ok := reveal["joke_audio"].(string); ok {
			revealJokeAudio = value
		}
		if value, ok := reveal["drawing_index"].(int); ok {
			revealDrawingIndex = value
		}
	}
	roundLabel := "--"
	if total := game.PromptsPerPlayer; total > 0 {
		if round := len(game.Rounds); round > 0 {
			roundLabel = "Round " + strconv.Itoa(round) + " of " + strconv.Itoa(total)
		}
	}
	phaseEndsAt := ""
	if !game.PhaseStartedAt.IsZero() {
		if duration := time.Duration(phaseDurationSeconds(s.cfg, game)) * time.Second; duration > 0 {
			phaseEndsAt = game.PhaseStartedAt.Add(duration).UTC().Format(time.RFC3339)
		}
	}
	players := make([]web.DisplayPlayer, 0, len(game.Players))
	for _, player := range game.Players {
		players = append(players, web.DisplayPlayer{
			Name:   player.Name,
			Avatar: encodeImageData(player.Avatar),
			IsHost: player.ID == game.HostID,
		})
	}
	scores := make([]web.DisplayScore, 0)
	for _, entry := range buildScores(game) {
		name, _ := entry["player_name"].(string)
		score, _ := entry["score"].(int)
		scores = append(scores, web.DisplayScore{
			Name:  name,
			Score: score,
		})
	}
	showScoreboard := false
	if phase == phaseDrawings {
		if round := currentRound(game); round != nil {
			if len(game.Rounds) > 1 && len(round.Drawings) == 0 {
				showScoreboard = true
			}
		}
	}
	showFinal := phase == phaseComplete
	return web.DisplayState{
		GameID:             game.ID,
		JoinCode:           game.JoinCode,
		Phase:              phase,
		PhaseEndsAt:        phaseEndsAt,
		RevealStage:        revealStage,
		RevealJokeAudio:    revealJokeAudio,
		RevealDrawingIndex: revealDrawingIndex,
		RoundLabel:         roundLabel,
		StageTitle:         stageTitle,
		StageStatus:        stageStatus,
		StageImage:         stageImage,
		Options:            options,
		Players:            players,
		Scores:             scores,
		ShowScoreboard:     showScoreboard,
		ShowFinal:          showFinal,
		PlayerCount:        len(game.Players),
		CurrentRound:       len(game.Rounds),
	}
}

func (s *Server) renderDisplayHTML(game *Game) string {
	var buf bytes.Buffer
	state := s.buildDisplayState(game)
	if err := web.DisplayContent(state).Render(context.Background(), &buf); err != nil {
		return ""
	}
	return buf.String()
}

func buildDisplayStage(game *Game) (string, string, string, []string) {
	phase := game.Phase
	if phase == phasePaused {
		status := "Game is paused. Players should rejoin and claim their name."
		if game.PausedPhase != "" {
			status = "Game paused during " + game.PausedPhase + ". Players should rejoin and claim their name."
		}
		return "Game paused", status, "", nil
	}
	if phase == phaseLobby {
		return "Waiting for players", "Share the join code so everyone can join.", "", nil
	}
	if phase == phaseDrawings {
		return "Drawing round", "Players are drawing their prompts.", "", nil
	}
	if phase == phaseGuesses {
		return buildGuessStage(game)
	}
	if phase == phaseGuessVotes {
		return buildVoteStage(game)
	}
	if phase == phaseResults {
		reveal := buildReveal(game)
		image, _ := reveal["drawing_image"].(string)
		status := "Reviewing answers and votes."
		stage, _ := reveal["stage"].(string)
		options := revealOptionsForDisplay(reveal)
		switch stage {
		case revealStageGuesses:
			status = "Revealing guesses."
		case revealStageVotes:
			status = "Revealing votes."
		case revealStageJoke:
			status = "Narrator is reading the joke."
		}
		return "Drawing results", status, image, options
	}
	if phase == phaseComplete {
		return "Game complete", "Thanks for playing!", "", nil
	}
	return "Waiting for updates", "Loading game status.", "", nil
}

func buildGuessStage(game *Game) (string, string, string, []string) {
	round := currentRound(game)
	if round == nil {
		return "Guessing prompts", "Guessing in progress.", "", nil
	}
	assignments := buildGuessAssignments(game, round)
	playerID, drawingIndex, ok := firstAssignmentByOrder(game, assignments)
	if !ok {
		required := requiredGuessCount(game, round)
		status := "Waiting for votes."
		if required > 0 {
			status = "All guesses submitted (" + strconv.Itoa(len(round.Guesses)) + "/" + strconv.Itoa(required) + ")."
		}
		return "Guessing prompts", status, "", nil
	}
	names := buildNameMap(game.Players)
	ownerID := round.Drawings[drawingIndex].PlayerID
	ownerName := names[ownerID]
	pending := pendingGuessersForDrawing(game, assignments, drawingIndex)
	required := requiredGuessCount(game, round)
	submitted := len(round.Guesses)
	status := "Collecting guesses."
	if ownerName != "" {
		status = "Collecting guesses for " + ownerName + "'s drawing."
	}
	if required > 0 {
		status += " (" + strconv.Itoa(submitted) + "/" + strconv.Itoa(required) + " submitted)"
	}
	if pendingCount := len(pending); pendingCount > 0 {
		status += " " + strconv.Itoa(pendingCount) + " left on this drawing."
	}
	if playerName := names[playerID]; playerName != "" {
		status += " Next up: " + playerName + "."
	}
	image := encodeImageData(round.Drawings[drawingIndex].ImageData)
	return "Guessing prompts", status, image, nil
}

func buildVoteStage(game *Game) (string, string, string, []string) {
	round := currentRound(game)
	if round == nil {
		return "Vote for the real prompt", "Voting on prompts.", "", nil
	}
	assignments := buildVoteAssignments(game, round)
	playerID, drawingIndex, ok := firstAssignmentByOrder(game, assignments)
	if !ok {
		required := requiredVoteCount(game, round)
		status := "Waiting for reveal."
		if required > 0 {
			status = "All votes submitted (" + strconv.Itoa(len(round.Votes)) + "/" + strconv.Itoa(required) + ")."
		}
		return "Vote for the real prompt", status, "", nil
	}
	names := buildNameMap(game.Players)
	ownerID := round.Drawings[drawingIndex].PlayerID
	ownerName := names[ownerID]
	status := "Voting on prompts."
	if ownerName != "" {
		status = "Vote on the real prompt for " + ownerName + "'s drawing."
	}
	required := requiredVoteCount(game, round)
	submitted := len(round.Votes)
	pending := pendingVotersForDrawing(game, assignments, drawingIndex)
	if required > 0 {
		status += " (" + strconv.Itoa(submitted) + "/" + strconv.Itoa(required) + " submitted)"
	}
	if pendingCount := len(pending); pendingCount > 0 {
		status += " " + strconv.Itoa(pendingCount) + " left on this drawing."
	}
	if playerName := names[playerID]; playerName != "" {
		status += " Next up: " + playerName + "."
	}
	image := encodeImageData(round.Drawings[drawingIndex].ImageData)
	options := voteOptionsForDrawing(round, drawingIndex)
	return "Vote for the real prompt", status, image, options
}

func buildNameMap(players []Player) map[int]string {
	names := make(map[int]string, len(players))
	for _, player := range players {
		names[player.ID] = player.Name
	}
	return names
}

func revealOptionsForDisplay(reveal map[string]any) []string {
	if reveal == nil {
		return nil
	}
	stage, _ := reveal["stage"].(string)
	lines := make([]string, 0)
	if stage == revealStageGuesses {
		if guesses, ok := reveal["guesses"].([]map[string]any); ok {
			for _, guess := range guesses {
				lines = append(lines, fmt.Sprintf("%s: %s", displayName(guess["player_name"]), displayText(guess["text"])))
			}
		} else if guessesRaw, ok := reveal["guesses"].([]any); ok {
			for _, raw := range guessesRaw {
				if guess, ok := raw.(map[string]any); ok {
					lines = append(lines, fmt.Sprintf("%s: %s", displayName(guess["player_name"]), displayText(guess["text"])))
				}
			}
		}
	}
	if stage == revealStageVotes || stage == revealStageJoke {
		if optionsRaw, ok := reveal["options"].([]any); ok && len(optionsRaw) > 0 {
			for _, raw := range optionsRaw {
				option, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				optionType, _ := option["type"].(string)
				optionText := displayText(option["text"])
				ownerName := displayName(option["owner_name"])
				if optionType == voteChoicePrompt {
					lines = append(lines, "Prompt: "+optionText)
				} else {
					lines = append(lines, fmt.Sprintf("%s wrote: %s", ownerName, optionText))
				}
				playerVotes := make([]string, 0)
				if rawVotes, ok := option["player_votes"].([]any); ok {
					for _, rawVote := range rawVotes {
						vote, ok := rawVote.(map[string]any)
						if !ok {
							continue
						}
						playerVotes = append(playerVotes, displayName(vote["player_name"]))
					}
				}
				if len(playerVotes) > 0 {
					lines = append(lines, "Picked by: "+strings.Join(playerVotes, ", "))
				}
				audienceCount := displayInt(option["audience_count"])
				if audienceCount > 0 {
					lines = append(lines, fmt.Sprintf("Audience picks: %d", audienceCount))
				}
			}
		} else {
			if prompt, _ := reveal["prompt"].(string); prompt != "" {
				lines = append(lines, "Prompt: "+prompt)
			}
			if votes, ok := reveal["votes"].([]map[string]any); ok {
				for _, vote := range votes {
					lines = append(lines, fmt.Sprintf("%s: %s", displayName(vote["player_name"]), displayText(vote["text"])))
				}
			} else if votesRaw, ok := reveal["votes"].([]any); ok {
				for _, raw := range votesRaw {
					vote, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					lines = append(lines, fmt.Sprintf("%s: %s", displayName(vote["player_name"]), displayText(vote["text"])))
				}
			}
			if audienceVotes, ok := reveal["audience_votes"].([]map[string]any); ok {
				for _, vote := range audienceVotes {
					lines = append(lines, fmt.Sprintf("Audience: %s (%d)", displayText(vote["text"]), displayInt(vote["count"])))
				}
			} else if audienceVotes, ok := reveal["audience_votes"].([]any); ok {
				for _, raw := range audienceVotes {
					vote, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					lines = append(lines, fmt.Sprintf("Audience: %s (%d)", displayText(vote["text"]), displayInt(vote["count"])))
				}
			}
		}
		if stage == revealStageJoke {
			if joke, _ := reveal["joke"].(string); joke != "" {
				lines = append(lines, "Joke: "+joke)
			}
		}
	}
	if deltasRaw, ok := reveal["score_deltas"].([]any); ok && len(deltasRaw) > 0 {
		lines = append(lines, "Score changes:")
		for _, raw := range deltasRaw {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: +%d", displayName(entry["player_name"]), displayInt(entry["delta"])))
		}
	}
	return lines
}

func displayName(value any) string {
	name, _ := value.(string)
	if name == "" {
		return "Player"
	}
	return name
}

func displayText(value any) string {
	text, _ := value.(string)
	return text
}

func displayInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
