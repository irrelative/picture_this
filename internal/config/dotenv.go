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
	PromptsPerPlayer int
}

func Default() Config {
	return Config{
		PromptsPerPlayer: 2,
	}
}

func Load() Config {
	cfg := Default()
	if raw := os.Getenv("PROMPTS_PER_PLAYER"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			cfg.PromptsPerPlayer = value
		}
	}
	return cfg
}
