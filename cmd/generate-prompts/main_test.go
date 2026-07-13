package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"picture-this/internal/server"
)

func TestWritePromptsCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompts.csv")
	prompts := []server.GeneratedPrompt{
		{Text: "Dentist hiding from a tooth", Joke: "Professional confidence has left the building."},
	}

	if err := writePromptsCSV(path, prompts); err != nil {
		t.Fatalf("write prompts: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open prompts: %v", err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read prompts: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header and one prompt, got %d rows", len(rows))
	}
	if rows[0][2] != "joke" || rows[1][2] != prompts[0].Joke {
		t.Fatalf("unexpected joke column: %#v", rows)
	}
}
