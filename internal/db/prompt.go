package db

import "time"

type Prompt struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null;uniqueIndex:idx_prompts_round_player_text"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_prompts_round_player_text"`
	Text      string    `gorm:"size:280;not null;uniqueIndex:idx_prompts_round_player_text"`
	Joke      string    `gorm:"size:280"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}
