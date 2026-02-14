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
	wsRolePlayer   = "player"
	wsRoleHost     = "host"
	wsRoleDisplay  = "display"
	wsRoleAudience = "audience"
)

const (
	voteChoicePrompt   = "prompt"
	voteChoiceGuess    = "guess"
	voteOptionIDPrompt = "prompt"
	voteOptionIDGuess  = "guess:"
)

const (
	revealStageGuesses = "guesses"
	revealStageVotes   = "votes"
	revealStageJoke    = "joke"
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
	PlayerAuthTokens map[int]string
	Audience         []AudienceMember
	Players          []Player
	Rounds           []RoundState
	PromptsPerPlayer int
}

type Player struct {
	ID           int
	Name         string
	Avatar       []byte
	AvatarLocked bool
	IsHost       bool
	DBID         uint
	Color        string
	Claimed      bool
}

type RoundState struct {
	Number        int
	DBID          uint
	Prompts       []PromptEntry
	Drawings      []DrawingEntry
	Guesses       []GuessEntry
	Votes         []VoteEntry
	AudienceVotes []AudienceVoteEntry
	RevealIndex   int
	RevealStage   string
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

type VoteEntry struct {
	PlayerID     int
	DrawingIndex int
	ChoiceText   string
	ChoiceType   string
	DBID         uint
}

type AudienceVoteEntry struct {
	AudienceID   int
	AudienceName string
	ChoiceID     string
	ChoiceText   string
	ChoiceType   string
	DrawingIndex int
}

type AudienceMember struct {
	ID    int
	Name  string
	Token string
}

type VoteOption struct {
	ID      string
	Text    string
	Type    string
	OwnerID int
}
