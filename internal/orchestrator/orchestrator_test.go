package orchestrator

import (
	"fmt"
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

type fakeCursor struct {
	last models.Request
	n    int
}

func (f *fakeCursor) ID() config.ModelChannel { return config.ModelCursorLocal }
func (f *fakeCursor) Chat(req models.Request) (models.Response, error) {
	f.last = req
	f.n++
	return models.Response{
		Channel:         f.ID(),
		Text:            "cursor:" + req.Prompt,
		ConversationID:  req.ConversationID,
		ExternalAgentID: "agent-test",
		Usage:           models.TokenUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}

func TestIsStaleCursorAgentErr(t *testing.T) {
	if !isStaleCursorAgentErr(fmt.Errorf("cursor_local failed: Agent agent-x not found")) {
		t.Fatal("expected stale")
	}
	if isStaleCursorAgentErr(fmt.Errorf("cursor_local failed: rate limited")) {
		t.Fatal("unexpected stale")
	}
}

type staleThenOKCursor struct {
	calls int
	last  models.Request
}

func (f *staleThenOKCursor) ID() config.ModelChannel { return config.ModelCursorLocal }
func (f *staleThenOKCursor) Chat(req models.Request) (models.Response, error) {
	f.calls++
	f.last = req
	if f.calls == 1 && req.ExternalAgentID != "" {
		return models.Response{}, fmt.Errorf("cursor_local failed: Agent %s not found", req.ExternalAgentID)
	}
	return models.Response{
		Channel:         f.ID(),
		Text:            "recovered",
		ExternalAgentID: "agent-new",
		Usage:           models.TokenUsage{TotalTokens: 1},
	}, nil
}

func TestChatWithCursorRecoversStaleAgent(t *testing.T) {
	cfg := &config.Config{ToolWorkspace: `E:\cursor`}
	store := session.NewStore(2, 30, 16)
	key := session.BuildKey(config.PlatformQQ, "c", "u")
	store.SetCursorMode(key, true)
	store.SetCursorAgentID(key, "agent-old")
	c := &staleThenOKCursor{}
	o := New(cfg, &fakeModel{}, store, c)

	reply, err := o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "继续"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "recovered") {
		t.Fatalf("reply=%q", reply)
	}
	if c.calls != 2 {
		t.Fatalf("calls=%d want 2", c.calls)
	}
	if c.last.ExternalAgentID != "" {
		t.Fatalf("retry should clear agent id, got %q", c.last.ExternalAgentID)
	}
	if store.LoadActiveCursorAgentID(key) != "agent-new" {
		t.Fatalf("want new agent id stored")
	}
}

func TestSlashCommands(t *testing.T) {
	cfg := &config.Config{SessionCompressAfter: 30, SessionKeepRecent: 16}
	store := session.NewStore(2, 30, 16)
	m := &fakeModel{}
	o := New(cfg, m, store, nil)

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

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/cursor"})
	if err != nil || !strings.Contains(reply, "CURSOR_API_KEY") {
		t.Fatalf("cursor disabled: %q err=%v", reply, err)
	}
}

func TestCursorMode(t *testing.T) {
	cfg := &config.Config{SessionCompressAfter: 30, SessionKeepRecent: 16, ToolWorkspace: `E:\cursor`}
	store := session.NewStore(2, 30, 16)
	m := &fakeModel{}
	c := &fakeCursor{}
	o := New(cfg, m, store, c)
	key := session.BuildKey(config.PlatformQQ, "c", "u")

	reply, err := o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/cursor"})
	if err != nil || !strings.Contains(reply, "已进入 Cursor 模式") {
		t.Fatalf("enter: %q err=%v", reply, err)
	}
	if !store.IsCursorMode(key) {
		t.Fatal("expected cursor mode on")
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "加 hello.txt"})
	if err != nil || !strings.Contains(reply, "cursor:加 hello.txt") {
		t.Fatalf("in-mode chat: %q err=%v", reply, err)
	}
	if c.n != 1 {
		t.Fatalf("cursor calls=%d", c.n)
	}
	if c.last.ExternalAgentID != "" {
		t.Fatalf("first agent id should be empty")
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "再改一下"})
	if err != nil || !strings.Contains(reply, "cursor:再改一下") {
		t.Fatalf("follow-up: %q err=%v", reply, err)
	}
	if c.last.ExternalAgentID != "agent-test" {
		t.Fatalf("want resume, got %q", c.last.ExternalAgentID)
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "/cursor exit"})
	if err != nil || !strings.Contains(reply, "已退出") {
		t.Fatalf("exit: %q err=%v", reply, err)
	}
	if store.IsCursorMode(key) {
		t.Fatal("expected cursor mode off")
	}

	reply, err = o.Handle(IncomingUserMessage{Platform: config.PlatformQQ, ChatID: "c", UserID: "u", Text: "普通聊天"})
	if err != nil || !strings.HasPrefix(reply, "ok:") {
		t.Fatalf("normal after exit: %q err=%v", reply, err)
	}
}
