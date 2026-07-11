package db

import "time"

type Game struct {
	ID               uint      `gorm:"primaryKey"`
	JoinCode         string    `gorm:"size:12;uniqueIndex;not null"`
	Phase            string    `gorm:"size:32;not null"`
	PromptsPerPlayer int       `gorm:"not null;default:2"`
	MinPlayers       int       `gorm:"not null;default:2"`
	MaxPlayers       int       `gorm:"not null;default:0"`
	LobbyLocked      bool      `gorm:"not null;default:false"`
	Ruleset          string    `gorm:"size:32;not null;default:'picture_this_v1'"`
	AvatarsEnabled   bool      `gorm:"not null;default:false"`
	AudienceEnabled  bool      `gorm:"not null;default:false"`
	JokesEnabled     bool      `gorm:"not null;default:false"`
	PublicReplay     bool      `gorm:"not null;default:false"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
	Players          []Player
	Rounds           []Round
	Events           []Event
}
