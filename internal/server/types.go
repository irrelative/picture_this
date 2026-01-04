package server

import "time"

const (
	phaseLobby      = "lobby"
	phaseDrawings   = "drawings"
	phaseGuesses    = "guesses"
	phaseGuessVotes = "guesses-votes"
	phaseResults    = "results"
	phasePaused     = "paused"
	phaseComplete   = "complete"
)

const (
	revealStageGuesses = "guesses"
	revealStageVotes   = "votes"
)

type GameSummary struct {
	ID       string
	JoinCode string
	Phase    string
	Players  int
}

type Game struct {
	ID               string
	DBID             uint
	JoinCode         string
	Phase            string
	PhaseStartedAt   time.Time
	MaxPlayers       int
	LobbyLocked      bool
	PausedPhase      string
	UsedPrompts      map[string]struct{}
	KickedPlayers    map[string]struct{}
	HostID           int
	Players          []Player
	Rounds           []RoundState
	PromptsPerPlayer int
}

type Player struct {
	ID      int
	Name    string
	Avatar  []byte
	IsHost  bool
	DBID    uint
	Color   string
	Claimed bool
}

type RoundState struct {
	Number       int
	DBID         uint
	Prompts      []PromptEntry
	Drawings     []DrawingEntry
	Guesses      []GuessEntry
	Votes        []VoteEntry
	GuessTurns   []GuessTurn
	CurrentGuess int
	VoteTurns    []VoteTurn
	CurrentVote  int
	RevealIndex  int
	RevealStage  string
}

type PromptEntry struct {
	PlayerID      int
	Text          string
	Joke          string
	JokeAudioPath string
	DBID          uint
}

type DrawingEntry struct {
	PlayerID  int
	ImageData []byte
	Prompt    string
	DBID      uint
}

type GuessEntry struct {
	PlayerID     int
	DrawingIndex int
	Text         string
	DBID         uint
}

type GuessTurn struct {
	DrawingIndex int
	GuesserID    int
}

type VoteEntry struct {
	PlayerID     int
	DrawingIndex int
	ChoiceText   string
	ChoiceType   string
	DBID         uint
}

type VoteTurn struct {
	DrawingIndex int
	VoterID      int
}
