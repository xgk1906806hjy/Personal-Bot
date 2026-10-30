package orchestrator

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

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

	o.recordUsage(ownerKey, result.Usage)

	_, count := o.sessions.AppendTurn(ownerKey, text, result.Text)
	if count > o.sessions.CompressAfter() {
		_ = o.sessions.MaybeCompress(ownerKey, modelSummarizer{model: o.model, sessions: o.sessions, ownerKey: ownerKey})
	}
	return result.Text, nil
}

func (o *Orchestrator) recordUsage(ownerKey string, u models.TokenUsage) {
	if u.TotalTokens == 0 && u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return
	}
	o.sessions.AddUsage(ownerKey, u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.Estimated)
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
		// /clear tokens  → same as /tokens clear
		if strings.EqualFold(strings.TrimSpace(arg), "tokens") || strings.EqualFold(strings.TrimSpace(arg), "token") {
			view := o.sessions.ClearUsage(ownerKey, false)
			return fmt.Sprintf("已清空 [%d] %s 的 token 统计", view.Index, view.Title), true
		}
		item := o.sessions.ClearActive(ownerKey)
		return fmt.Sprintf("已清空 [%d] %s 的上下文", item.Index, item.Title), true
	case "/ctx", "/context":
		return formatContext(o.sessions.ActiveSnapshot(ownerKey)), true
	case "/tokens", "/usage":
		argLower := strings.ToLower(strings.TrimSpace(arg))
		switch argLower {
		case "clear", "reset":
			view := o.sessions.ClearUsage(ownerKey, false)
			return fmt.Sprintf("已清空 [%d] %s 的 token 统计", view.Index, view.Title), true
		case "clear all", "reset all":
			view := o.sessions.ClearUsage(ownerKey, true)
			return fmt.Sprintf("已清空本用户全部会话的 token 统计（当前 [%d] %s）", view.Index, view.Title), true
		default:
			return formatTokens(o.sessions.ActiveSnapshot(ownerKey)), true
		}
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
/ctx — 查看当前上下文
/tokens — 查看 token 消耗
/tokens clear — 清空当前会话 token 统计
/tokens clear all — 清空本用户全部 token 统计
/clear — 清空当前上下文
/clear tokens — 同 /tokens clear

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

func formatContext(v session.ActiveView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "当前会话 [%d] %s\n条数 %d｜约估上下文 token %d｜字符 %d\n",
		v.Index, v.Title, len(v.Messages), v.ContextEst, v.ContextChars)
	if len(v.Messages) == 0 {
		b.WriteString("（空）")
		return b.String()
	}
	const maxOut = 3500
	for i, m := range v.Messages {
		role := m.Role
		if role == "" {
			role = "user"
		}
		line := fmt.Sprintf("%d. [%s] %s\n", i+1, role, truncateRunes(m.Content, 280))
		if b.Len()+len(line) > maxOut {
			fmt.Fprintf(&b, "…其余 %d 条已省略，可用 /clear 清空", len(v.Messages)-i)
			break
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String())
}

func formatTokens(v session.ActiveView) string {
	cu, ou := v.ConvoUsage, v.OwnerUsage
	var b strings.Builder
	fmt.Fprintf(&b, "Token 消耗 — [%d] %s\n", v.Index, v.Title)
	fmt.Fprintf(&b, "当前会话：调用 %d 次｜prompt %d｜completion %d｜合计 %d%s\n",
		cu.Calls, cu.PromptTokens, cu.CompletionTokens, cu.TotalTokens, estNote(cu))
	fmt.Fprintf(&b, "本用户合计：调用 %d 次｜prompt %d｜completion %d｜合计 %d%s\n",
		ou.Calls, ou.PromptTokens, ou.CompletionTokens, ou.TotalTokens, estNote(ou))
	if cu.Calls > 0 {
		fmt.Fprintf(&b, "最近一次：prompt %d｜completion %d｜合计 %d%s\n",
			cu.LastPrompt, cu.LastCompletion, cu.LastTotal, lastEstNote(cu.LastEstimated))
	}
	fmt.Fprintf(&b, "当前上下文约估 token：%d（未发往模型的累计统计另计）\n", v.ContextEst)
	b.WriteString("清空：/tokens clear 或 /tokens clear all")
	return strings.TrimSpace(b.String())
}

func estNote(u session.UsageStats) string {
	if u.Calls == 0 {
		return ""
	}
	if u.EstimatedCalls == u.Calls {
		return "（约估）"
	}
	if u.EstimatedCalls > 0 {
		return fmt.Sprintf("（含 %d 次约估）", u.EstimatedCalls)
	}
	return ""
}

func lastEstNote(estimated bool) string {
	if estimated {
		return "（约估）"
	}
	return ""
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
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
	model    models.Channel
	sessions *session.Store
	ownerKey string
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
	if m.sessions != nil && m.ownerKey != "" {
		u := resp.Usage
		if u.TotalTokens > 0 || u.PromptTokens > 0 || u.CompletionTokens > 0 {
			m.sessions.AddUsage(m.ownerKey, u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.Estimated)
		}
	}
	return resp.Text, nil
}
