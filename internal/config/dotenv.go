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
	PromptsPerPlayer         int
	DrawDurationSeconds      int
	GuessDurationSeconds     int
	VoteDurationSeconds      int
	RevealDurationSeconds    int
	RevealGuessesSeconds     int
	RevealVotesSeconds       int
	RevealJokeSeconds        int
	DBMaxOpenConns           int
	DBMaxIdleConns           int
	DBConnMaxLifetimeSeconds int
	DBConnMaxIdleTimeSeconds int
	OpenAIAPIKey             string
	OpenAIModel              string
	OpenAIEmbeddingModel     string
	PromptSimilarityMax      float64
	OpenAIPromptSystemPath   string
	OpenAIPromptUserPath     string
}

func Default() Config {
	return Config{
		PromptsPerPlayer:         2,
		DrawDurationSeconds:      90,
		GuessDurationSeconds:     60,
		VoteDurationSeconds:      45,
		RevealDurationSeconds:    6,
		RevealGuessesSeconds:     6,
		RevealVotesSeconds:       6,
		RevealJokeSeconds:        6,
		DBMaxOpenConns:           10,
		DBMaxIdleConns:           10,
		DBConnMaxLifetimeSeconds: 300,
		DBConnMaxIdleTimeSeconds: 60,
		OpenAIModel:              "gpt-5.2",
		OpenAIEmbeddingModel:     "text-embedding-3-small",
		PromptSimilarityMax:      0.12,
		OpenAIPromptSystemPath:   "prompts/openai_drawing_system.txt",
		OpenAIPromptUserPath:     "prompts/openai_drawing_user.txt",
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
	if raw := os.Getenv("REVEAL_GUESSES_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.RevealGuessesSeconds = value
		}
	}
	if raw := os.Getenv("REVEAL_VOTES_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.RevealVotesSeconds = value
		}
	}
	if raw := os.Getenv("REVEAL_JOKE_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			cfg.RevealJokeSeconds = value
		}
	}
	if raw := os.Getenv("DB_MAX_OPEN_CONNS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			cfg.DBMaxOpenConns = value
		}
	}
	if raw := os.Getenv("DB_MAX_IDLE_CONNS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			cfg.DBMaxIdleConns = value
		}
	}
	if raw := os.Getenv("DB_CONN_MAX_LIFETIME_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			cfg.DBConnMaxLifetimeSeconds = value
		}
	}
	if raw := os.Getenv("DB_CONN_MAX_IDLE_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			cfg.DBConnMaxIdleTimeSeconds = value
		}
	}
	if raw := os.Getenv("OPENAI_API_KEY"); raw != "" {
		cfg.OpenAIAPIKey = raw
	}
	if raw := os.Getenv("OPENAI_MODEL"); raw != "" {
		cfg.OpenAIModel = raw
	}
	if raw := os.Getenv("OPENAI_EMBEDDING_MODEL"); raw != "" {
		cfg.OpenAIEmbeddingModel = raw
	}
	if raw := os.Getenv("PROMPT_SIMILARITY_MAX"); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err == nil && value > 0 {
			cfg.PromptSimilarityMax = value
		}
	}
	if raw := os.Getenv("OPENAI_PROMPT_SYSTEM_PATH"); raw != "" {
		cfg.OpenAIPromptSystemPath = raw
	}
	if raw := os.Getenv("OPENAI_PROMPT_USER_PATH"); raw != "" {
		cfg.OpenAIPromptUserPath = raw
	}
	return cfg
}
