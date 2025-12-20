package web

type GameSummary struct {
	ID       string `json:"id"`
	JoinCode string `json:"join_code"`
	Phase    string `json:"phase"`
	Players  int    `json:"players"`
}
