package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

const MaxConversations = 10

type Message struct {
	Role    string // user | assistant | system
	Content string
}

type Conversation struct {
	ID        string
	Title     string
	UpdatedAt time.Time
	Messages  []Message
}

type ownerBag struct {
	ActiveID      string
	Conversations []*Conversation
	UpdatedAt     time.Time
}

// ListItem is a numbered view of one conversation for /list.
type ListItem struct {
	Index  int
	ID     string
	Title  string
	Active bool
	Msgs   int
}

type Store struct {
	ttl           time.Duration
	compressAfter int
	keepRecent    int
	mu            sync.Mutex
	m             map[string]*ownerBag
	seq           uint64
}

func NewStore(ttlHours, compressAfter, keepRecent int) *Store {
	if compressAfter <= 0 {
		compressAfter = 30
	}
	if keepRecent <= 0 {
		keepRecent = 16
	}
	if keepRecent >= compressAfter {
		keepRecent = compressAfter / 2
		if keepRecent < 1 {
			keepRecent = 1
		}
	}
	return &Store{
		ttl:           time.Duration(ttlHours) * time.Hour,
		compressAfter: compressAfter,
		keepRecent:    keepRecent,
		m:             make(map[string]*ownerBag),
	}
}

func (s *Store) CompressAfter() int { return s.compressAfter }
func (s *Store) KeepRecent() int    { return s.keepRecent }

func BuildKey(platform config.MessagingPlatform, chatID, userID string) string {
	return fmt.Sprintf("%s:%s:%s", platform, chatID, userID)
}

func (s *Store) getOrCreateLocked(ownerKey string) *ownerBag {
	bag, ok := s.m[ownerKey]
	if ok && time.Since(bag.UpdatedAt) > s.ttl {
		delete(s.m, ownerKey)
		ok = false
	}
	if !ok {
		s.seq++
		id := fmt.Sprintf("c%d", s.seq)
		bag = &ownerBag{
			ActiveID: id,
			Conversations: []*Conversation{{
				ID:        id,
				Title:     "默认会话",
				UpdatedAt: time.Now(),
			}},
			UpdatedAt: time.Now(),
		}
		s.m[ownerKey] = bag
	}
	bag.UpdatedAt = time.Now()
	return bag
}

func (s *Store) activeLocked(bag *ownerBag) *Conversation {
	for _, c := range bag.Conversations {
		if c.ID == bag.ActiveID {
			return c
		}
	}
	if len(bag.Conversations) == 0 {
		return nil
	}
	bag.ActiveID = bag.Conversations[0].ID
	return bag.Conversations[0]
}

// EnsureActive returns the active conversation id, creating a default bag if needed.
func (s *Store) EnsureActive(ownerKey string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	return c.ID
}

// LoadActiveMessages returns a copy of messages for the active conversation.
func (s *Store) LoadActiveMessages(ownerKey string) (conversationID string, msgs []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	out := make([]Message, len(c.Messages))
	copy(out, c.Messages)
	return c.ID, out
}

// AppendTurn appends user+assistant turns and returns the new message count.
func (s *Store) AppendTurn(ownerKey, userText, assistantText string) (conversationID string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.Messages = append(c.Messages,
		Message{Role: "user", Content: userText},
		Message{Role: "assistant", Content: assistantText},
	)
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
	return c.ID, len(c.Messages)
}

// ReplaceMessages sets the active conversation history (used after compression).
func (s *Store) ReplaceMessages(ownerKey string, msgs []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.Messages = append([]Message{}, msgs...)
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
}

func (s *Store) List(ownerKey string) []ListItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	out := make([]ListItem, 0, len(bag.Conversations))
	for i, c := range bag.Conversations {
		out = append(out, ListItem{
			Index:  i + 1,
			ID:     c.ID,
			Title:  c.Title,
			Active: c.ID == bag.ActiveID,
			Msgs:   len(c.Messages),
		})
	}
	return out
}

func (s *Store) Create(ownerKey, title string) (ListItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	if len(bag.Conversations) >= MaxConversations {
		return ListItem{}, fmt.Errorf("已达上限 %d 个会话，请先 /clear 或继续用现有会话", MaxConversations)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = fmt.Sprintf("会话 %d", len(bag.Conversations)+1)
	}
	s.seq++
	id := fmt.Sprintf("c%d", s.seq)
	c := &Conversation{
		ID:        id,
		Title:     title,
		UpdatedAt: time.Now(),
	}
	bag.Conversations = append(bag.Conversations, c)
	bag.ActiveID = id
	bag.UpdatedAt = time.Now()
	return ListItem{Index: len(bag.Conversations), ID: id, Title: title, Active: true, Msgs: 0}, nil
}

func (s *Store) Switch(ownerKey string, index int) (ListItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	if index < 1 || index > len(bag.Conversations) {
		return ListItem{}, fmt.Errorf("无效序号 %d，请先 /list", index)
	}
	c := bag.Conversations[index-1]
	bag.ActiveID = c.ID
	bag.UpdatedAt = time.Now()
	c.UpdatedAt = time.Now()
	return ListItem{Index: index, ID: c.ID, Title: c.Title, Active: true, Msgs: len(c.Messages)}, nil
}

func (s *Store) Rename(ownerKey, title string) (ListItem, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return ListItem{}, fmt.Errorf("标题不能为空，用法：/rename 新标题")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.Title = title
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
	idx := 1
	for i, x := range bag.Conversations {
		if x.ID == c.ID {
			idx = i + 1
			break
		}
	}
	return ListItem{Index: idx, ID: c.ID, Title: c.Title, Active: true, Msgs: len(c.Messages)}, nil
}

func (s *Store) ClearActive(ownerKey string) ListItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.Messages = nil
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
	idx := 1
	for i, x := range bag.Conversations {
		if x.ID == c.ID {
			idx = i + 1
			break
		}
	}
	return ListItem{Index: idx, ID: c.ID, Title: c.Title, Active: true, Msgs: 0}
}
