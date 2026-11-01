package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

// CursorLocalChannel 通过 Node 助手脚本调用 Cursor SDK 本地 Agent。
// stdin/stdout 交换 JSON；SDK 日志被脚本重定向到 stderr，避免污染结果。
type CursorLocalChannel struct {
	cfg *config.Config
}

func newCursorLocalChannel(cfg *config.Config) *CursorLocalChannel {
	return &CursorLocalChannel{cfg: cfg}
}

func (c *CursorLocalChannel) ID() config.ModelChannel { return config.ModelCursorLocal }

// parseCursorAgentOut accepts a single JSON object, or the last JSON object line
// when the SDK leaks log lines onto stdout.
func parseCursorAgentOut(raw string) (cursorAgentOut, error) {
	raw = strings.TrimSpace(raw)
	var parsed cursorAgentOut
	if raw == "" {
		return parsed, fmt.Errorf("empty stdout from cursor agent")
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		return parsed, nil
	}
	lines := strings.Split(raw, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || line[0] != '{' {
			continue
		}
		if err := json.Unmarshal([]byte(line), &parsed); err == nil {
			return parsed, nil
		}
	}
	snippet := raw
	if len(snippet) > 240 {
		snippet = snippet[:240] + "…"
	}
	return cursorAgentOut{}, fmt.Errorf("invalid JSON from cursor agent: %s", snippet)
}

type cursorAgentIn struct {
	Prompt  string `json:"prompt"`
	Cwd     string `json:"cwd"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
	AgentID string `json:"agentId,omitempty"`
}

type cursorAgentOut struct {
	OK        bool   `json:"ok"`
	Text      string `json:"text"`
	AgentID   string `json:"agentId"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	Retryable bool   `json:"retryable"`
	Usage     *struct {
		PromptTokens     int `json:"promptTokens"`
		CompletionTokens int `json:"completionTokens"`
		TotalTokens      int `json:"totalTokens"`
	} `json:"usage"`
}

func (c *CursorLocalChannel) Chat(req Request) (Response, error) {
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = req.SessionKey
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return Response{}, fmt.Errorf("cursor_local: empty prompt")
	}
	// On first turn (no Cursor agent yet), prepend bot system prompt once.
	if req.ExternalAgentID == "" && c.cfg.BotSystemPrompt != "" {
		prompt = c.cfg.BotSystemPrompt + "\n\nUser request:\n" + prompt
	}

	cwd := strings.TrimSpace(req.Workspace)
	if cwd == "" {
		cwd = c.cfg.ToolWorkspace
	}

	payload, err := json.Marshal(cursorAgentIn{
		Prompt:  prompt,
		Cwd:     cwd,
		APIKey:  c.cfg.CursorAPIKey,
		Model:   c.cfg.CursorModel,
		AgentID: req.ExternalAgentID,
	})
	if err != nil {
		return Response{}, err
	}

	bin := c.cfg.CursorNodeBin
	if bin == "" {
		bin = "node"
	}
	cmd := exec.Command(bin, c.cfg.CursorAgentScript)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(30 * time.Minute):
		_ = cmd.Process.Kill()
		runErr = fmt.Errorf("cursor agent timed out after 30m")
	}

	rawOut := strings.TrimSpace(stdout.String())
	parsed, parseErr := parseCursorAgentOut(rawOut)

	if runErr != nil {
		msg := parsed.Error
		if msg == "" {
			msg = strings.TrimSpace(stderr.String())
		}
		if msg == "" && parseErr != nil {
			msg = parseErr.Error()
		}
		if msg == "" {
			msg = runErr.Error()
		}
		return Response{}, fmt.Errorf("cursor_local failed: %s", msg)
	}
	if parseErr != nil {
		return Response{}, fmt.Errorf("cursor_local failed: %w", parseErr)
	}
	if !parsed.OK {
		msg := parsed.Error
		if msg == "" {
			msg = "cursor agent returned ok=false"
		}
		return Response{}, fmt.Errorf("cursor_local failed: %s", msg)
	}

	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		text = "(Cursor returned empty response)"
	}

	usage := TokenUsage{}
	if parsed.Usage != nil {
		usage = TokenUsage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
	} else {
		usage = EstimateUsage(prompt, text)
	}

	return Response{
		Channel:         c.ID(),
		Text:            text,
		ConversationID:  conversationID,
		ExternalAgentID: parsed.AgentID,
		Usage:           usage,
	}, nil
}
