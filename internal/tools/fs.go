package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type readFileTool struct {
	r        *pathResolver
	maxBytes int
}

func newReadFileTool(r *pathResolver, maxBytes int) *readFileTool {
	return &readFileTool{r: r, maxBytes: maxBytes}
}

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Description() string {
	return "Read a UTF-8 text file. Paths are relative to the current workdir (see get_workdir / set_workdir), confined under the tool root."
}
func (t *readFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to current workdir (or absolute under root)",
			},
			"max_bytes": map[string]any{
				"type":        "integer",
				"description": "Optional max bytes to read (defaults to tool limit)",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (t *readFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path     string `json:"path"`
		MaxBytes int    `json:"max_bytes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	abs, err := t.r.Resolve(ctx, args.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	limit := t.maxBytes
	if args.MaxBytes > 0 && args.MaxBytes < limit {
		limit = args.MaxBytes
	}
	truncated := false
	if len(data) > limit {
		data = data[:limit]
		truncated = true
	}
	out := string(data)
	if truncated {
		out += "\n…(truncated)"
	}
	return out, nil
}

type writeFileTool struct {
	r *pathResolver
}

func newWriteFileTool(r *pathResolver) *writeFileTool {
	return &writeFileTool{r: r}
}

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Description() string {
	return "Create or overwrite a text file under the current workdir (confined to tool root). Parent directories are created as needed."
}
func (t *writeFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to current workdir",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Full file contents to write",
			},
		},
		"required":             []string{"path", "content"},
		"additionalProperties": false,
	}
}

func (t *writeFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	abs, err := t.r.Resolve(ctx, args.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, []byte(args.Content), 0o644); err != nil {
		return "", err
	}
	return mustJSON(map[string]any{
		"ok":    true,
		"path":  abs,
		"bytes": len(args.Content),
	}), nil
}

type listDirTool struct {
	r *pathResolver
}

func newListDirTool(r *pathResolver) *listDirTool {
	return &listDirTool{r: r}
}

func (t *listDirTool) Name() string { return "list_dir" }
func (t *listDirTool) Description() string {
	return "List files and directories. Default path \".\" is the current workdir. Use this to discover projects under the Cursor root, then set_workdir to enter one."
}
func (t *listDirTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative directory path (default \".\")",
			},
		},
		"additionalProperties": false,
	}
}

func (t *listDirTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
	}
	if args.Path == "" {
		args.Path = "."
	}
	abs, err := t.r.Resolve(ctx, args.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "cwd="+abs)
	for _, e := range entries {
		suffix := ""
		if e.IsDir() {
			suffix = "/"
		}
		lines = append(lines, e.Name()+suffix)
	}
	return strings.Join(lines, "\n"), nil
}
