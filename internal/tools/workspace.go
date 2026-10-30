package tools

import (
	"path/filepath"
	"strings"
)

// Workspace holds the default starting directory (TOOL_WORKSPACE).
// Session workdirs may point anywhere on the machine; relative paths resolve
// from the current workdir with no root jail.
type Workspace struct {
	Root string
}

func NewWorkspace(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		abs, err = filepath.Abs(root)
		if err != nil {
			return nil, err
		}
	}
	return &Workspace{Root: abs}, nil
}

// Resolve joins rel under the default root.
func (w *Workspace) Resolve(rel string) (string, error) {
	return w.ResolveFrom(w.Root, rel)
}

// ResolveFrom joins rel under base, or accepts an absolute path as-is.
// No confinement to TOOL_WORKSPACE — any reachable path is allowed.
func (w *Workspace) ResolveFrom(base, rel string) (string, error) {
	if rel == "" {
		rel = "."
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = w.Root
	}
	base = filepath.Clean(base)
	cleaned := filepath.Clean(rel)
	var target string
	if filepath.IsAbs(cleaned) {
		target = cleaned
	} else {
		target = filepath.Join(base, cleaned)
	}
	return filepath.Abs(target)
}
