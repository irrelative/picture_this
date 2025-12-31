package server

import (
	"bytes"
	"context"
	"strconv"
	"time"

	"picture-this/internal/web"
)

func (s *Server) buildDisplayState(game *Game) web.DisplayState {
	phase := game.Phase
	stageTitle, stageStatus, stageImage, options := buildDisplayStage(game)
	roundLabel := "--"
	if total := game.PromptsPerPlayer; total > 0 {
		if round := len(game.Rounds); round > 0 {
			roundLabel = "Round " + strconv.Itoa(round) + " of " + strconv.Itoa(total)
		}
	}
	phaseEndsAt := ""
	if !game.PhaseStartedAt.IsZero() {
		if duration := time.Duration(phaseDurationSeconds(s.cfg, phase)) * time.Second; duration > 0 {
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
		GameID:         game.ID,
		JoinCode:       game.JoinCode,
		Phase:          phase,
		PhaseEndsAt:    phaseEndsAt,
		RoundLabel:     roundLabel,
		StageTitle:     stageTitle,
		StageStatus:    stageStatus,
		StageImage:     stageImage,
		Options:        options,
		Players:        players,
		Scores:         scores,
		ShowScoreboard: showScoreboard,
		ShowFinal:      showFinal,
		PlayerCount:    len(game.Players),
		CurrentRound:   len(game.Rounds),
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
		return "Drawing results", "Reviewing answers and votes.", image, nil
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
	if round.CurrentGuess >= len(round.GuessTurns) {
		return "Guessing prompts", "Guessing in progress.", "", nil
	}
	turn := round.GuessTurns[round.CurrentGuess]
	names := buildNameMap(game.Players)
	ownerID := 0
	if turn.DrawingIndex >= 0 && turn.DrawingIndex < len(round.Drawings) {
		ownerID = round.Drawings[turn.DrawingIndex].PlayerID
	}
	ownerName := names[ownerID]
	guesserName := names[turn.GuesserID]
	status := "Guessing in progress."
	if ownerName != "" {
		status = "Guessing the prompt for " + ownerName + "'s drawing."
	}
	title := "Guessing prompts"
	if guesserName != "" {
		title = "Guessing: " + guesserName
	}
	image := ""
	if turn.DrawingIndex >= 0 && turn.DrawingIndex < len(round.Drawings) {
		image = encodeImageData(round.Drawings[turn.DrawingIndex].ImageData)
	}
	return title, status, image, nil
}

func buildVoteStage(game *Game) (string, string, string, []string) {
	round := currentRound(game)
	if round == nil {
		return "Vote for the real prompt", "Voting on prompts.", "", nil
	}
	if round.CurrentVote >= len(round.VoteTurns) {
		return "Vote for the real prompt", "Voting on prompts.", "", nil
	}
	turn := round.VoteTurns[round.CurrentVote]
	names := buildNameMap(game.Players)
	ownerID := 0
	if turn.DrawingIndex >= 0 && turn.DrawingIndex < len(round.Drawings) {
		ownerID = round.Drawings[turn.DrawingIndex].PlayerID
	}
	ownerName := names[ownerID]
	status := "Voting on prompts."
	if ownerName != "" {
		status = "Vote on the real prompt for " + ownerName + "'s drawing."
	}
	image := ""
	if turn.DrawingIndex >= 0 && turn.DrawingIndex < len(round.Drawings) {
		image = encodeImageData(round.Drawings[turn.DrawingIndex].ImageData)
	}
	options := voteOptionsForDrawing(round, turn.DrawingIndex)
	return "Vote for the real prompt", status, image, options
}

func buildNameMap(players []Player) map[int]string {
	names := make(map[int]string, len(players))
	for _, player := range players {
		names[player.ID] = player.Name
	}
	return names
}
