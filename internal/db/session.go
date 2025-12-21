package db

import "time"

type Session struct {
	ID         string    `gorm:"primaryKey;size:64"`
	Flash      string    `gorm:"size:280"`
	PlayerName string    `gorm:"size:64"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}
