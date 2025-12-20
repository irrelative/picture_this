package main

import (
	"log"

	"picture-this/internal/config"
	"picture-this/internal/db"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	conn, err := db.Open()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
}
