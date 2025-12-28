package server

type EventPayload struct {
	GameID           string `json:"game_id,omitempty"`
	JoinCode         string `json:"join_code,omitempty"`
	PlayerName       string `json:"player,omitempty"`
	PlayerID         int    `json:"player_id,omitempty"`
	RoundNumber      int    `json:"round_number,omitempty"`
	Phase            string `json:"phase,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Prompt           string `json:"prompt,omitempty"`
	Guess            string `json:"guess,omitempty"`
	Choice           string `json:"choice,omitempty"`
	PromptsPerPlayer int    `json:"prompts_per_player,omitempty"`
	MaxPlayers       int    `json:"max_players,omitempty"`
	LobbyLocked      bool   `json:"lobby_locked,omitempty"`
	Count            int    `json:"count,omitempty"`
}
