package db

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Email        string    `gorm:"size:255;unique;not null"`
	Username     string    `gorm:"size:64;not null"`
	PasswordHash string    `gorm:"size:255;not null"`
	IsAdmin      bool      `gorm:"not null;default:false"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}
