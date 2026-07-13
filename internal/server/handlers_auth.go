package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required"`
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) handleRegister(c *gin.Context) {
	if !s.enforceRateLimit(c, "register") {
		return
	}
	if s.sessions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication is unavailable"})
		return
	}
	var req registerRequest
	if !bindJSON(c, &req, bindMessages{
		"Email": {
			"required": "email is required",
		},
		"Password": {
			"required": "password is required",
		},
	}, "invalid registration request") {
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := normalizeText(req.Username)
	if username == "" {
		username = defaultUsernameFromEmail(email)
	}
	username, err = validateName(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, exists := s.sessions.FindUserByEmail(email); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "email is already registered"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to secure password"})
		return
	}

	user, err := s.sessions.CreateUser(email, username, string(hashed))
	if err != nil {
		if isUniqueViolation(err) || strings.Contains(strings.ToLower(err.Error()), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "email is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	s.sessions.SetUserID(c.Writer, c.Request, user.ID)
	c.JSON(http.StatusCreated, gin.H{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
		"is_admin": user.IsAdmin,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	if !s.enforceRateLimit(c, "login") {
		return
	}
	if s.sessions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication is unavailable"})
		return
	}
	var req loginRequest
	if !bindJSON(c, &req, bindMessages{
		"Email": {
			"required": "email is required",
		},
		"Password": {
			"required": "password is required",
		},
	}, "invalid login request") {
		return
	}
	email, err := validateEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, ok := s.sessions.FindUserByEmail(email)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	s.sessions.SetUserID(c.Writer, c.Request, user.ID)
	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
		"is_admin": user.IsAdmin,
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	if s.sessions != nil {
		s.sessions.ClearUser(c.Writer, c.Request)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func defaultUsernameFromEmail(email string) string {
	local, _, found := strings.Cut(email, "@")
	if !found || local == "" {
		return "Player"
	}
	builder := strings.Builder{}
	count := 0
	for _, r := range local {
		if count >= maxNameLength {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			builder.WriteRune(r)
			count++
		}
	}
	name := strings.TrimSpace(builder.String())
	if name == "" {
		return "Player"
	}
	if _, err := validateName(name); err != nil {
		return "Player"
	}
	return name
}
