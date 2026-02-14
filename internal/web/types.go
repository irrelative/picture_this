package web

type GameSummary struct {
	ID       string `json:"id"`
	JoinCode string `json:"join_code"`
	Phase    string `json:"phase"`
	Players  int    `json:"players"`
}

type DisplayPlayer struct {
	Name   string
	Avatar string
	IsHost bool
}

type DisplayScore struct {
	Name  string
	Score int
}

type DisplayState struct {
	GameID             string
	JoinCode           string
	Phase              string
	PhaseEndsAt        string
	RevealStage        string
	RevealJokeAudio    string
	RevealDrawingIndex int
	RoundLabel         string
	StageTitle         string
	StageStatus        string
	StageImage         string
	Options            []string
	Players            []DisplayPlayer
	Scores             []DisplayScore
	ShowScoreboard     bool
	ShowFinal          bool
	PlayerCount        int
	CurrentRound       int
}

type PlayerListItem struct {
	ID     int
	Name   string
	Avatar string
	Color  string
	IsHost bool
}
