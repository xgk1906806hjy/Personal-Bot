package tools

import "context"

type ctxKey int

const sessionKeyCtx ctxKey = 1

// WorkdirStore holds per-chat working directories under the tool workspace root.
type WorkdirStore interface {
	GetWorkdir(sessionKey string) string
	SetWorkdir(sessionKey, absPath string) error
}

func WithSessionKey(ctx context.Context, sessionKey string) context.Context {
	return context.WithValue(ctx, sessionKeyCtx, sessionKey)
}

func SessionKeyFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(sessionKeyCtx).(string)
	return v
}
