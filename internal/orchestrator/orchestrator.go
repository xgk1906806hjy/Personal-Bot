package orchestrator

import (
	"fmt"
	"strconv"
	"strings"

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
	ownerKey := session.BuildKey(msg.Platform, msg.ChatID, msg.UserID)
	text := strings.TrimSpace(msg.Text)

	if reply, handled := o.handleCommand(ownerKey, text); handled {
		return reply, nil
	}

	conversationID, hist := o.sessions.LoadActiveMessages(ownerKey)
	chatMsgs := toModelMessages(hist)

	result, err := o.model.Chat(models.Request{
		SessionKey:     ownerKey,
		ConversationID: conversationID,
		Prompt:         text,
		Messages:       chatMsgs,
	})
	if err != nil {
		return "", err
	}

	_, count := o.sessions.AppendTurn(ownerKey, text, result.Text)
	if count > o.sessions.CompressAfter() {
		_ = o.sessions.MaybeCompress(ownerKey, modelSummarizer{model: o.model})
	}
	return result.Text, nil
}

func (o *Orchestrator) ModelChannelID() config.ModelChannel {
	return o.model.ID()
}

func (o *Orchestrator) handleCommand(ownerKey, text string) (string, bool) {
	if !strings.HasPrefix(text, "/") {
		return "", false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", false
	}
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))

	switch cmd {
	case "/help":
		return helpText(), true
	case "/list":
		return formatList(o.sessions.List(ownerKey)), true
	case "/new":
		item, err := o.sessions.Create(ownerKey, arg)
		if err != nil {
			return err.Error(), true
		}
		return fmt.Sprintf("已新建并切换到 [%d] %s", item.Index, item.Title), true
	case "/use":
		n, err := strconv.Atoi(strings.TrimSpace(arg))
		if err != nil || n < 1 {
			return "用法：/use 序号（先 /list）", true
		}
		item, err := o.sessions.Switch(ownerKey, n)
		if err != nil {
			return err.Error(), true
		}
		return fmt.Sprintf("已切换到 [%d] %s（%d 条消息）", item.Index, item.Title, item.Msgs), true
	case "/rename":
		item, err := o.sessions.Rename(ownerKey, arg)
		if err != nil {
			return err.Error(), true
		}
		return fmt.Sprintf("已重命名 [%d] → %s", item.Index, item.Title), true
	case "/clear":
		item := o.sessions.ClearActive(ownerKey)
		return fmt.Sprintf("已清空 [%d] %s 的上下文", item.Index, item.Title), true
	default:
		return "未知指令。发送 /help 查看可用命令。", true
	}
}

func helpText() string {
	return strings.TrimSpace(`会话指令：
/help — 帮助
/list — 列出会话（* 为当前）
/new [标题] — 新建并切换
/use N — 切换到第 N 个
/rename 标题 — 重命名当前
/clear — 清空当前上下文

消息超过阈值会自动压缩摘要。`)
}

func formatList(items []session.ListItem) string {
	if len(items) == 0 {
		return "暂无会话"
	}
	var b strings.Builder
	b.WriteString("会话列表：\n")
	for _, it := range items {
		mark := " "
		if it.Active {
			mark = "*"
		}
		fmt.Fprintf(&b, "%s [%d] %s (%d 条)\n", mark, it.Index, it.Title, it.Msgs)
	}
	return strings.TrimSpace(b.String())
}

func toModelMessages(hist []session.Message) []models.ChatMessage {
	out := make([]models.ChatMessage, 0, len(hist))
	for _, m := range hist {
		out = append(out, models.ChatMessage{
			Role:    models.ChatRole(m.Role),
			Content: m.Content,
		})
	}
	return out
}

type modelSummarizer struct {
	model models.Channel
}

func (m modelSummarizer) Summarize(old []session.Message) (string, error) {
	prompt := session.SummaryPrompt(session.FormatMessagesForSummary(old))
	resp, err := m.model.Chat(models.Request{
		Prompt:       prompt,
		DisableTools: true,
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}
