package orchestrator

import (
	"strings"
	"testing"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/models"
	"github.com/xgk1906806hjy/Personal-Bot/internal/session"
)

type fakeModel struct {
	last models.Request
	text string
}

func (f *fakeModel) ID() config.ModelChannel { return config.ModelGatewayAPI }
func (f *fakeModel) Chat(req models.Request) (models.Response, error) {
	f.last = req
	out := f.text
	if out == "" {
		out = "ok:" + req.Prompt
	}
	return models.Response{
		Channel:        f.ID(),
		Text:           out,
		ConversationID: req.ConversationID,
		Usage: models.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func TestSlashCommands(t *testing.T) {
	cfg := &config.Config{SessionCompressAfter: 30, SessionKeepRecent: 16}
	store := session.NewStore(2, 30, 16)
	m := &fakeModel{}
	o := New(cfg, m, store)

	msg := IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/help"}
	reply, err := o.Handle(msg)
	if err != nil || !strings.Contains(reply, "/list") {
		t.Fatalf("help: %q err=%v", reply, err)
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/new 项目A"})
	if err != nil || !strings.Contains(reply, "项目A") {
		t.Fatalf("new: %q err=%v", reply, err)
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "你好"})
	if err != nil || !strings.HasPrefix(reply, "ok:") {
		t.Fatalf("chat: %q err=%v", reply, err)
	}
	if m.last.DisableTools {
		t.Fatal("normal chat should allow tools")
	}
	if len(m.last.Messages) != 0 {
		t.Fatalf("first turn history should be empty, got %d", len(m.last.Messages))
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/list"})
	if err != nil || !strings.Contains(reply, "项目A") {
		t.Fatalf("list: %q", reply)
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/ctx"})
	if err != nil || !strings.Contains(reply, "你好") || !strings.Contains(reply, "项目A") {
		t.Fatalf("ctx: %q err=%v", reply, err)
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/tokens"})
	if err != nil || !strings.Contains(reply, "合计 15") {
		t.Fatalf("tokens: %q err=%v", reply, err)
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/tokens clear"})
	if err != nil || !strings.Contains(reply, "已清空") {
		t.Fatalf("tokens clear: %q err=%v", reply, err)
	}
	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/tokens"})
	if err != nil || !strings.Contains(reply, "调用 0 次") {
		t.Fatalf("tokens after clear: %q", reply)
	}
}
