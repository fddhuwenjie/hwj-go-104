package jobs

import (
	"context"
	"fmt"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
	"gowork/wafer/internal/rules"
)

// QualificationScanHandler 资质到期扫描：
// 把已过期但仍为 ACTIVE 的资质置为 REVOKED，并写入审计；可重复执行（幂等）。
func QualificationScanHandler(store repository.Store, clk clock.Clock) Handler {
	return func(ctx context.Context, payload string) error {
		now := clk.Now()
		quals, err := store.ListQualifications(ctx)
		if err != nil {
			return fmt.Errorf("资质扫描读取失败: %w", err)
		}
		expired := rules.FindExpired(quals, now)
		return store.InTx(ctx, func(tx context.Context) error {
			for _, q := range expired {
				if err := store.UpdateQualificationStatus(tx, q.ID, domain.QualRevoked); err != nil {
					return err
				}
				if err := store.CreateAudit(tx, &domain.AuditEvent{
					ID:        domain.NewID(domain.IDPrefixAudit),
					Entity:    "qualification",
					EntityID:  q.ID,
					Action:    domain.AuditJobQualification,
					Detail:    fmt.Sprintf(`{"equipment_id":%q,"valid_to":%q}`, q.EquipmentID, q.ValidTo),
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
