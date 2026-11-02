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

// Resolve 把相对路径拼到默认根目录下。
func (w *Workspace) Resolve(rel string) (string, error) {
	return w.ResolveFrom(w.Root, rel)
}

// ResolveFrom 相对 base 拼接路径；若已是绝对路径则原样接受。
// 不限制在 TOOL_WORKSPACE 内，本机可达路径均可。
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
