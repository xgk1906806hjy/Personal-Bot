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
	ws      *Workspace
	maxBytes int
}

func newReadFileTool(ws *Workspace, maxBytes int) *readFileTool {
	return &readFileTool{ws: ws, maxBytes: maxBytes}
}

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Description() string {
	return "Read a UTF-8 text file under the tool workspace. Paths are relative to the workspace root."
}
func (t *readFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path inside the workspace",
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

func (t *readFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path     string `json:"path"`
		MaxBytes int    `json:"max_bytes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	abs, err := t.ws.Resolve(args.Path)
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
	ws *Workspace
}

func newWriteFileTool(ws *Workspace) *writeFileTool {
	return &writeFileTool{ws: ws}
}

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Description() string {
	return "Create or overwrite a text file under the tool workspace. Parent directories are created as needed."
}
func (t *writeFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Relative path inside the workspace",
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

func (t *writeFileTool) Execute(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	abs, err := t.ws.Resolve(args.Path)
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
		"ok":   true,
		"path": abs,
		"bytes": len(args.Content),
	}), nil
}

type listDirTool struct {
	ws *Workspace
}

func newListDirTool(ws *Workspace) *listDirTool {
	return &listDirTool{ws: ws}
}

func (t *listDirTool) Name() string { return "list_dir" }
func (t *listDirTool) Description() string {
	return "List files and directories under a workspace path."
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

func (t *listDirTool) Execute(_ context.Context, argsJSON string) (string, error) {
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
	abs, err := t.ws.Resolve(args.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		suffix := ""
		if e.IsDir() {
			suffix = "/"
		}
		lines = append(lines, e.Name()+suffix)
	}
	return strings.Join(lines, "\n"), nil
}
