package tools

import (
	"context"
	"strings"
)

// pathResolver resolves paths relative to the session workdir (or workspace root).
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
