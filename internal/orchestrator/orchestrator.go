// Package orchestrator 是对话中枢：斜杠指令、Cursor 模式、主通道 Chat、会话压缩触发。
// 消息适配器把平台差异收敛为 IncomingUserMessage 后只调用 Handle。
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

// IncomingUserMessage 是与平台无关的入站消息。
type IncomingUserMessage struct {
	Platform  config.MessagingPlatform
	ChatID    string
	UserID    string
	MessageID string
	Text      string
}

// Orchestrator 持有主模型、可选 Cursor 旁路与会话存储。
type Orchestrator struct {
	cfg      *config.Config
	model    models.Channel
	cursor   models.Channel // 可选；仅 /cursor 模式使用
	sessions *session.Store
}

// New 装配编排器。cursor 可为 nil（未配置 CURSOR_API_KEY 时）。
func New(cfg *config.Config, model models.Channel, sessions *session.Store, cursor models.Channel) *Orchestrator {
	return &Orchestrator{cfg: cfg, model: model, cursor: cursor, sessions: sessions}
}

// Handle 处理一条用户消息：
//  1. 斜杠指令 → 直接回状态文案
//  2. Cursor 模式（或主通道即 cursor_local）→ chatWithCursor
//  3. 否则主通道 Chat；条数超阈值则触发摘要压缩
func (o *Orchestrator) Handle(msg IncomingUserMessage) (string, error) {
	ownerKey := session.BuildKey(msg.Platform, msg.ChatID, msg.UserID)
	text := strings.TrimSpace(msg.Text)

	if reply, handled, err := o.handleCommand(ownerKey, text); handled {
		return reply, err
	}

	useCursor := o.sessions.IsCursorMode(ownerKey) || o.model.ID() == config.ModelCursorLocal
	if useCursor {
		return o.chatWithCursor(ownerKey, text)
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

// cursorChannel 优先用旁路 cursor；否则当主通道本身是 cursor_local 时复用 model。
func (o *Orchestrator) cursorChannel() models.Channel {
	if o.cursor != nil {
		return o.cursor
	}
	if o.model != nil && o.model.ID() == config.ModelCursorLocal {
		return o.model
	}
	return nil
}

// chatWithCursor 走 Cursor SDK（不经过 gateway 工具循环），并写回 agentId 便于 resume。
func (o *Orchestrator) chatWithCursor(ownerKey, text string) (string, error) {
	cursor := o.cursorChannel()
	if cursor == nil {
		o.sessions.SetCursorMode(ownerKey, false)
		return "未启用 Cursor：请配置 CURSOR_API_KEY 并 npm install --prefix scripts/cursor-agent。已退出 Cursor 模式。", nil
	}

	conversationID, _ := o.sessions.LoadActiveMessages(ownerKey)
	ws := o.sessions.ActiveWorkspace(ownerKey, o.defaultWorkspace())
	agentID := o.sessions.LoadActiveCursorAgentID(ownerKey)
	result, err := cursor.Chat(models.Request{
		SessionKey:      ownerKey,
		ConversationID:  conversationID,
		Prompt:          text,
		ExternalAgentID: agentID,
		Workspace:       ws,
	})
	// 旧 agentId 可能已失效（bot/SDK 重启等）；清掉后无 id 重试一次。
	if err != nil && agentID != "" && isStaleCursorAgentErr(err) {
		o.sessions.SetCursorAgentID(ownerKey, "")
		result, err = cursor.Chat(models.Request{
			SessionKey:     ownerKey,
			ConversationID: conversationID,
			Prompt:         text,
			Workspace:      ws,
		})
	}
	if err != nil {
		return "", err
	}
	o.recordUsage(ownerKey, result.Usage)
	if result.ExternalAgentID != "" {
		o.sessions.SetCursorAgentID(ownerKey, result.ExternalAgentID)
	}
	o.sessions.AppendTurn(ownerKey, text, result.Text)
	return fmt.Sprintf("[Cursor | %s]\n%s", ws, result.Text), nil
}

func isStaleCursorAgentErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") && strings.Contains(msg, "agent")
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

// handleCommand 解析以 / 开头的指令；handled=false 表示交给普通对话。
func (o *Orchestrator) handleCommand(ownerKey, text string) (reply string, handled bool, err error) {
	if !strings.HasPrefix(text, "/") {
		return "", false, nil
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", false, nil
	}
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))

	switch cmd {
	case "/help":
		return helpText(), true, nil
	case "/list":
		return formatList(o.sessions.List(ownerKey)), true, nil
	case "/new":
		item, err := o.sessions.Create(ownerKey, arg)
		if err != nil {
			return err.Error(), true, nil
		}
		return fmt.Sprintf("已新建并切换到 [%d] %s", item.Index, item.Title), true, nil
	case "/use":
		n, err := strconv.Atoi(strings.TrimSpace(arg))
		if err != nil || n < 1 {
			return "用法：/use 序号（先 /list）", true, nil
		}
		item, err := o.sessions.Switch(ownerKey, n)
		if err != nil {
			return err.Error(), true, nil
		}
		return fmt.Sprintf("已切换到 [%d] %s（%d 条消息）", item.Index, item.Title, item.Msgs), true, nil
	case "/rename":
		item, err := o.sessions.Rename(ownerKey, arg)
		if err != nil {
			return err.Error(), true, nil
		}
		return fmt.Sprintf("已重命名 [%d] → %s", item.Index, item.Title), true, nil
	case "/clear":
		if strings.EqualFold(strings.TrimSpace(arg), "tokens") || strings.EqualFold(strings.TrimSpace(arg), "token") {
			view := o.sessions.ClearUsage(ownerKey, false)
			return fmt.Sprintf("已清空 [%d] %s 的 token 统计", view.Index, view.Title), true, nil
		}
		item := o.sessions.ClearActive(ownerKey)
		return fmt.Sprintf("已清空 [%d] %s 的上下文（含 Cursor Agent）", item.Index, item.Title), true, nil
	case "/ctx", "/context":
		return formatContext(o.sessions.ActiveSnapshot(ownerKey)), true, nil
	case "/tokens", "/usage":
		argLower := strings.ToLower(strings.TrimSpace(arg))
		switch argLower {
		case "clear", "reset":
			view := o.sessions.ClearUsage(ownerKey, false)
			return fmt.Sprintf("已清空 [%d] %s 的 token 统计", view.Index, view.Title), true, nil
		case "clear all", "reset all":
			view := o.sessions.ClearUsage(ownerKey, true)
			return fmt.Sprintf("已清空本用户全部会话的 token 统计（当前 [%d] %s）", view.Index, view.Title), true, nil
		default:
			return formatTokens(o.sessions.ActiveSnapshot(ownerKey)), true, nil
		}
	case "/workspace", "/cwd", "/pwd", "/repo":
		return o.handleWorkspace(ownerKey, arg), true, nil
	case "/cd":
		return o.handleCd(ownerKey, arg), true, nil
	case "/ls", "/dir":
		return o.handleLs(ownerKey, arg), true, nil
	case "/cursor":
		out, err := o.handleCursorMode(ownerKey, arg)
		return out, true, err
	default:
		return "未知指令。发送 /help 查看可用命令。", true, nil
	}
}

