// Package models 定义可插拔的模型通道（gateway_api / codex_local / cursor_local）。
package models

import "github.com/xgk1906806hjy/Personal-Bot/internal/config"

// ChatRole 对应 chat.completions 的 role。
type ChatRole string

const (
	RoleSystem    ChatRole = "system"
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
)

// ChatMessage 是发给模型的一条历史消息。
type ChatMessage struct {
	Role    ChatRole
	Content string
}

// Request 是一次 Chat 调用的入参。
type Request struct {
	SessionKey     string
	ConversationID string
	Messages       []ChatMessage
	Prompt         string
	// DisableTools 跳过工具绑定（会话压缩摘要使用；单次请求标志，非会话级开关）。
	DisableTools bool
	// ExternalAgentID 为 Cursor 等可 resume 的 agent id。
	ExternalAgentID string
	// Workspace 覆盖本次调用的工作目录（Cursor cwd）。
	Workspace string
}

// TokenUsage 是单次 Chat 的 token 统计（API 回报或估算）。
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Estimated        bool // true 表示非 API usage 字段估算
}

func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
		Estimated:        u.Estimated || other.Estimated,
	}
}

// Response 是模型通道的统一出参。
type Response struct {
	Channel         config.ModelChannel
	Text            string
	ConversationID  string
	Usage           TokenUsage
	ExternalAgentID string
}

// Channel 是可插拔模型后端。
type Channel interface {
	ID() config.ModelChannel
	Chat(req Request) (Response, error)
}
