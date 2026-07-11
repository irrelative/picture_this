package db

import "time"

type Like struct {
	ID           uint      `gorm:"primaryKey"`
	RoundID      uint      `gorm:"index;not null;uniqueIndex:idx_likes_round_player_drawing_owner"`
	PlayerID     uint      `gorm:"index;not null;uniqueIndex:idx_likes_round_player_drawing_owner"`
	DrawingID    uint      `gorm:"index;not null;uniqueIndex:idx_likes_round_player_drawing_owner"`
	GuessOwnerID uint      `gorm:"index;not null;uniqueIndex:idx_likes_round_player_drawing_owner"`
	CreatedAt    time.Time `gorm:"not null"`
}
