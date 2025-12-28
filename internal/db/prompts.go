package db

import (
	"encoding/csv"
	"os"
	"strings"

	"gorm.io/gorm"
)

type promptRecord struct {
	Text string
}

// LoadPromptLibrary reads prompts from a CSV and upserts them into the prompt_library table.
func LoadPromptLibrary(conn *gorm.DB, path string) (int, error) {
	if conn == nil {
		return 0, nil
	}
	records, err := readPrompts(path)
	if err != nil {
		return 0, err
	}
	inserted := 0
	for _, record := range records {
		entry := PromptLibrary{
			Text: record.Text,
		}
		if err := conn.FirstOrCreate(&entry, PromptLibrary{Text: entry.Text}).Error; err != nil {
			return inserted, err
		}
		inserted++
	}
	return inserted, nil
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
		if len(row) == 0 {
			continue
		}
		text := ""
		if len(row) >= 2 {
			text = strings.TrimSpace(row[1])
		} else {
			text = strings.TrimSpace(row[0])
		}
		if text == "" {
			continue
		}
		records = append(records, promptRecord{Text: text})
	}
	return records, nil
}
