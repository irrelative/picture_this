package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
	rateLimitExceeded = "rate limit exceeded"
)

type rateLimitRule struct {
	Capacity int
	Window   time.Duration
}

var rateLimitRules = map[string]rateLimitRule{
	"create":   {Capacity: 5, Window: time.Minute},
	"join":     {Capacity: 10, Window: time.Minute},
	"avatar":   {Capacity: 10, Window: time.Minute},
	"settings": {Capacity: 10, Window: time.Minute},
	"kick":     {Capacity: 10, Window: time.Minute},
	"rename":   {Capacity: 10, Window: time.Minute},
	"start":    {Capacity: 5, Window: time.Minute},
	"advance":  {Capacity: 10, Window: time.Minute},
	"end":      {Capacity: 5, Window: time.Minute},
	"drawings": {Capacity: 30, Window: time.Minute},
	"guesses":  {Capacity: 30, Window: time.Minute},
	"votes":    {Capacity: 30, Window: time.Minute},
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	now     func() time.Time
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		now:     time.Now,
	}
}

func (r *rateLimiter) allow(key string, capacity int, window time.Duration) bool {
	if capacity <= 0 || window <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	bucket, ok := r.buckets[key]
	if !ok {
		r.buckets[key] = &tokenBucket{
			tokens: float64(capacity - 1),
			last:   now,
		}
		return true
	}
	elapsed := now.Sub(bucket.last).Seconds()
	refill := (float64(capacity) / window.Seconds()) * elapsed
	bucket.tokens = minFloat(float64(capacity), bucket.tokens+refill)
	bucket.last = now
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func (s *Server) enforceRateLimit(c *gin.Context, action string) bool {
	if s.limiter == nil {
		return true
	}
	rule, ok := rateLimitRules[action]
	if !ok {
		return true
	}
	key := action + ":" + clientKey(c)
	if s.limiter.allow(key, rule.Capacity, rule.Window) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": rateLimitExceeded})
	return false
}

func clientKey(c *gin.Context) string {
	if value, err := c.Cookie("pt_session"); err == nil && value != "" {
		return "session:" + value
	}
	return c.ClientIP()
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
