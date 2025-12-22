package web

import "picture-this/internal/db"

type AdminData struct {
	Game     db.Game
	Players  []db.Player
	Rounds   []db.Round
	Prompts  []db.Prompt
	Drawings []db.Drawing
	Guesses  []db.Guess
	Votes    []db.Vote
	Events   []db.Event
	Error    string
}
