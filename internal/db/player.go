package db

import "time"

type Player struct {
	ID          uint      `gorm:"primaryKey"`
	GameID      uint      `gorm:"index;not null;uniqueIndex:idx_players_game_name"`
	Name        string    `gorm:"size:64;not null;uniqueIndex:idx_players_game_name"`
	AvatarImage []byte    `gorm:"type:bytea"`
	Color       string    `gorm:"size:16;not null;default:''"`
	IsHost      bool      `gorm:"not null;default:false"`
	JoinedAt    time.Time `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
	Prompts     []Prompt
	Drawings    []Drawing
	Guesses     []Guess
	Votes       []Vote
	Events      []Event
}
