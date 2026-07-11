package server

func cloneGame(source *Game) *Game {
	if source == nil {
		return nil
	}
	game := *source
	game.Players = append([]Player(nil), source.Players...)
	for i := range game.Players {
		game.Players[i].Avatar = append([]byte(nil), source.Players[i].Avatar...)
	}
	game.Audience = append([]AudienceMember(nil), source.Audience...)
	game.Rounds = append([]RoundState(nil), source.Rounds...)
	for i := range game.Rounds {
		sourceRound := source.Rounds[i]
		game.Rounds[i].Prompts = append([]PromptEntry(nil), sourceRound.Prompts...)
		game.Rounds[i].Drawings = append([]DrawingEntry(nil), sourceRound.Drawings...)
		for j := range game.Rounds[i].Drawings {
			game.Rounds[i].Drawings[j].ImageData = append([]byte(nil), sourceRound.Drawings[j].ImageData...)
		}
		game.Rounds[i].Guesses = append([]GuessEntry(nil), sourceRound.Guesses...)
		game.Rounds[i].Votes = append([]VoteEntry(nil), sourceRound.Votes...)
		game.Rounds[i].AudienceVotes = append([]AudienceVoteEntry(nil), sourceRound.AudienceVotes...)
		game.Rounds[i].Likes = append([]LikeEntry(nil), sourceRound.Likes...)
	}
	game.UsedPrompts = cloneStringSet(source.UsedPrompts)
	game.KickedPlayers = cloneStringSet(source.KickedPlayers)
	game.PlayerAuthTokens = make(map[int]string, len(source.PlayerAuthTokens))
	for id, token := range source.PlayerAuthTokens {
		game.PlayerAuthTokens[id] = token
	}
	return &game
}

func cloneStringSet(source map[string]struct{}) map[string]struct{} {
	if source == nil {
		return nil
	}
	result := make(map[string]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}
