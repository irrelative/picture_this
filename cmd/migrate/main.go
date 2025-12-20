package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"picture-this/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	sourceURL, hasFiles, err := migrationSourceURL()
	if err != nil {
		log.Fatalf("migration source error: %v", err)
	}
	if !hasFiles {
		log.Println("no migrations found")
		return
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

func migrationSourceURL() (string, bool, error) {
	path := filepath.Join("db", "migrations")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", false, err
	}
	if !hasMigrationFiles(path) {
		return "", false, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false, err
	}
	return "file://" + abs, true, nil
}

func hasMigrationFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			return true
		}
	}
	return false
}
