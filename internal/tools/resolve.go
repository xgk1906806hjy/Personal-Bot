package tools

import (
	"context"
	"strings"
)

// pathResolver 相对「会话 workdir（或默认根）」解析文件路径。
type pathResolver struct {
	ws    *Workspace
	store WorkdirStore
}

func (r *pathResolver) cwd(ctx context.Context) string {
	if r.store != nil {
		if sk := SessionKeyFrom(ctx); sk != "" {
			if w := strings.TrimSpace(r.store.GetWorkdir(sk)); w != "" {
				return w
			}
		}
	}
	return r.ws.Root
}

func (r *pathResolver) Resolve(ctx context.Context, rel string) (string, error) {
	return r.ws.ResolveFrom(r.cwd(ctx), rel)
}
