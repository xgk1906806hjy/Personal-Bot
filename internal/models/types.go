package models

import "github.com/xgk1906806hjy/Personal-Bot/internal/config"

type ChatRole string

const (
	RoleSystem    ChatRole = "system"
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
)

type ChatMessage struct {
	Role    ChatRole
	Content string
}

type Request struct {
	SessionKey     string
	ConversationID string
	Messages       []ChatMessage
	Prompt         string
	// DisableTools skips tool binding (used for session compression summaries).
	DisableTools bool
}

// TokenUsage is API-reported or estimated token counts for one Chat call.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Estimated        bool // true when not from API usage field
}

func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
		Estimated:        u.Estimated || other.Estimated,
	}
}

type Response struct {
	Channel        config.ModelChannel
	Text           string
	ConversationID string
	Usage          TokenUsage
}

// Channel is a pluggable model backend.
type Channel interface {
	ID() config.ModelChannel
	Chat(req Request) (Response, error)
}
