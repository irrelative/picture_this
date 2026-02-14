package server

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"picture-this/internal/db"

	"gorm.io/gorm"
)

type sessionStore struct {
	db           *gorm.DB
	mu           sync.Mutex
	sessions     map[string]sessionData
	nextUserID   uint
	usersByID    map[uint]db.User
	usersByEmail map[string]uint
}

type sessionData struct {
	Flash  string
	Name   string
	UserID uint
}

func newSessionStore(conn *gorm.DB) *sessionStore {
	return &sessionStore{
		db:           conn,
		sessions:     make(map[string]sessionData),
		nextUserID:   1,
		usersByID:    make(map[uint]db.User),
		usersByEmail: make(map[string]uint),
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
	record := s.loadSessionRecord(id)
	record.Flash = message
	s.saveSessionRecord(&record)
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
	record := s.loadSessionRecord(id)
	if record.Flash == "" {
		return ""
	}
	message := record.Flash
	record.Flash = ""
	s.saveSessionRecord(&record)
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
	record := s.loadSessionRecord(id)
	record.PlayerName = name
	s.saveSessionRecord(&record)
}

func (s *sessionStore) GetName(w http.ResponseWriter, r *http.Request) string {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.sessions[id].Name
	}
	record, found := s.findSessionRecord(id)
	if !found {
		return ""
	}
	return record.PlayerName
}

func (s *sessionStore) SetUserID(w http.ResponseWriter, r *http.Request, userID uint) {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		data := s.sessions[id]
		data.UserID = userID
		s.sessions[id] = data
		s.mu.Unlock()
		return
	}
	record := s.loadSessionRecord(id)
	record.UserID = &userID
	s.saveSessionRecord(&record)
}

func (s *sessionStore) ClearUser(w http.ResponseWriter, r *http.Request) {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		data := s.sessions[id]
		data.UserID = 0
		s.sessions[id] = data
		s.mu.Unlock()
		return
	}
	record := s.loadSessionRecord(id)
	record.UserID = nil
	s.saveSessionRecord(&record)
}

func (s *sessionStore) CurrentUser(w http.ResponseWriter, r *http.Request) (db.User, bool) {
	id := s.ensureSessionID(w, r)
	if s.db == nil {
		s.mu.Lock()
		userID := s.sessions[id].UserID
		user, ok := s.usersByID[userID]
		s.mu.Unlock()
		return user, ok && userID != 0
	}
	record, found := s.findSessionRecord(id)
	if !found || record.UserID == nil || *record.UserID == 0 {
		return db.User{}, false
	}
	var user db.User
	if err := s.db.Where("id = ?", *record.UserID).First(&user).Error; err != nil {
		return db.User{}, false
	}
	return user, true
}

func (s *sessionStore) CreateUser(email, username, passwordHash string) (db.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return db.User{}, errors.New("email is required")
	}
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, exists := s.usersByEmail[email]; exists {
			return db.User{}, errors.New("user already exists")
		}
		user := db.User{
			ID:           s.nextUserID,
			Email:        email,
			Username:     username,
			PasswordHash: passwordHash,
			IsAdmin:      false,
		}
		s.nextUserID++
		s.usersByID[user.ID] = user
		s.usersByEmail[email] = user.ID
		return user, nil
	}
	record := db.User{
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
		IsAdmin:      false,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return db.User{}, err
	}
	return record, nil
}

func (s *sessionStore) FindUserByEmail(email string) (db.User, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return db.User{}, false
	}
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		userID, ok := s.usersByEmail[email]
		if !ok {
			return db.User{}, false
		}
		user, ok := s.usersByID[userID]
		return user, ok
	}
	var user db.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return db.User{}, false
	}
	return user, true
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

func (s *sessionStore) findSessionRecord(id string) (db.Session, bool) {
	if s.db == nil {
		return db.Session{}, false
	}
	var record db.Session
	if err := s.db.Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Session{}, false
		}
		return db.Session{}, false
	}
	return record, true
}

func (s *sessionStore) loadSessionRecord(id string) db.Session {
	record, found := s.findSessionRecord(id)
	if found {
		return record
	}
	return db.Session{ID: id}
}

func (s *sessionStore) saveSessionRecord(record *db.Session) {
	if s.db == nil || record == nil {
		return
	}
	_ = s.db.Save(record).Error
}

func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buf)
}
