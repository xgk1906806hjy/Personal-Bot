package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

type Record struct {
	SessionKey     string
	ConversationID string
	UpdatedAt      time.Time
}

type Store struct {
	ttl time.Duration
	mu  sync.Mutex
	m   map[string]Record
}

func NewStore(ttlHours int) *Store {
	return &Store{
		ttl: time.Duration(ttlHours) * time.Hour,
		m:   make(map[string]Record),
	}
}

func (s *Store) Get(sessionKey string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.m[sessionKey]
	if !ok {
		return Record{}, false
	}
	if time.Since(rec.UpdatedAt) > s.ttl {
		delete(s.m, sessionKey)
		return Record{}, false
	}
	return rec, true
}

func (s *Store) Touch(sessionKey, conversationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.m[sessionKey]
	if conversationID == "" && ok {
		conversationID = existing.ConversationID
	}
	s.m[sessionKey] = Record{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		UpdatedAt:      time.Now(),
	}
}

func BuildKey(platform config.MessagingPlatform, chatID, userID string) string {
	return fmt.Sprintf("%s:%s:%s", platform, chatID, userID)
}
