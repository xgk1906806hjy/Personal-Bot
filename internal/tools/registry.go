package tools

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Options struct {
	Workspace         string
	CommandTimeout    time.Duration
	MaxOutputBytes    int
	EnableShell       bool
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	ws    *Workspace
	opts  Options
}

func NewRegistry(opts Options) (*Registry, error) {
	if opts.CommandTimeout <= 0 {
		opts.CommandTimeout = 60 * time.Second
	}
	if opts.MaxOutputBytes <= 0 {
		opts.MaxOutputBytes = 100_000
	}
	ws, err := NewWorkspace(opts.Workspace)
	if err != nil {
		return nil, err
	}
	r := &Registry{
		tools: make(map[string]Tool),
		ws:    ws,
		opts:  opts,
	}
	r.Register(newReadFileTool(ws, opts.MaxOutputBytes))
	r.Register(newWriteFileTool(ws))
	r.Register(newListDirTool(ws))
	if opts.EnableShell {
		r.Register(newRunCommandTool(ws, opts.CommandTimeout, opts.MaxOutputBytes))
	}
	return r, nil
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Tool, 0, len(names))
	for _, n := range names {
		out = append(out, r.tools[n])
	}
	return out
}

func (r *Registry) OpenAITools() []OpenAITool {
	return ToOpenAITools(r.List())
}

func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, argsJSON)
}

func (r *Registry) WorkspaceRoot() string {
	return r.ws.Root
}

func (r *Registry) ToolNames() []string {
	list := r.List()
	names := make([]string, 0, len(list))
	for _, t := range list {
		names = append(names, t.Name())
	}
	return names
}
