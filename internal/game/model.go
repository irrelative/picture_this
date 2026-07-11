package game

type Phase string

const (
	PhaseLobby    Phase = "lobby"
	PhaseDrawings Phase = "drawings"
	PhaseLies     Phase = "guesses"
	PhaseVoting   Phase = "guesses-votes"
	PhaseResults  Phase = "results"
	PhasePaused   Phase = "paused"
	PhaseComplete Phase = "complete"
)

type Ruleset string

const (
	RulesetLegacy  Ruleset = "picture_this_v1"
	RulesetDrawful Ruleset = "drawful_v1"
)

type Drawing struct {
	ArtistID int
}

type Lie struct {
	PlayerID     int
	DrawingIndex int
	Text         string
}

type Vote struct {
	PlayerID     int
	DrawingIndex int
	ChoiceText   string
	Correct      bool
}

type Round struct {
	Drawings []Drawing
	Lies     []Lie
	Votes    []Vote
}

type State struct {
	Ruleset Ruleset
	Players []int
	Rounds  []Round
}

type Score struct {
	PlayerID int
	Points   int
}
