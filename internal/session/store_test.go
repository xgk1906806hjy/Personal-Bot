package session

import (
	"fmt"
	"testing"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

func TestMultiConversationSwitch(t *testing.T) {
	s := NewStore(2, 30, 16)
	key := BuildKey(config.PlatformQQ, "chat1", "user1")

	a, err := s.Create(key, "工作")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Active || a.Title != "工作" {
		t.Fatalf("create: %+v", a)
	}

	b, err := s.Create(key, "闲聊")
	if err != nil {
		t.Fatal(err)
	}
	list := s.List(key)
	if len(list) != 3 { // default + 2
		t.Fatalf("want 3 convos, got %d", len(list))
	}
	if !list[2].Active || list[2].Title != "闲聊" {
		t.Fatalf("active should be 闲聊: %+v", list)
	}

	sw, err := s.Switch(key, a.Index)
	if err != nil {
		t.Fatal(err)
	}
	if sw.Title != "工作" || !sw.Active {
		t.Fatalf("switch: %+v", sw)
	}

	s.AppendTurn(key, "hello", "hi")
	id, msgs := s.LoadActiveMessages(key)
	if id != a.ID || len(msgs) != 2 {
		t.Fatalf("load active: id=%s msgs=%d", id, len(msgs))
	}

	_, err = s.Switch(key, b.Index)
	if err != nil {
		t.Fatal(err)
	}
	_, msgs = s.LoadActiveMessages(key)
	if len(msgs) != 0 {
		t.Fatalf("other convo should be empty, got %d", len(msgs))
	}

	renamed, err := s.Rename(key, "随便聊聊")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Title != "随便聊聊" {
		t.Fatalf("rename: %+v", renamed)
	}

	cleared := s.ClearActive(key)
	if cleared.Msgs != 0 {
		t.Fatalf("clear: %+v", cleared)
	}
}

func TestMaxConversations(t *testing.T) {
	s := NewStore(2, 30, 16)
	key := BuildKey(config.PlatformQQ, "c", "u")
	// default already 1
	for i := 0; i < MaxConversations-1; i++ {
		if _, err := s.Create(key, fmt.Sprintf("t%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Create(key, "overflow"); err == nil {
		t.Fatal("expected max conversations error")
	}
}

type stubSummarizer struct {
	calls int
	text  string
	err   error
}

func (s *stubSummarizer) Summarize(old []Message) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	if s.text != "" {
		return s.text, nil
	}
	return fmt.Sprintf("摘要共%d条", len(old)), nil
}

func TestMaybeCompress(t *testing.T) {
	s := NewStore(2, 30, 16)
	key := BuildKey(config.PlatformQQ, "c2", "u2")

	for i := 0; i < 16; i++ {
		s.AppendTurn(key, fmt.Sprintf("u%d", i), fmt.Sprintf("a%d", i))
	}
	// 32 messages > 30
	sum := &stubSummarizer{}
	if err := s.MaybeCompress(key, sum); err != nil {
		t.Fatal(err)
	}
	if sum.calls != 1 {
		t.Fatalf("summarize calls=%d", sum.calls)
	}
	_, msgs := s.LoadActiveMessages(key)
	// 1 summary + 16 recent
	if len(msgs) != 17 {
		t.Fatalf("want 17 msgs after compress, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content == "" {
		t.Fatalf("first should be summary system msg: %+v", msgs[0])
	}
	if msgs[1].Content != "u8" { // kept last 16 of 32 → starts at turn index 8 (msgs 16..)
		// 32 messages: indices 0..31; keep last 16 → indices 16..31
		// message 16 is user of turn 8 → "u8"
		t.Fatalf("unexpected first kept content: %q", msgs[1].Content)
	}

	// Under threshold: no-op
	sum2 := &stubSummarizer{}
	if err := s.MaybeCompress(key, sum2); err != nil {
		t.Fatal(err)
	}
	if sum2.calls != 0 {
		t.Fatalf("should not compress again, calls=%d", sum2.calls)
	}
}

func TestUsageAndClear(t *testing.T) {
	s := NewStore(2, 30, 16)
	key := BuildKey(config.PlatformQQ, "c4", "u4")
	s.AddUsage(key, 100, 50, 150, false)
	s.AddUsage(key, 20, 10, 30, true)
	view := s.ActiveSnapshot(key)
	if view.ConvoUsage.TotalTokens != 180 || view.ConvoUsage.Calls != 2 {
		t.Fatalf("convo usage: %+v", view.ConvoUsage)
	}
	if view.OwnerUsage.TotalTokens != 180 {
		t.Fatalf("owner usage: %+v", view.OwnerUsage)
	}
	s.ClearUsage(key, false)
	view = s.ActiveSnapshot(key)
	if view.ConvoUsage.Calls != 0 || view.OwnerUsage.Calls != 0 {
		t.Fatalf("after clear: convo=%+v owner=%+v", view.ConvoUsage, view.OwnerUsage)
	}
}

func TestMaybeCompressFallback(t *testing.T) {
	s := NewStore(2, 30, 16)
	key := BuildKey(config.PlatformQQ, "c3", "u3")
	for i := 0; i < 16; i++ {
		s.AppendTurn(key, "u", "a")
	}
	sum := &stubSummarizer{err: fmt.Errorf("boom")}
	if err := s.MaybeCompress(key, sum); err != nil {
		t.Fatal(err)
	}
	_, msgs := s.LoadActiveMessages(key)
	if len(msgs) != 16 {
		t.Fatalf("fallback keep recent: got %d", len(msgs))
	}
}
