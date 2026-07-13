package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPromptsIncludesJokes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompts.csv")
	content := "category,prompt,joke\ngenerated,Dentist hiding from a tooth,Professional confidence has left.\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	records, err := readPrompts(path)
	if err != nil {
		t.Fatalf("read prompts: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one prompt, got %d", len(records))
	}
	if records[0].Joke != "Professional confidence has left." {
		t.Fatalf("unexpected joke: %q", records[0].Joke)
	}
}
