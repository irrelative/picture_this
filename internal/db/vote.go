package db

import "time"

type Vote struct {
	ID         uint      `gorm:"primaryKey"`
	RoundID    uint      `gorm:"index;not null;uniqueIndex:idx_votes_round_player_drawing"`
	PlayerID   uint      `gorm:"index;not null;uniqueIndex:idx_votes_round_player_drawing"`
	DrawingID  uint      `gorm:"index;not null;default:0;uniqueIndex:idx_votes_round_player_drawing"`
	GuessID    uint      `gorm:"index;not null;default:0"`
	ChoiceText string    `gorm:"size:280;not null;default:''"`
	ChoiceType string    `gorm:"size:32;not null;default:''"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}
