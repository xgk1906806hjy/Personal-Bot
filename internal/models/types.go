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

type Response struct {
	Channel        config.ModelChannel
	Text           string
	ConversationID string
}

// Channel is a pluggable model backend.
type Channel interface {
	ID() config.ModelChannel
	Chat(req Request) (Response, error)
}
