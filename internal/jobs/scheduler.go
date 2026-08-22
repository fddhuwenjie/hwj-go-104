package jobs

import (
	"context"
	"sync"
	"time"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/logx"
	"gowork/wafer/internal/repository"
)

// Handler 作业处理器。
type Handler func(ctx context.Context, payload string) error

// Scheduler 后台作业调度器：持久化作业、到期执行、失败重试、重启恢复。
type Scheduler struct {
	store       repository.Store
	clock       clock.Clock
	interval    time.Duration
	maxAttempts int

	mu       sync.Mutex
	handlers map[string]Handler
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewScheduler 创建调度器并注册内建作业处理器。
func NewScheduler(store repository.Store, clk clock.Clock, interval time.Duration, maxAttempts int, env Env) *Scheduler {
	s := &Scheduler{
		store:       store,
		clock:       clk,
		interval:    interval,
		maxAttempts: maxAttempts,
		handlers:    map[string]Handler{},
	}
	s.Register(domain.JobKindQualificationScan, QualificationScanHandler(store, clk))
	s.Register(domain.JobKindRunTimeout, RunTimeoutHandler(store, clk, env.RunTimeout))
	s.Register(domain.JobKindHoldEscalation, HoldEscalationHandler(store, clk, env.HoldEscalateAfter))
	s.Register(domain.JobKindRetry, RetryHandler(store, clk))
	return s
}

// Env 调度器环境参数。
type Env struct {
	RunTimeout        time.Duration
	HoldEscalateAfter time.Duration
}

// Register 注册作业处理器（测试可注入自定义处理器模拟失败重试）。
func (s *Scheduler) Register(kind string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[kind] = h
}

// Recover 重启恢复：把遗留 RUNNING 作业重置为 PENDING。
func (s *Scheduler) Recover(ctx context.Context) error {
	n, err := s.store.ResetRunningJobs(ctx, s.clock.Now())
	if err != nil {
		return err
	}
	if n > 0 {
		logx.L().Info("作业恢复：重置遗留运行中作业", "count", n)
	}
	return nil
}

// Start 启动后台循环：周期入队周期作业并执行到期作业。
func (s *Scheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.enqueuePeriodic(ctx)
				if err := s.RunOnce(ctx, 10); err != nil {
					logx.FromContext(ctx).Error("作业执行失败", "err", err)
				}
			}
		}
	}()
}

// Stop 停止后台循环。
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// enqueuePeriodic 周期入队扫描类作业（幂等：已有未完成作业时跳过）。
func (s *Scheduler) enqueuePeriodic(ctx context.Context) {
	now := s.clock.Now()
	for _, kind := range []string{domain.JobKindQualificationScan, domain.JobKindRunTimeout, domain.JobKindHoldEscalation, domain.JobKindRetry} {
		j := &domain.Job{
			ID:          domain.NewID(domain.IDPrefixJob),
			Kind:        kind,
			Status:      domain.JobPending,
			MaxAttempts: s.maxAttempts,
			RunAt:       now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.store.EnqueueIfAbsent(ctx, j); err != nil {
			logx.FromContext(ctx).Error("周期作业入队失败", "kind", kind, "err", err)
		}
	}
}

// RunOnce 执行一轮到期作业：抢占 -> 处理 -> 完成/失败记录。
func (s *Scheduler) RunOnce(ctx context.Context, limit int) error {
	due, err := s.store.DueJobs(ctx, s.clock.Now(), limit)
	if err != nil {
		return err
	}
	for _, j := range due {
		if err := s.runJob(ctx, j); err != nil {
			logx.FromContext(ctx).Error("作业处理失败", "job", j.ID, "kind", j.Kind, "err", err)
		}
	}
	return nil
}

func (s *Scheduler) runJob(ctx context.Context, j domain.Job) error {
	if err := s.store.ClaimJob(ctx, j.ID, s.clock.Now()); err != nil {
		return nil // 被其它执行者抢占，忽略
	}
	s.mu.Lock()
	h, ok := s.handlers[j.Kind]
	s.mu.Unlock()
	if !ok {
		return s.store.FailJob(ctx, j.ID, "未注册的作业类型: "+j.Kind, s.clock.Now())
	}
	if err := h(ctx, j.Payload); err != nil {
		return s.store.FailJob(ctx, j.ID, err.Error(), s.clock.Now())
	}
	return s.store.CompleteJob(ctx, j.ID, s.clock.Now())
}
