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

// UsageStats accumulates API/estimated token counts.
type UsageStats struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Calls            int
	EstimatedCalls   int // how many calls used estimates instead of API usage
	LastPrompt       int
	LastCompletion   int
	LastTotal        int
	LastEstimated    bool
}

func (u *UsageStats) Add(prompt, completion, total int, estimated bool) {
	if total == 0 {
		total = prompt + completion
	}
	u.PromptTokens += prompt
	u.CompletionTokens += completion
	u.TotalTokens += total
	u.Calls++
	if estimated {
		u.EstimatedCalls++
	}
	u.LastPrompt = prompt
	u.LastCompletion = completion
	u.LastTotal = total
	u.LastEstimated = estimated
}

func (u *UsageStats) Clear() {
	*u = UsageStats{}
}

type Conversation struct {
	ID            string
	Title         string
	UpdatedAt     time.Time
	Messages      []Message
	Usage         UsageStats
	CursorAgentID string // Cursor SDK local agent id for resume
	Workspace     string // optional override of TOOL_WORKSPACE for Cursor
	CursorMode    bool   // /cursor entered: normal messages route to Cursor
}

type ownerBag struct {
	ActiveID      string
	Conversations []*Conversation
	UpdatedAt     time.Time
	Usage         UsageStats // owner-wide across all conversations
}

// ListItem is a numbered view of one conversation for /list.
type ListItem struct {
	Index  int
	ID     string
	Title  string
	Active bool
	Msgs   int
}

// ActiveView is a snapshot for /ctx and /tokens.
type ActiveView struct {
	Index        int
	ID           string
	Title        string
	Messages     []Message
	ConvoUsage   UsageStats
	OwnerUsage   UsageStats
	ContextChars int
	ContextEst   int // estimated tokens for current context text
	Workspace    string
	CursorMode   bool
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

// LoadActiveCursorAgentID returns the Cursor agent id bound to the active conversation.
func (s *Store) LoadActiveCursorAgentID(ownerKey string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	return c.CursorAgentID
}

// SetCursorAgentID stores a Cursor agent id on the active conversation.
func (s *Store) SetCursorAgentID(ownerKey, agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.CursorAgentID = strings.TrimSpace(agentID)
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
}

// IsCursorMode reports whether the active conversation is in Cursor mode.
func (s *Store) IsCursorMode(ownerKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	return c.CursorMode
}

// SetCursorMode enables or disables Cursor mode for the active conversation.
func (s *Store) SetCursorMode(ownerKey string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.CursorMode = on
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
}

// ActiveWorkspace returns the session workspace override, or fallback if unset.
func (s *Store) ActiveWorkspace(ownerKey, fallback string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	if strings.TrimSpace(c.Workspace) != "" {
		return c.Workspace
	}
	return fallback
}

// GetWorkdir implements tools.WorkdirStore (empty means use TOOL_WORKSPACE root).
func (s *Store) GetWorkdir(sessionKey string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(sessionKey)
	c := s.activeLocked(bag)
	return strings.TrimSpace(c.Workspace)
}

// SetWorkdir implements tools.WorkdirStore.
func (s *Store) SetWorkdir(sessionKey, absPath string) error {
	s.SetWorkspace(sessionKey, absPath)
	return nil
}

// SetWorkspace sets the Cursor/tool working directory for the active conversation.
// Changing workspace clears CursorAgentID so the next run creates a new agent.
func (s *Store) SetWorkspace(ownerKey, absPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	prev := c.Workspace
	c.Workspace = strings.TrimSpace(absPath)
	if c.Workspace != prev {
		c.CursorAgentID = ""
	}
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
}

// ClearWorkspace removes the session override (back to TOOL_WORKSPACE default).
func (s *Store) ClearWorkspace(ownerKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	if c.Workspace != "" {
		c.Workspace = ""
		c.CursorAgentID = ""
	}
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
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
	c.CursorAgentID = ""
	// Keep CursorMode + Workspace so /clear only drops transcript, not the coding mode.
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

// AddUsage records token usage on the active conversation and owner totals.
func (s *Store) AddUsage(ownerKey string, prompt, completion, total int, estimated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	c := s.activeLocked(bag)
	c.Usage.Add(prompt, completion, total, estimated)
	bag.Usage.Add(prompt, completion, total, estimated)
	c.UpdatedAt = time.Now()
	bag.UpdatedAt = time.Now()
}

// ClearUsage resets token counters for the active conversation and subtracts from owner totals.
// If all=true, clears every conversation and the owner total.
func (s *Store) ClearUsage(ownerKey string, all bool) ActiveView {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	if all {
		for _, c := range bag.Conversations {
			c.Usage.Clear()
		}
		bag.Usage.Clear()
	} else {
		c := s.activeLocked(bag)
		// Rebuild owner totals from remaining conversations after clearing active.
		c.Usage.Clear()
		bag.Usage.Clear()
		for _, x := range bag.Conversations {
			if x.Usage.Calls == 0 {
				continue
			}
			bag.Usage.PromptTokens += x.Usage.PromptTokens
			bag.Usage.CompletionTokens += x.Usage.CompletionTokens
			bag.Usage.TotalTokens += x.Usage.TotalTokens
			bag.Usage.Calls += x.Usage.Calls
			bag.Usage.EstimatedCalls += x.Usage.EstimatedCalls
		}
	}
	return s.activeViewLocked(bag)
}

// ActiveSnapshot returns context + usage for the active conversation.
func (s *Store) ActiveSnapshot(ownerKey string) ActiveView {
	s.mu.Lock()
	defer s.mu.Unlock()
	bag := s.getOrCreateLocked(ownerKey)
	return s.activeViewLocked(bag)
}

func (s *Store) activeViewLocked(bag *ownerBag) ActiveView {
	c := s.activeLocked(bag)
	idx := 1
	for i, x := range bag.Conversations {
		if x.ID == c.ID {
			idx = i + 1
			break
		}
	}
	msgs := make([]Message, len(c.Messages))
	copy(msgs, c.Messages)
	chars, est := contextSize(msgs)
	return ActiveView{
		Index:        idx,
		ID:           c.ID,
		Title:        c.Title,
		Messages:     msgs,
		ConvoUsage:   c.Usage,
		OwnerUsage:   bag.Usage,
		ContextChars: chars,
		ContextEst:   est,
		Workspace:    c.Workspace,
		CursorMode:   c.CursorMode,
	}
}

func contextSize(msgs []Message) (chars, estTokens int) {
	for _, m := range msgs {
		chars += len([]rune(m.Role)) + len([]rune(m.Content)) + 2
	}
	estTokens = chars // rune-based rough estimate
	return chars, estTokens
}
