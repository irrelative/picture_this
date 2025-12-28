package web

import (
	"time"

	"picture-this/internal/db"
)

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

type AdminDBGameSummary struct {
	ID        uint
	JoinCode  string
	Phase     string
	Players   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AdminPromptLibraryData struct {
	Prompts       []db.PromptLibrary
	Error         string
	Notice        string
	DraftText     string
}
