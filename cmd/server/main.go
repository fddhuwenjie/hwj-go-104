// 晶圆制程批次与配方版本隔离服务入口。
// 读取 PORT 与 DB_PATH，提供 /healthz、结构化日志与优雅关闭。
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/config"
	"gowork/wafer/internal/httpx"
	"gowork/wafer/internal/jobs"
	"gowork/wafer/internal/logx"
	"gowork/wafer/internal/service"
	"gowork/wafer/internal/sqlite"
)

func main() {
	if err := run(); err != nil {
		logx.L().Error("服务退出", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	clk := clock.Real{}
	deps := service.Deps{Store: store, Clock: clk}
	svc := service.NewServices(deps)

	// 后台作业：重启恢复 + 周期调度。
	scheduler := jobs.NewScheduler(store, clk, cfg.JobInterval, cfg.JobMaxAttempts, jobs.Env{
		RunTimeout:        cfg.RunTimeout,
		HoldEscalateAfter: cfg.HoldEscalateAfter,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := scheduler.Recover(ctx); err != nil {
		return err
	}
	scheduler.Start(ctx)
	defer scheduler.Stop()

	health := func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}

	server := httpx.NewServer(cfg, httpx.NewRouter(svc, health))
	errCh := make(chan error, 1)
	go func() { errCh <- server.Start() }()

	select {
	case <-ctx.Done():
		logx.L().Info("收到退出信号，优雅关闭")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
