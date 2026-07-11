package db

import "time"

type Round struct {
	ID        uint      `gorm:"primaryKey"`
	GameID    uint      `gorm:"index;not null;uniqueIndex:idx_rounds_game_number"`
	Number    int       `gorm:"not null;uniqueIndex:idx_rounds_game_number"`
	Status    string    `gorm:"size:32;not null"`
	ActiveDrawingIndex int       `gorm:"not null;default:0"`
	RevealStage        string    `gorm:"size:32;not null;default:''"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
	Prompts   []Prompt
	Drawings  []Drawing
	Guesses   []Guess
	Votes     []Vote
	Events    []Event
}
