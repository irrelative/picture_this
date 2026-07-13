package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"picture-this/internal/config"
	"picture-this/internal/server"
)

func main() {
	outputPath := flag.String("output", "prompts.csv", "destination CSV path")
	count := flag.Int("count", 100, "number of prompt and joke pairs to generate (1-100)")
	instructions := flag.String("instructions", "Varied family-friendly party prompts with broad subjects and comedy styles.", "optional creative direction")
	flag.Parse()

	if *count < 1 || *count > 100 {
		log.Fatal("count must be between 1 and 100")
	}
	if strings.TrimSpace(*outputPath) == "" {
		log.Fatal("output path is required")
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	cfg := config.Load()
	if strings.TrimSpace(cfg.OpenAIAPIKey) == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	prompts, err := server.New(nil, cfg).GeneratePromptsFromOpenAI(context.Background(), *instructions, *count)
	if err != nil {
		log.Fatalf("generate prompts: %v", err)
	}
	if len(prompts) != *count {
		log.Fatalf("generated %d complete prompt pairs; expected %d; existing file was not changed", len(prompts), *count)
	}
	if err := writePromptsCSV(*outputPath, prompts); err != nil {
		log.Fatalf("write prompts: %v", err)
	}
	log.Printf("wrote %d prompt and joke pairs to %s using %s", len(prompts), *outputPath, cfg.OpenAIModel)
}

func writePromptsCSV(path string, prompts []server.GeneratedPrompt) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".prompts-*.csv")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer temp.Close()
	defer os.Remove(tempPath)

	writer := csv.NewWriter(temp)
	if err := writer.Write([]string{"category", "prompt", "joke"}); err != nil {
		return err
	}
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt.Text) == "" || strings.TrimSpace(prompt.Joke) == "" {
			return errors.New("prompt and joke must not be empty")
		}
		if err := writer.Write([]string{"generated", prompt.Text, prompt.Joke}); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
