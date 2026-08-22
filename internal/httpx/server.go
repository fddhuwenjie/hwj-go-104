package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gowork/wafer/internal/config"
	"gowork/wafer/internal/logx"
)

// Server HTTP 服务器：支持优雅关闭。
type Server struct {
	http *http.Server
}

// NewServer 构建服务器。
func NewServer(cfg config.Config, handler http.Handler) *Server {
	return &Server{
		http: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Port),
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Start 启动服务，阻塞直到退出。
func (s *Server) Start() error {
	logx.L().Info("HTTP 服务启动", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown 优雅关闭。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
