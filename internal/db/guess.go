package db

import "time"

type Guess struct {
	ID        uint      `gorm:"primaryKey"`
	RoundID   uint      `gorm:"index;not null;uniqueIndex:idx_guesses_round_player_text"`
	PlayerID  uint      `gorm:"index;not null;uniqueIndex:idx_guesses_round_player_text"`
	DrawingID uint      `gorm:"index;not null"`
	Text      string    `gorm:"size:280;not null;uniqueIndex:idx_guesses_round_player_text"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}
