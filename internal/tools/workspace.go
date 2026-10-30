package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Workspace confines file operations under Root.
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
		// Root may not exist yet; keep abs without symlink resolve.
		abs, err = filepath.Abs(root)
		if err != nil {
			return nil, err
		}
	}
	return &Workspace{Root: abs}, nil
}

// Resolve joins rel under root and rejects path escape.
func (w *Workspace) Resolve(rel string) (string, error) {
	if rel == "" {
		rel = "."
	}
	cleaned := filepath.Clean(rel)
	if filepath.IsAbs(cleaned) {
		// Allow absolute only if still inside root.
		target := filepath.Clean(cleaned)
		return w.ensureInside(target)
	}
	target := filepath.Join(w.Root, cleaned)
	return w.ensureInside(target)
}

func (w *Workspace) ensureInside(target string) (string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	root := filepath.Clean(w.Root)
	// Use Rel to detect escape; if Rel starts with .. then outside.
	rel, err := filepath.Rel(root, absTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes tool workspace: %s", target)
	}
	return absTarget, nil
}
