package tools

import "context"

type ctxKey int

const sessionKeyCtx ctxKey = 1

// WorkdirStore 保存每个会话的当前工作目录（由 session.Store 实现）。
type WorkdirStore interface {
	GetWorkdir(sessionKey string) string
	SetWorkdir(sessionKey, absPath string) error
}

// WithSessionKey 把会话键放入 context，供路径解析与 set_workdir 使用。
func WithSessionKey(ctx context.Context, sessionKey string) context.Context {
	return context.WithValue(ctx, sessionKeyCtx, sessionKey)
}

// SessionKeyFrom 从 context 取出会话键；缺失时返回空串。
func SessionKeyFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(sessionKeyCtx).(string)
	return v
}
