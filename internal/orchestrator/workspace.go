package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (o *Orchestrator) cursorRoot() string {
	return filepath.Clean(o.defaultWorkspace())
}

// resolveAnyDir resolves raw (absolute or relative to current) to an existing directory.
// No drive/root jail — any path on the machine is allowed.
func (o *Orchestrator) resolveAnyDir(raw, current string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	if raw == "" {
		return "", fmt.Errorf("路径为空")
	}

	var candidate string
	if filepath.IsAbs(raw) || looksLikePath(raw) {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return "", err
		}
		candidate = filepath.Clean(abs)
	} else {
		base := current
		if base == "" {
			base = o.cursorRoot()
		}
		candidate = filepath.Clean(filepath.Join(base, raw))
	}

	st, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("不是目录：%s", candidate)
	}
	return candidate, nil
}

func listDirEntries(dir string, max int) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if max <= 0 {
		max = 80
	}
	var b strings.Builder
	fmt.Fprintf(&b, "目录 %s\n", dir)
	n := 0
	dirs := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		n++
		if n > max {
			fmt.Fprintf(&b, "…还有更多（只显示前 %d 项）\n", max)
			break
		}
		if e.IsDir() {
			dirs++
			fmt.Fprintf(&b, "[dir] %s/\n", e.Name())
		} else {
			fmt.Fprintf(&b, "      %s\n", e.Name())
		}
	}
	if n == 0 {
		b.WriteString("（空）\n")
	}
	fmt.Fprintf(&b, "— %d 项（目录 %d）", n, dirs)
	return strings.TrimSpace(b.String()), nil
}
