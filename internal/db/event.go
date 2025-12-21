package db

import (
	"time"

	"gorm.io/datatypes"
)

type Event struct {
	ID        uint           `gorm:"primaryKey"`
	GameID    uint           `gorm:"index;not null"`
	RoundID   *uint          `gorm:"index"`
	PlayerID  *uint          `gorm:"index"`
	Type      string         `gorm:"size:64;not null"`
	Payload   datatypes.JSON `gorm:"type:jsonb;not null"`
	CreatedAt time.Time      `gorm:"not null"`
}
