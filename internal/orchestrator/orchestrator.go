package orchestrator

import (
	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/models"
	"github.com/xgk1906806hjy/Personal-Bot/internal/session"
)

// IncomingUserMessage is platform-agnostic inbound chat.
type IncomingUserMessage struct {
	Platform  config.MessagingPlatform
	ChatID    string
	UserID    string
	MessageID string
	Text      string
}

type Orchestrator struct {
	cfg      *config.Config
	model    models.Channel
	sessions *session.Store
}

func New(cfg *config.Config, model models.Channel, sessions *session.Store) *Orchestrator {
	return &Orchestrator{cfg: cfg, model: model, sessions: sessions}
}

func (o *Orchestrator) Handle(msg IncomingUserMessage) (string, error) {
	sessionKey := session.BuildKey(msg.Platform, msg.ChatID, msg.UserID)
	existing, _ := o.sessions.Get(sessionKey)

	result, err := o.model.Chat(models.Request{
		SessionKey:     sessionKey,
		ConversationID: existing.ConversationID,
		Prompt:         msg.Text,
		Messages: []models.ChatMessage{
			{Role: models.RoleUser, Content: msg.Text},
		},
	})
	if err != nil {
		return "", err
	}
	o.sessions.Touch(sessionKey, result.ConversationID)
	return result.Text, nil
}

func (o *Orchestrator) ModelChannelID() config.ModelChannel {
	return o.model.ID()
}
