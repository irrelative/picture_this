package config

import (
	"os"

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
