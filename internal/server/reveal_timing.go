package server

const revealVoteStepSeconds = 3

func revealVotesStageDurationSeconds(baseSeconds int, round *RoundState) int {
	if baseSeconds < 0 {
		baseSeconds = 0
	}
	steps := revealVoteStepCount(round)
	estimated := steps * revealVoteStepSeconds
	if estimated > baseSeconds {
		return estimated
	}
	return baseSeconds
}

func revealVoteStepCount(round *RoundState) int {
	if round == nil {
		return 0
	}
	drawingIndex := normalizeDrawingIndex(round)
	options := revealOptionsPayload(round, drawingIndex, map[int]string{})
	reveal := map[string]any{
		"stage":   revealStageVotes,
		"options": options,
	}
	return len(buildRevealVoteSequence(reveal))
}
