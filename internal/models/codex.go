package models

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

// CodexLocalChannel shells out to the Codex CLI (no Node SDK in Go).
// Requires Codex CLI installed and authenticated on this machine.
type CodexLocalChannel struct {
	cfg *config.Config

	mu        sync.Mutex
	histories map[string][]ChatMessage
}

func newCodexLocalChannel(cfg *config.Config) *CodexLocalChannel {
	return &CodexLocalChannel{
		cfg:       cfg,
		histories: make(map[string][]ChatMessage),
	}
}

func (c *CodexLocalChannel) ID() config.ModelChannel { return config.ModelCodexLocal }

func (c *CodexLocalChannel) Chat(req Request) (Response, error) {
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = req.SessionKey
	}

	c.mu.Lock()
	history := append([]ChatMessage{}, c.histories[conversationID]...)
	c.mu.Unlock()

	prompt := c.buildPrompt(req, history)
	bin := c.cfg.CodexBin
	if bin == "" {
		bin = "codex"
	}

	// Non-interactive one-shot. History is embedded in the prompt for multi-turn.
	cmd := exec.Command(bin, "exec", "--skip-git-repo-check", prompt)
	cmd.Dir = c.cfg.CodexWorkingDirectory
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Minute):
		_ = cmd.Process.Kill()
		err = fmt.Errorf("codex exec timed out")
	}
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if out != "" {
			detail = out + "\n" + detail
		}
		return Response{}, fmt.Errorf("codex_local failed: %s", detail)
	}
	if out == "" {
		out = "(Codex returned empty response)"
	}

	c.mu.Lock()
	history = append(history,
		ChatMessage{Role: RoleUser, Content: req.Prompt},
		ChatMessage{Role: RoleAssistant, Content: out},
	)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	c.histories[conversationID] = history
	c.mu.Unlock()

	return Response{
		Channel:        c.ID(),
		Text:           out,
		ConversationID: conversationID,
	}, nil
}

func (c *CodexLocalChannel) buildPrompt(req Request, history []ChatMessage) string {
	var b strings.Builder
	b.WriteString(c.cfg.BotSystemPrompt)
	b.WriteString("\n\nConversation context (if any). Answer the latest user message.\n")
	if len(history) == 0 {
		b.WriteString("User: ")
		b.WriteString(req.Prompt)
	} else {
		for _, m := range history {
			switch m.Role {
			case RoleUser:
				b.WriteString("User: ")
			default:
				b.WriteString("Assistant: ")
			}
			b.WriteString(m.Content)
			b.WriteString("\n\n")
		}
		b.WriteString("User: ")
		b.WriteString(req.Prompt)
	}
	b.WriteString("\n\nWorking directory: ")
	b.WriteString(c.cfg.CodexWorkingDirectory)
	return b.String()
}
