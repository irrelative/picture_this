package db

import (
	"time"

	"gorm.io/datatypes"
)

type Game struct {
	ID        uint      `gorm:"primaryKey"`
	JoinCode  string    `gorm:"size:12;uniqueIndex;not null"`
	Phase     string    `gorm:"size:32;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
	Players   []Player
	Rounds    []Round
	Events    []Event
}

type Player struct {
	ID        uint      `gorm:"primaryKey"`
	GameID    uint      `gorm:"index;not null"`
	Name      string    `gorm:"size:64;not null;uniqueIndex:idx_players_game_name"`
	IsHost    bool      `gorm:"not null;default:false"`
	JoinedAt  time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
	Prompts   []Prompt
	Drawings  []Drawing
	Guesses   []Guess
	Votes     []Vote
	Events    []Event
}

type Round struct {
	ID        uint      `gorm:"primaryKey"`
	GameID    uint      `gorm:"index;not null;uniqueIndex:idx_rounds_game_number"`
	Number    int       `gorm:"not null;uniqueIndex:idx_rounds_game_number"`
	Status    string    `gorm:"size:32;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
	Prompts   []Prompt
	Drawings  []Drawing
	Guesses   []Guess
	Votes     []Vote
	Events    []Event
}

type Prompt struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null;uniqueIndex:idx_prompts_round_player"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_prompts_round_player"`
	Text      string    `gorm:"size:280;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type Drawing struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null;uniqueIndex:idx_drawings_round_player"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_drawings_round_player"`
	PromptID  uint      `gorm:"index;not null;uniqueIndex:idx_drawings_prompt"`
	ImageData []byte    `gorm:"type:bytea;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type Guess struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_guesses_drawing_player"`
	DrawingID uint      `gorm:"index;not null;uniqueIndex:idx_guesses_drawing_player"`
	Text      string    `gorm:"size:280;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type Vote struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null;uniqueIndex:idx_votes_round_player"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_votes_round_player"`
	GuessID   uint      `gorm:"index;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type Event struct {
	ID        uint           `gorm:"primaryKey"`
	GameID    uint           `gorm:"index;not null"`
	RoundID   *uint          `gorm:"index"`
	PlayerID  *uint          `gorm:"index"`
	Type      string         `gorm:"size:64;not null"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null"`
	CreatedAt time.Time      `gorm:"not null"`
}
