package models

import (
	"encoding/json"
	"testing"
)

func TestCursorAgentPayloadRoundTrip(t *testing.T) {
	in := cursorAgentIn{
		Prompt:  "add hello.txt",
		Cwd:     `E:\workspace\demo`,
		APIKey:  "cursor_test",
		Model:   "grok-4.7",
		AgentID: "agent-1",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out cursorAgentIn
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Prompt != in.Prompt || out.AgentID != in.AgentID || out.Cwd != in.Cwd {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}

	raw := `{"ok":true,"text":"done","agentId":"agent-2","status":"finished","usage":{"promptTokens":1,"completionTokens":2,"totalTokens":3}}`
	resp, err := parseCursorAgentOut(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.AgentID != "agent-2" || resp.Usage == nil || resp.Usage.TotalTokens != 3 {
		t.Fatalf("parse out: %+v", resp)
	}
}

func TestParseCursorAgentOutIgnoresSDKLogNoise(t *testing.T) {
	raw := "16:18:00.251 INFO  Resolved the fallback encoding for shell output meta={encoding: \"cp936\"}\n" +
		`{"ok":true,"text":"pong","agentId":"agent-x","status":"finished"}`
	resp, err := parseCursorAgentOut(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Text != "pong" || resp.AgentID != "agent-x" {
		t.Fatalf("got %+v", resp)
	}
}

func TestParseCursorAgentOutEmpty(t *testing.T) {
	if _, err := parseCursorAgentOut("   "); err == nil {
		t.Fatal("expected error")
	}
}