func (o *Orchestrator) defaultWorkspace() string {
	if o.cfg != nil && o.cfg.ToolWorkspace != "" {
		return o.cfg.ToolWorkspace
	}
	return "."
}

func (o *Orchestrator) currentDir(ownerKey string) string {
	return o.sessions.ActiveWorkspace(ownerKey, o.cursorRoot())
}

func (o *Orchestrator) handleWorkspace(ownerKey, arg string) string {
	arg = strings.TrimSpace(arg)
	root := o.cursorRoot()
	cur := o.currentDir(ownerKey)
	if arg == "" {
		return fmt.Sprintf("默认目录：%s\n当前目录：%s\n可用对话让模型 set_workdir 切到任意路径；/workspace clear 回默认", root, cur)
	}
	if strings.EqualFold(arg, "clear") || strings.EqualFold(arg, "reset") || arg == "-" || arg == "/" || arg == `\` {
		o.sessions.ClearWorkspace(ownerKey)
		return fmt.Sprintf("已回到 Cursor 根目录：\n%s", root)
	}
	abs, err := o.resolveAnyDir(arg, cur)
	if err != nil {
		return "无效路径：" + err.Error()
	}
	o.sessions.SetWorkspace(ownerKey, abs)
	return fmt.Sprintf("已进入：\n%s\n可用自然语言让我切换目录，或 /cursor 需求 开始改代码。", abs)
}

func (o *Orchestrator) handleCd(ownerKey, arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "用法：/cd 路径（任意盘符）  或  /cd ..\n当前：" + o.currentDir(ownerKey)
	}
	if arg == "/" || arg == `\` || strings.EqualFold(arg, "root") {
		o.sessions.ClearWorkspace(ownerKey)
		return "已回到默认目录：\n" + o.cursorRoot()
	}
	abs, err := o.resolveAnyDir(arg, o.currentDir(ownerKey))
	if err != nil {
		return "无法进入：" + err.Error()
	}
	o.sessions.SetWorkspace(ownerKey, abs)
	return fmt.Sprintf("已进入：\n%s", abs)
}

func (o *Orchestrator) handleLs(ownerKey, arg string) string {
	dir := o.currentDir(ownerKey)
	if strings.TrimSpace(arg) != "" {
		abs, err := o.resolveAnyDir(arg, dir)
		if err != nil {
			return "无法列出：" + err.Error()
		}
		dir = abs
	}
	out, err := listDirEntries(dir, 80)
	if err != nil {
		return "列出失败：" + err.Error()
	}
	return out
}

func looksLikePath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\\`) {
		return true
	}
	// Windows drive path: E:\ or E:/
	if len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') {
		return true
	}
	return false
}

