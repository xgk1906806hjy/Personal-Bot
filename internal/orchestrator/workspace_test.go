package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/session"
)

func TestResolveAnyDirAllowsOutsideDefault(t *testing.T) {
	defaultRoot := t.TempDir()
	other := t.TempDir()
	cfg := &config.Config{ToolWorkspace: defaultRoot}
	o := New(cfg, &fakeModel{}, session.NewStore(2, 30, 16), nil)

	got, err := o.resolveAnyDir(other, defaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(other) {
		t.Fatalf("got %q want %q", got, other)
	}

	sub := filepath.Join(other, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = o.resolveAnyDir("nested", other)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(sub) {
		t.Fatalf("rel got %q", got)
	}
}
