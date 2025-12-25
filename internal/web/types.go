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
	GameID         string
	JoinCode       string
	Phase          string
	PhaseEndsAt    string
	RoundLabel     string
	StageTitle     string
	StageStatus    string
	StageImage     string
	Options        []string
	Players        []DisplayPlayer
	Scores         []DisplayScore
	ShowScoreboard bool
	ShowFinal      bool
}
