package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// get_workdir / set_workdir：查看与切换会话工作目录（可任意绝对路径）。

type getWorkdirTool struct {
	r *pathResolver
}

func newGetWorkdirTool(r *pathResolver) *getWorkdirTool {
	return &getWorkdirTool{r: r}
}

func (t *getWorkdirTool) Name() string { return "get_workdir" }
func (t *getWorkdirTool) Description() string {
	return "Get the current working directory for this chat. Use before switching projects."
}
func (t *getWorkdirTool) Parameters() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func (t *getWorkdirTool) Execute(ctx context.Context, _ string) (string, error) {
	cwd := t.r.cwd(ctx)
	return mustJSON(map[string]any{
		"default_root": t.r.ws.Root,
		"cwd":          cwd,
	}), nil
}

type setWorkdirTool struct {
	r *pathResolver
}

func newSetWorkdirTool(r *pathResolver) *setWorkdirTool {
	return &setWorkdirTool{r: r}
}

func (t *setWorkdirTool) Name() string { return "set_workdir" }
func (t *setWorkdirTool) Description() string {
	return "Change the current working directory for this chat (like cd). Accepts relative paths from the current dir or any absolute path on this machine (e.g. D:\\\\repos\\\\app, E:\\\\cursor\\\\demo). After switching, file/command tools and /cursor use this directory."
}
func (t *setWorkdirTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Target directory: relative (\"my-app\", \"..\") or absolute (any drive/path)",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (t *setWorkdirTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	path := strings.TrimSpace(args.Path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if path == "/" || path == `\` {
		path = t.r.ws.Root
	}

	abs, err := t.r.Resolve(ctx, path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}

	sk := SessionKeyFrom(ctx)
	if t.r.store == nil || sk == "" {
		return "", fmt.Errorf("session workdir store unavailable")
	}
	if err := t.r.store.SetWorkdir(sk, abs); err != nil {
		return "", err
	}
	return mustJSON(map[string]any{
		"ok":  true,
		"cwd": abs,
	}), nil
}
