package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

const (
	maxNameLength    = 20
	maxEmailLength   = 255
	maxGuessLength   = 60
	maxPromptLength  = 140
	maxChoiceLength  = 140
	maxJokeLength    = 140
	maxDrawingBytes  = 250 * 1024
	minPasswordLen   = 8
	maxPasswordLen   = 128
	maxRoundsPerGame = 10
	maxLobbyPlayers  = 10
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
	})
}

func (s *Server) enforceRateLimit(c *gin.Context, action string) bool {
	limit, window := ratePolicy(action)
	now := s.rateNow()
	key := action + ":" + requestClientIP(c.Request)
	s.rateMu.Lock()
	entry := s.rateEntries[key]
	if entry == nil || now.Sub(entry.started) >= window {
		entry = &rateEntry{started: now}
		s.rateEntries[key] = entry
	}
	entry.count++
	allowed := entry.count <= limit
	retry := window - now.Sub(entry.started)
	if len(s.rateEntries) > 4096 {
		for candidate, value := range s.rateEntries {
			if now.Sub(value.started) > 2*window {
				delete(s.rateEntries, candidate)
			}
		}
	}
	s.rateMu.Unlock()
	if allowed {
		return true
	}
	c.Header("Retry-After", strconv.Itoa(max(1, int(retry.Seconds()))))
	c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests; try again shortly"})
	return false
}

type rateEntry struct {
	started time.Time
	count   int
}

func ratePolicy(action string) (int, time.Duration) {
	switch action {
	case "login", "recover":
		return 10, time.Minute
	case "register", "create":
		return 20, time.Minute
	case "join", "audience-join":
		return 60, time.Minute
	default:
		return 240, time.Minute
	}
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	return host
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

func validateJoke(text string) (string, error) {
	trimmed := normalizeText(text)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > maxJokeLength {
		return "", fmt.Errorf("joke must be %d characters or fewer", maxJokeLength)
	}
	if !isSafeText(trimmed) {
		return "", errors.New("joke contains unsupported characters")
	}
	return trimmed, nil
}

func validateChoice(text string) (string, error) {
	return validateText("choice", text, maxChoiceLength)
}

func validateEmail(email string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(email))
	if trimmed == "" {
		return "", errors.New("email is required")
	}
	if len(trimmed) > maxEmailLength {
		return "", fmt.Errorf("email must be %d characters or fewer", maxEmailLength)
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed || parsed.Name != "" {
		return "", errors.New("email is invalid")
	}
	local, domain, ok := strings.Cut(trimmed, "@")
	if !ok || local == "" || domain == "" || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return "", errors.New("email is invalid")
	}
	return trimmed, nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	if len(password) > maxPasswordLen {
		return fmt.Errorf("password must be %d characters or fewer", maxPasswordLen)
	}
	return nil
}

func validateText(label, text string, maxLen int) (string, error) {
	trimmed := normalizeText(text)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if utf8.RuneCountInString(trimmed) > maxLen {
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
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) {
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
