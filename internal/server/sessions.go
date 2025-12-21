package server

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"picture-this/internal/db"

	"gorm.io/gorm"
)

type sessionStore struct {
	db       *gorm.DB
	mu       sync.Mutex
	sessions map[string]sessionData
}

type sessionData struct {
	Flash string
	Name  string
}

func newSessionStore(conn *gorm.DB) *sessionStore {
	return &sessionStore{
		db:       conn,
		sessions: make(map[string]sessionData),
	}
}

func (s *sessionStore) SetFlash(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		return
	}
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		data := s.sessions[id]
		data.Flash = message
		s.sessions[id] = data
		s.mu.Unlock()
		return
	}
	record := db.Session{
		ID:    id,
		Flash: message,
	}
	_ = s.db.Save(&record).Error
}

func (s *sessionStore) PopFlash(w http.ResponseWriter, r *http.Request) string {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		data := s.sessions[id]
		message := data.Flash
		data.Flash = ""
		s.sessions[id] = data
		return message
	}
	var record db.Session
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return ""
	}
	if record.Flash == "" {
		return ""
	}
	message := record.Flash
	record.Flash = ""
	_ = s.db.Save(&record).Error
	return message
}

func (s *sessionStore) SetName(w http.ResponseWriter, r *http.Request, name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		data := s.sessions[id]
		data.Name = name
		s.sessions[id] = data
		s.mu.Unlock()
		return
	}
	record := db.Session{
		ID:         id,
		PlayerName: name,
	}
	_ = s.db.Save(&record).Error
}

func (s *sessionStore) GetName(w http.ResponseWriter, r *http.Request) string {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.sessions[id].Name
	}
	var record db.Session
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		return ""
	}
	return record.PlayerName
}

func (s *sessionStore) ensureSessionID(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("pt_session")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	id := newSessionID()
	http.SetCookie(w, &http.Cookie{
		Name:     "pt_session",
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return id
}

func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buf)
}
