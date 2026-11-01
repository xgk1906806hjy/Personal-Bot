package tools

import (
	"path/filepath"
	"strings"
)

// Workspace 保存默认起点目录（TOOL_WORKSPACE）。
// 会话 workdir 可指向本机任意路径；相对路径相对当前 workdir 解析，无根目录监禁。
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
