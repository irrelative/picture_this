package game

import "sort"

func AdaptiveRounds(players int) int {
	if players >= 7 {
		return 1
	}
	return 2
}

func Scores(state State) []Score {
	points := make(map[int]int, len(state.Players))
	for _, playerID := range state.Players {
		points[playerID] = 0
	}
	for _, round := range state.Rounds {
		for drawingIndex, drawing := range round.Drawings {
			fooled, correct := 0, 0
			for _, vote := range round.Votes {
				if vote.DrawingIndex != drawingIndex {
					continue
				}
				if vote.Correct {
					points[vote.PlayerID] += 1000
					correct++
					continue
				}
				if owner := lieOwner(round.Lies, drawingIndex, vote.ChoiceText); owner != 0 {
					points[owner] += 500
					fooled++
				}
			}
			if state.Ruleset == RulesetDrawful {
				points[drawing.ArtistID] += 500 * correct
			} else if fooled == 0 {
				points[drawing.ArtistID] += 1000
			} else {
				points[drawing.ArtistID] += 500 * fooled
			}
		}
	}
	result := make([]Score, 0, len(state.Players))
	for _, playerID := range state.Players {
		result = append(result, Score{PlayerID: playerID, Points: points[playerID]})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Points == result[j].Points {
			return result[i].PlayerID < result[j].PlayerID
		}
		return result[i].Points > result[j].Points
	})
	return result
}

func lieOwner(lies []Lie, drawingIndex int, text string) int {
	for _, lie := range lies {
		if lie.DrawingIndex == drawingIndex && lie.Text == text {
			return lie.PlayerID
		}
	}
	return 0
}
