package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memWorkdirs struct {
	m map[string]string
}

func (m *memWorkdirs) GetWorkdir(sessionKey string) string { return m.m[sessionKey] }
func (m *memWorkdirs) SetWorkdir(sessionKey, absPath string) error {
	if m.m == nil {
		m.m = map[string]string{}
	}
	m.m[sessionKey] = absPath
	return nil
}

func TestSetWorkdirThenList(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	if err := os.Mkdir(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &memWorkdirs{}
	reg, err := NewRegistry(Options{Workspace: root, Workdirs: store, MaxOutputBytes: 10_000})
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithSessionKey(t.Context(), "qq:c:u")
	out, err := reg.Execute(ctx, "set_workdir", `{"path":"proj"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "proj") {
		t.Fatalf("set out=%s", out)
	}
	got := filepath.Clean(store.GetWorkdir("qq:c:u"))
	if !strings.HasSuffix(strings.ToLower(got), strings.ToLower(filepath.Base(proj))) {
		t.Fatalf("store cwd=%q want suffix %q", got, filepath.Base(proj))
	}
	listing, err := reg.Execute(ctx, "list_dir", `{"path":"."}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listing, "a.txt") {
		t.Fatalf("list=%q", listing)
	}
}
