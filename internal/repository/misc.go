package repository

import (
	"context"
	"time"

	"gowork/wafer/internal/domain"
)

// ReleaseRepo 放行记录仓储。
type ReleaseRepo interface {
	CreateRelease(ctx context.Context, r *domain.Release) error
	ListReleases(ctx context.Context, lotID string) ([]domain.Release, error)
}

// AuditRepo 审计事件仓储。
type AuditRepo interface {
	CreateAudit(ctx context.Context, e *domain.AuditEvent) error
	ListAudit(ctx context.Context, entity, entityID string) ([]domain.AuditEvent, error)
}

// IdempotencyRepo 业务幂等键仓储。
type IdempotencyRepo interface {
	// GetIdempotency 返回已存储的幂等响应，未命中返回 ErrNotFound。
	GetIdempotency(ctx context.Context, scope, key string) (string, error)
	PutIdempotency(ctx context.Context, scope, key, response string, at time.Time) error
}

// JobRepo 后台作业仓储。
type JobRepo interface {
	CreateJob(ctx context.Context, j *domain.Job) error
	GetJob(ctx context.Context, id string) (*domain.Job, error)
	// DueJobs 返回到期可执行作业。
	DueJobs(ctx context.Context, now time.Time, limit int) ([]domain.Job, error)
	// ClaimJob 抢占作业（PENDING -> RUNNING），失败返回 ErrConflict。
	ClaimJob(ctx context.Context, id string, now time.Time) error
	CompleteJob(ctx context.Context, id string, now time.Time) error
	// FailJob 记录失败；未超过最大次数置 FAILED 以便重试，否则 DEAD。
	FailJob(ctx context.Context, id, errMsg string, now time.Time) error
	// RetryableJobs 返回可重试的失败作业。
	RetryableJobs(ctx context.Context) ([]domain.Job, error)
	// RequeueJob 把失败作业重新排队为 PENDING。
	RequeueJob(ctx context.Context, id string, now time.Time) error
	// ResetRunningJobs 重启恢复：把遗留 RUNNING 作业重置为 PENDING。
	ResetRunningJobs(ctx context.Context, now time.Time) (int, error)
	// EnqueueIfAbsent 幂等入队：同 kind+payload 的未完成作业不重复创建。
	EnqueueIfAbsent(ctx context.Context, j *domain.Job) error
}
