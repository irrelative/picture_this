package server

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

const (
	maxNameLength     = 20
	maxGuessLength    = 60
	maxPromptLength   = 140
	maxChoiceLength   = 140
	maxCategoryLength = 32
	maxDrawingBytes   = 250 * 1024
	maxRoundsPerGame  = 10
	maxLobbyPlayers   = 12
)

var validatorOnce sync.Once

func registerValidators() {
	validatorOnce.Do(func() {
		engine, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			return
		}
		_ = engine.RegisterValidation("name", func(fl validator.FieldLevel) bool {
			_, err := validateName(fl.Field().String())
			return err == nil
		})
		_ = engine.RegisterValidation("prompt", func(fl validator.FieldLevel) bool {
			_, err := validatePrompt(fl.Field().String())
			return err == nil
		})
		_ = engine.RegisterValidation("guess", func(fl validator.FieldLevel) bool {
			_, err := validateGuess(fl.Field().String())
			return err == nil
		})
		_ = engine.RegisterValidation("choice", func(fl validator.FieldLevel) bool {
			_, err := validateChoice(fl.Field().String())
			return err == nil
		})
		_ = engine.RegisterValidation("category", func(fl validator.FieldLevel) bool {
			_, err := validateCategory(fl.Field().String())
			return err == nil
		})
	})
}

func (s *Server) enforceRateLimit(c *gin.Context, action string) bool {
	return true
}

func validateName(name string) (string, error) {
	return validateText("name", name, maxNameLength)
}

func validateGuess(text string) (string, error) {
	return validateText("guess", text, maxGuessLength)
}

func validatePrompt(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", errors.New("prompt is required")
	}
	if len(trimmed) > maxPromptLength {
		return "", fmt.Errorf("prompt must be %d characters or fewer", maxPromptLength)
	}
	if !isSafeText(trimmed) {
		return "", errors.New("prompt contains unsupported characters")
	}
	return trimmed, nil
}

func validateChoice(text string) (string, error) {
	return validateText("choice", text, maxChoiceLength)
}

func validateCategory(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > maxCategoryLength {
		return "", fmt.Errorf("prompt category must be %d characters or fewer", maxCategoryLength)
	}
	for _, r := range trimmed {
		if r > 127 {
			return "", errors.New("prompt category contains unsupported characters")
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' {
			continue
		}
		return "", errors.New("prompt category contains unsupported characters")
	}
	return trimmed, nil
}

func validateText(label, text string, maxLen int) (string, error) {
	trimmed := normalizeText(text)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if len(trimmed) > maxLen {
		return "", fmt.Errorf("%s must be %d characters or fewer", label, maxLen)
	}
	if !isSafeText(trimmed) {
		return "", fmt.Errorf("%s contains unsupported characters", label)
	}
	return trimmed, nil
}

func normalizeText(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	return strings.Join(fields, " ")
}

func isSafeText(text string) bool {
	for _, r := range text {
		if r > 127 {
			return false
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case ' ', '-', '_', '\'', '"', '.', ',', '!', '?', ':', ';', '&', '(', ')', '/':
			continue
		default:
			return false
		}
	}
	return true
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
