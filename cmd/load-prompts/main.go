package main

import (
	"encoding/csv"
	"flag"
	"log"
	"os"
	"strings"

	"picture-this/internal/config"
	"picture-this/internal/db"
)

type promptRecord struct {
	Category string
	Text     string
}

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

	records, err := readPrompts(*filePath)
	if err != nil {
		log.Fatalf("failed to read prompts: %v", err)
	}

	inserted := 0
	for _, record := range records {
		entry := db.PromptLibrary{
			Category: record.Category,
			Text:     record.Text,
		}
		if err := conn.FirstOrCreate(&entry, db.PromptLibrary{Category: entry.Category, Text: entry.Text}).Error; err != nil {
			log.Fatalf("failed to upsert prompt: %v", err)
		}
		inserted++
	}

	log.Printf("loaded %d prompts", inserted)
}

func readPrompts(path string) ([]promptRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var records []promptRecord
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) < 2 {
			continue
		}
		category := strings.TrimSpace(row[0])
		text := strings.TrimSpace(row[1])
		if category == "" || text == "" {
			continue
		}
		records = append(records, promptRecord{Category: category, Text: text})
	}
	return records, nil
}
