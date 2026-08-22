package jobs

import (
	"context"
	"fmt"
	"time"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
)

// RunTimeoutHandler 超时运行检查：
// 运行超过阈值仍未完工的运行置为 ABORTED，生成未关闭暂扣阻断批次，
// 批次置 ON_HOLD，全部在同一事务内完成。
func RunTimeoutHandler(store repository.Store, clk clock.Clock, timeout time.Duration) Handler {
	return func(ctx context.Context, payload string) error {
		now := clk.Now()
		running, err := store.RunningRuns(ctx)
		if err != nil {
			return fmt.Errorf("超时运行读取失败: %w", err)
		}
		return store.InTx(ctx, func(tx context.Context) error {
			for _, r := range running {
				if now.Sub(r.StartedAt) < timeout {
					continue
				}
				run := r
				run.Status = domain.RunAborted
				if err := store.UpdateRun(tx, &run, run.Version); err != nil {
					return err
				}
				lot, err := store.GetLot(tx, r.LotID)
				if err != nil {
					return err
				}
				hold := &domain.Hold{
					ID:        domain.NewID(domain.IDPrefixHold),
					LotID:     lot.ID,
					RunID:     r.ID,
					Reason:    "运行超时自动暂扣",
					Status:    domain.HoldOpen,
					CreatedAt: now,
				}
				if err := store.CreateHold(tx, hold); err != nil {
					return err
				}
				lot.Status = domain.LotOnHold
				if err := store.UpdateLot(tx, lot, lot.Version); err != nil {
					return err
				}
				txTag := domain.NewID("tx")
				if err := store.CreateAudit(tx, &domain.AuditEvent{
					ID:        domain.NewID(domain.IDPrefixAudit),
					Entity:    "run",
					EntityID:  r.ID,
					Action:    domain.AuditJobRunTimeout,
					Detail:    fmt.Sprintf(`{"hold_id":%q,"started_at":%q}`, hold.ID, r.StartedAt),
					TxTag:     txTag,
					CreatedAt: now,
				}); err != nil {
					return err
				}
			}
			return nil
		})
	}
}
