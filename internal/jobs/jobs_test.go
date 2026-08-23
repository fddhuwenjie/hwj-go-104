package jobs_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/jobs"
	"gowork/wafer/internal/sqlite"
)

var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newStore(t *testing.T) (*sqlite.Store, string) {
	t.Helper()
	db := filepath.Join(t.TempDir(), "jobs.db")
	s, err := sqlite.Open(db)
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, db
}

// TestFailedJobRetry 失败作业重试：首次失败 -> FAILED，
// 重试作业重新排队 -> 再次执行成功 -> DONE。
func TestFailedJobRetry(t *testing.T) {
	store, _ := newStore(t)
	clk := clock.NewManual(baseTime)
	ctx := context.Background()

	s := jobs.NewScheduler(store, clk, time.Minute, 3, jobs.Env{})
	var calls atomic.Int32
	s.Register("FLAKY", func(ctx context.Context, payload string) error {
		if calls.Add(1) == 1 {
			return errors.New("模拟瞬时故障")
		}
		return nil
	})

	now := clk.Now()
	job := &domain.Job{
		ID:          domain.NewID(domain.IDPrefixJob),
		Kind:        "FLAKY",
		Status:      domain.JobPending,
		MaxAttempts: 3,
		RunAt:       now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("创建作业: %v", err)
	}

	// 第一轮：失败 -> FAILED。
	if err := s.RunOnce(ctx, 10); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	after, _ := store.GetJob(ctx, job.ID)
	if after.Status != domain.JobFailed || after.Attempts != 1 {
		t.Fatalf("失败后状态错误: %+v", after)
	}

	// 重试作业重新排队。
	retry := jobs.RetryHandler(store, clk)
	if err := retry(ctx, ""); err != nil {
		t.Fatalf("重试处理器: %v", err)
	}
	requeued, _ := store.GetJob(ctx, job.ID)
	if requeued.Status != domain.JobPending {
		t.Fatalf("重试排队失败: %s", requeued.Status)
	}

	// 第二轮：成功 -> DONE。
	if err := s.RunOnce(ctx, 10); err != nil {
		t.Fatalf("RunOnce2: %v", err)
	}
	done, _ := store.GetJob(ctx, job.ID)
	if done.Status != domain.JobDone || done.Attempts != 2 {
		t.Fatalf("重试后状态错误: %+v", done)
	}
	if calls.Load() != 2 {
		t.Fatalf("处理器调用次数错误: %d", calls.Load())
	}
}

// TestJobDeadAfterMaxAttempts 超过最大重试次数进入 DEAD。
func TestJobDeadAfterMaxAttempts(t *testing.T) {
	store, _ := newStore(t)
	clk := clock.NewManual(baseTime)
	ctx := context.Background()

	s := jobs.NewScheduler(store, clk, time.Minute, 2, jobs.Env{})
	s.Register("ALWAYS-FAIL", func(ctx context.Context, payload string) error {
		return errors.New("永久故障")
	})
	now := clk.Now()
	job := &domain.Job{
		ID:          domain.NewID(domain.IDPrefixJob),
		Kind:        "ALWAYS-FAIL",
		Status:      domain.JobPending,
		MaxAttempts: 2,
		RunAt:       now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("创建作业: %v", err)
	}
	retry := jobs.RetryHandler(store, clk)
	for i := 0; i < 2; i++ {
		if err := s.RunOnce(ctx, 10); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		if err := retry(ctx, ""); err != nil {
			t.Fatalf("重试: %v", err)
		}
	}
	final, _ := store.GetJob(ctx, job.ID)
	if final.Status != domain.JobDead {
		t.Fatalf("应为 DEAD: %+v", final)
	}
}

// TestQualificationScanJob 资质到期扫描：过期资质置 REVOKED，且可重复执行。
func TestQualificationScanJob(t *testing.T) {
	store, _ := newStore(t)
	clk := clock.NewManual(baseTime)
	ctx := context.Background()

	// 直接写入设备与站点主数据的最小集合（绕过服务层，验证作业本身）。
	now := clk.Now()
	mk := func() {
		_ = now
	}
	mk()
	pf := &domain.ProductFamily{ID: domain.NewID(domain.IDPrefixProductFamily), Code: "PF", Name: "x", CreatedAt: now}
	if err := store.CreateProductFamily(ctx, pf); err != nil {
		t.Fatalf("产品族: %v", err)
	}
	st := &domain.Station{ID: domain.NewID(domain.IDPrefixStation), Code: "ST", Name: "x", Capability: "etch", CreatedAt: now}
	if err := store.CreateStation(ctx, st); err != nil {
		t.Fatalf("站点: %v", err)
	}
	eq := &domain.Equipment{ID: domain.NewID(domain.IDPrefixEquipment), Code: "EQ", Name: "x", Family: "F", StationID: st.ID, Status: domain.EquipActive, CreatedAt: now}
	if err := store.CreateEquipment(ctx, eq); err != nil {
		t.Fatalf("设备: %v", err)
	}
	q := &domain.Qualification{
		ID:          domain.NewID(domain.IDPrefixQualification),
		EquipmentID: eq.ID,
		StationID:   st.ID,
		ValidFrom:   now.Add(-2 * time.Hour),
		ValidTo:     now.Add(-time.Hour), // 已过期
		Status:      domain.QualActive,
		CreatedAt:   now,
	}
	if err := store.CreateQualification(ctx, q); err != nil {
		t.Fatalf("资质: %v", err)
	}

	h := jobs.QualificationScanHandler(store, clk)
	if err := h(ctx, ""); err != nil {
		t.Fatalf("扫描: %v", err)
	}
	after, _ := store.GetQualification(ctx, q.ID)
	if after.Status != domain.QualRevoked {
		t.Fatalf("过期资质应 REVOKED: %s", after.Status)
	}
	// 幂等：重复执行无副作用。
	if err := h(ctx, ""); err != nil {
		t.Fatalf("重复扫描: %v", err)
	}
}

// TestRestartRecovery 关闭后使用同一数据库文件恢复：
// 遗留 RUNNING 作业重置为 PENDING 并可继续执行。
func TestRestartRecovery(t *testing.T) {
	store, db := newStore(t)
	clk := clock.NewManual(baseTime)
	ctx := context.Background()

	now := clk.Now()
	running := &domain.Job{
		ID:          domain.NewID(domain.IDPrefixJob),
		Kind:        domain.JobKindQualificationScan,
		Status:      domain.JobRunning, // 模拟崩溃时遗留
		Attempts:    1,
		MaxAttempts: 3,
		RunAt:       now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.CreateJob(ctx, running); err != nil {
		t.Fatalf("创建作业: %v", err)
	}
	store.Close()

	// 重新打开同一文件。
	store2, err := sqlite.Open(db)
	if err != nil {
		t.Fatalf("重开数据库: %v", err)
	}
	defer store2.Close()
	s := jobs.NewScheduler(store2, clk, time.Minute, 3, jobs.Env{})
	if err := s.Recover(ctx); err != nil {
		t.Fatalf("恢复: %v", err)
	}
	after, _ := store2.GetJob(ctx, running.ID)
	if after.Status != domain.JobPending {
		t.Fatalf("遗留作业应重置为 PENDING: %s", after.Status)
	}
	// 恢复后可正常执行。
	if err := s.RunOnce(ctx, 10); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	done, _ := store2.GetJob(ctx, running.ID)
	if done.Status != domain.JobDone {
		t.Fatalf("恢复后执行失败: %s", done.Status)
	}
}
