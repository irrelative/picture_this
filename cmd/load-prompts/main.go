package main

import (
	"flag"
	"log"

	"picture-this/internal/config"
	"picture-this/internal/db"
)

func main() {
	filePath := flag.String("file", "prompts.csv", "path to prompts csv")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	conn, err := db.Open()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	inserted, err := db.LoadPromptLibrary(conn, *filePath)
	if err != nil {
		log.Fatalf("failed to load prompts: %v", err)
	}

	log.Printf("loaded %d prompts", inserted)
}
