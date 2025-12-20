package db

import (
	"errors"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open connects to Postgres using DATABASE_URL.
func Open() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

// Migrate runs GORM auto-migrations for the core tables.
func Migrate(conn *gorm.DB) error {
	if conn == nil {
		return errors.New("db connection is nil")
	}
	if err := conn.AutoMigrate(
		&Game{},
		&Player{},
		&Round{},
		&Prompt{},
		&Drawing{},
		&Guess{},
		&Vote{},
		&Event{},
	); err != nil {
		return err
	}
	log.Println("database migration complete")
	return nil
}
