package models

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

// CodexLocalChannel shells out to the Codex CLI (no Node SDK in Go).
// Requires Codex CLI installed and authenticated on this machine.
type CodexLocalChannel struct {
	cfg *config.Config
}

func newCodexLocalChannel(cfg *config.Config) *CodexLocalChannel {
	return &CodexLocalChannel{cfg: cfg}
}

func (c *CodexLocalChannel) ID() config.ModelChannel { return config.ModelCodexLocal }

func (c *CodexLocalChannel) Chat(req Request) (Response, error) {
	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = req.SessionKey
	}

	prompt := c.buildPrompt(req)
	bin := c.cfg.CodexBin
	if bin == "" {
		bin = "codex"
	}

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

	return Response{
		Channel:        c.ID(),
		Text:           out,
		ConversationID: conversationID,
		Usage:          EstimateUsage(prompt, out),
	}, nil
}

func (c *CodexLocalChannel) buildPrompt(req Request) string {
	var b strings.Builder
	b.WriteString(c.cfg.BotSystemPrompt)
	b.WriteString("\n\nConversation context (if any). Answer the latest user message.\n")
	if len(req.Messages) == 0 {
		b.WriteString("User: ")
		b.WriteString(req.Prompt)
	} else {
		for _, m := range req.Messages {
			switch m.Role {
			case RoleUser:
				b.WriteString("User: ")
			case RoleSystem:
				b.WriteString("System: ")
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