// handleCursorMode 处理 /cursor [exit|status|路径|需求]。
func (o *Orchestrator) handleCursorMode(ownerKey, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	argLower := strings.ToLower(arg)
	ws := o.currentDir(ownerKey)

	switch argLower {
	case "exit", "off", "leave", "quit", "end":
		if !o.sessions.IsCursorMode(ownerKey) {
			return "当前不在 Cursor 模式。", nil
		}
		o.sessions.SetCursorMode(ownerKey, false)
		return "已退出 Cursor 模式，后续消息回到普通对话。", nil
	case "status", "？", "?":
		mode := "关"
		if o.sessions.IsCursorMode(ownerKey) {
			mode = "开"
		}
		return fmt.Sprintf("Cursor 模式：%s\n工作目录：%s\n进入：/cursor\n退出：/cursor exit", mode, ws), nil
	}

	if o.cursorChannel() == nil {
		return "未启用 Cursor：请在 .env 配置 CURSOR_API_KEY，并执行 npm install --prefix scripts/cursor-agent", nil
	}

	// Optional: /cursor D:\repo — set workdir then enter mode
	if looksLikePath(arg) {
		abs, err := o.resolveAnyDir(arg, ws)
		if err != nil {
			return "无效目录：" + err.Error(), nil
		}
		o.sessions.SetWorkspace(ownerKey, abs)
		ws = abs
		arg = ""
	}

	already := o.sessions.IsCursorMode(ownerKey)
	o.sessions.SetCursorMode(ownerKey, true)
	if arg == "" {
		if already {
			return fmt.Sprintf("已在 Cursor 模式。\n工作目录：%s\n直接发需求即可；退出：/cursor exit", ws), nil
		}
		return fmt.Sprintf("已进入 Cursor 模式。\n工作目录：%s\n接下来普通消息都会交给 Cursor 执行；退出：/cursor exit", ws), nil
	}

	// /cursor 修一下登录 — enter mode and run first prompt
	return o.chatWithCursor(ownerKey, arg)
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
/cursor — 进入 Cursor 模式（之后普通消息走 Cursor）
/cursor exit — 退出 Cursor 模式
/cursor status — 查看是否在 Cursor 模式
/workspace — 查看当前工作目录
/clear — 清空当前上下文（含 Cursor Agent）

可先对话让模型 set_workdir 切目录，再 /cursor 进入模式改代码。`)
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

// modelSummarizer 用主模型做会话摘要；DisableTools 避免压缩时再调工具。
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
