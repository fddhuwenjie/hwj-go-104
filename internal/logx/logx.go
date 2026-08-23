package logx

import (
	"context"
	"log/slog"
	"os"
)

// ctxKey 日志上下文字段键。
type ctxKey struct{}

var base *slog.Logger

func init() {
	base = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(base)
}

// L 返回全局结构化日志器。
func L() *slog.Logger { return base }

// WithRequest 把请求级字段放入上下文。
func WithRequest(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, requestID)
}

// FromContext 提取带请求字段的日志器。
func FromContext(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(ctxKey{}).(string); ok && v != "" {
		return base.With("request_id", v)
	}
	return base
}
