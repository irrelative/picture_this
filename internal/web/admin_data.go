package web

import (
	"time"

	"picture-this/internal/db"
)

type AdminData struct {
	Game           db.Game
	Players        []db.Player
	Rounds         []db.Round
	Prompts        []db.Prompt
	Drawings       []db.Drawing
	Guesses        []db.Guess
	Votes          []db.Vote
	Events         []db.Event
	InMemory       bool
	Paused         bool
	PausedPhase    string
	ClaimedPlayers int
	TotalPlayers   int
	Error          string
}

type AdminDBGameSummary struct {
	ID        uint
	JoinCode  string
	Phase     string
	Players   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PaginationData struct {
	BasePath   string
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
}

type AdminPromptLibraryData struct {
	Prompts              []db.PromptLibrary
	Error                string
	Notice               string
	SearchQuery          string
	DraftText            string
	DraftJoke            string
	GenerateInstructions string
	Pagination           PaginationData
}

type AdminHomeData struct {
	Active     []GameSummary
	History    []AdminDBGameSummary
	Pagination PaginationData
}
