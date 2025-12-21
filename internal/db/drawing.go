package db

import "time"

type Drawing struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null;uniqueIndex:idx_drawings_round_player;uniqueIndex:idx_drawings_round_prompt"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_drawings_round_player"`
	PromptID  uint      `gorm:"index;not null;uniqueIndex:idx_drawings_round_prompt"`
	ImageData []byte    `gorm:"type:bytea;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}
