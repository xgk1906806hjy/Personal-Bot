package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceAllowsAbsoluteOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	ws, err := NewWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ws.ResolveFrom(root, other)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(other) {
		t.Fatalf("got %s want %s", got, other)
	}
}

func TestWorkspaceRelativeFromBase(t *testing.T) {
	root := t.TempDir()
	ws, err := NewWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ws.Resolve("subdir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join(ws.Root, "subdir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestReadWriteListRoundTrip(t *testing.T) {
	root := t.TempDir()
	reg, err := NewRegistry(Options{
		Workspace:      root,
		MaxOutputBytes: 10_000,
		EnableShell:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	names := reg.ToolNames()
	want := map[string]bool{"read_file": true, "write_file": true, "list_dir": true, "get_workdir": true, "set_workdir": true}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools: %v", want)
	}

	_, err = reg.Execute(t.Context(), "write_file", `{"path":"a/hello.txt","content":"hi"}`)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "a", "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hi" {
		t.Fatalf("content=%q", data)
	}
	out, err := reg.Execute(t.Context(), "read_file", `{"path":"a/hello.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi" {
		t.Fatalf("read=%q", out)
	}
	listing, err := reg.Execute(t.Context(), "list_dir", `{"path":"a"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listing, "hello.txt") {
		t.Fatalf("list=%q", listing)
	}
}
