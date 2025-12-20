package main

import (
	"log"
	"os"
	"path/filepath"

	"picture-this/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	sourceURL, err := migrationSourceURL()
	if err != nil {
		log.Fatalf("migration source error: %v", err)
	}

	m, err := migrate.New(sourceURL, mustDatabaseURL())
	if err != nil {
		log.Fatalf("migration setup failed: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("database migration failed: %v", err)
	}
	log.Println("database migrations applied")
}

func mustDatabaseURL() string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	return dsn
}

func migrationSourceURL() (string, error) {
	path := filepath.Join("db", "migrations")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return "file://" + abs, nil
}
