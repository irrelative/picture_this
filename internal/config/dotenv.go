package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads environment variables from a .env file if present.
// Existing environment variables are not overwritten.
func LoadDotEnv(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return godotenv.Load(path)
}

type Config struct {
	PromptsPerPlayer      int
	DrawDurationSeconds   int
	GuessDurationSeconds  int
	VoteDurationSeconds   int
	RevealDurationSeconds int
}

func Default() Config {
	return Config{
		PromptsPerPlayer:      2,
		DrawDurationSeconds:   90,
		GuessDurationSeconds:  60,
		VoteDurationSeconds:   45,
		RevealDurationSeconds: 6,
	}
}

func Load() Config {
	cfg := Default()
	if raw := os.Getenv("PROMPTS_PER_PLAYER"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			cfg.PromptsPerPlayer = value
		}
	}
	if raw := os.Getenv("DRAW_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.DrawDurationSeconds = value
		}
	}
	if raw := os.Getenv("GUESS_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.GuessDurationSeconds = value
		}
	}
	if raw := os.Getenv("VOTE_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.VoteDurationSeconds = value
		}
	}
	if raw := os.Getenv("REVEAL_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.RevealDurationSeconds = value
		}
	}
	return cfg
}
