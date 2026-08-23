package jobs

import (
	"context"
	"fmt"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
)

// RetryHandler 失败作业重试：把可重试的失败作业重新置为 PENDING 立即到期，
// 由调度器下一轮重新执行；重试动作写入审计。
func RetryHandler(store repository.Store, clk clock.Clock) Handler {
	return func(ctx context.Context, payload string) error {
		now := clk.Now()
		failed, err := store.RetryableJobs(ctx)
		if err != nil {
			return fmt.Errorf("失败作业读取失败: %w", err)
		}
		for _, j := range failed {
			fresh, err := store.GetJob(ctx, j.ID)
			if err != nil {
				return err
			}
			if !fresh.Retryable() {
				continue
			}
			if err := store.RequeueJob(ctx, j.ID, now); err != nil {
				return err
			}
			if err := store.CreateAudit(ctx, &domain.AuditEvent{
				ID:        domain.NewID(domain.IDPrefixAudit),
				Entity:    "job",
				EntityID:  j.ID,
				Action:    domain.AuditJobRetry,
				Detail:    fmt.Sprintf(`{"kind":%q,"attempts":%d}`, j.Kind, j.Attempts),
				TxTag:     domain.NewID("tx"),
				CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		return nil
	}
}
