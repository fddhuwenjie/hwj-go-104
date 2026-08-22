package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"gowork/wafer/internal/logx"
)

// statusWriter 记录响应状态码。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware 结构化访问日志。
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			var b [8]byte
			_, _ = rand.Read(b[:])
			reqID = hex.EncodeToString(b[:])
		}
		ctx := logx.WithRequest(r.Context(), reqID)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))
		logx.FromContext(ctx).Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// recoverMiddleware panic 恢复，保证统一 JSON 错误。
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				logx.FromContext(r.Context()).Error("panic", "err", p)
				writeJSON(w, http.StatusInternalServerError, errorBody{
					Error: "内部错误",
					Code:  "INTERNAL",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
