package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

type runCommandTool struct {
	ws       *Workspace
	timeout  time.Duration
	maxBytes int
}

func newRunCommandTool(ws *Workspace, timeout time.Duration, maxBytes int) *runCommandTool {
	return &runCommandTool{ws: ws, timeout: timeout, maxBytes: maxBytes}
}

func (t *runCommandTool) Name() string { return "run_command" }
func (t *runCommandTool) Description() string {
	return "Run a shell command with cwd set to the tool workspace. Prefer non-interactive commands. Output is truncated."
}
func (t *runCommandTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command string to execute",
			},
		},
		"required":             []string{"command"},
		"additionalProperties": false,
	}
}

func (t *runCommandTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if args.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", args.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", args.Command)
	}
	cmd.Dir = t.ws.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	out := truncateBytes(stdout.Bytes(), t.maxBytes)
	errOut := truncateBytes(stderr.Bytes(), t.maxBytes/4)
	result := map[string]any{
		"exit_code": 0,
		"stdout":    string(out),
		"stderr":    string(errOut),
		"cwd":       t.ws.Root,
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			result["exit_code"] = -1
			result["error"] = "command timed out"
		} else {
			result["exit_code"] = -1
			result["error"] = err.Error()
		}
	}
	return mustJSON(result), nil
}

func truncateBytes(b []byte, max int) []byte {
	if max <= 0 || len(b) <= max {
		return b
	}
	out := make([]byte, max)
	copy(out, b[:max])
	return append(out, []byte("\n…(truncated)")...)
}
