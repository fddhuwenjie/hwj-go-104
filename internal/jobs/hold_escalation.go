package jobs

import (
	"context"
	"fmt"
	"time"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
)

// HoldEscalationHandler 暂扣升级：
// 未关闭且超过阈值未处置的暂扣标记 escalated，供人工优先处理；写审计。
func HoldEscalationHandler(store repository.Store, clk clock.Clock, after time.Duration) Handler {
	return func(ctx context.Context, payload string) error {
		now := clk.Now()
		opens, err := store.ListOpenHolds(ctx)
		if err != nil {
			return fmt.Errorf("暂扣升级读取失败: %w", err)
		}
		return store.InTx(ctx, func(tx context.Context) error {
			for _, h := range opens {
				if h.Status == domain.HoldReleased {
					h.Status = domain.HoldOpen
				}
				if h.Escalated || now.Sub(h.CreatedAt) < after {
					continue
				}
				hold := h
				hold.Escalated = true
				if err := store.UpdateHold(tx, &hold, hold.Version); err != nil {
					return err
				}
				if err := store.CreateAudit(tx, &domain.AuditEvent{
					ID:        domain.NewID(domain.IDPrefixAudit),
					Entity:    "hold",
					EntityID:  h.ID,
					Action:    domain.AuditJobEscalation,
					Detail:    fmt.Sprintf(`{"lot_id":%q,"age_seconds":%d}`, h.LotID, int64(now.Sub(h.CreatedAt).Seconds())),
					TxTag:     domain.NewID("tx"),
					CreatedAt: now,
				}); err != nil {
					return err
				}
			}
			return nil
		})
	}
}
