package service

import (
	"context"
	"fmt"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/rules"
)

// RunService 制程运行服务：开工（资格校核）与完工（站点推进）。
type RunService struct {
	d Deps
}

// CreateRun 开工：校核暂扣阻断、站点排队状态、设备与腔体能力、
// 设备资质开工窗口、配方设备族兼容、晶圆占用，随后创建运行并置批次 RUNNING（幂等）。
func (s *RunService) CreateRun(ctx context.Context, lotID, equipmentID, chamberID string, waferIDs []string, idemKey string) (*domain.Run, bool, error) {
	return idempotent(ctx, s.d, "run.create:"+lotID, idemKey, func(tx context.Context) (*domain.Run, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		if lot.Status != domain.LotQueued {
			return nil, fmt.Errorf("%w: 批次状态 %s 不可开工", domain.ErrInvalidState, lot.Status)
		}
		if err := holdBlockingCheck(tx, s.d, lot.ID); err != nil {
			return nil, err
		}
		snap, err := loadFreeze(lot)
		if err != nil {
			return nil, err
		}
		fs := snap.StationAt(lot.CurrentSeq)
		if fs == nil {
			return nil, domain.ErrStationSkip
		}
		// 同一冻结修订的同站点不可重复开工（返工换版后以新修订重入）。
		exists, err := s.d.Store.HasRunAtStation(tx, lot.ID, lot.FrozenRevisionID, lot.CurrentSeq)
		if err != nil {
			return nil, err
		}
		if exists {
			// 允许同一修订下已中止运行的重开由状态机保证；此处防御重复运行。
			return nil, fmt.Errorf("%w: 当前站点已存在运行", domain.ErrInvalidState)
		}
		eq, err := s.d.Store.GetEquipment(tx, equipmentID)
		if err != nil {
			return nil, err
		}
		ch, err := s.d.Store.GetChamber(tx, chamberID)
		if err != nil {
			return nil, err
		}
		st, err := s.d.Store.GetStation(tx, fs.StationID)
		if err != nil {
			return nil, err
		}
		if err := rules.CheckCapability(*eq, *ch, *st); err != nil {
			return nil, err
		}
		recipe, err := s.d.Store.GetRecipe(tx, fs.RecipeID)
		if err != nil {
			return nil, err
		}
		if err := rules.CheckRecipeFamily(*recipe, *eq); err != nil {
			return nil, err
		}
		now := s.d.Clock.Now()
		quals, err := s.d.Store.QualificationsFor(tx, eq.ID, st.ID)
		if err != nil {
			return nil, err
		}
		if err := rules.CheckStartQualification(quals, *eq, ch.ID, st.ID, now); err != nil {
			return nil, err
		}
		// 晶圆：默认批次全部 ACTIVE 晶圆。
		lotWafers, err := s.d.Store.ListWafers(tx, lot.ID)
		if err != nil {
			return nil, err
		}
		if len(waferIDs) == 0 {
			for _, w := range lotWafers {
				if w.Status == domain.WaferActive {
					waferIDs = append(waferIDs, w.ID)
				}
			}
		} else {
			owned := map[string]bool{}
			for _, w := range lotWafers {
				owned[w.ID] = true
			}
			for _, id := range waferIDs {
				if !owned[id] {
					return nil, fmt.Errorf("%w: 晶圆 %s 不属于批次", domain.ErrValidation, id)
				}
			}
		}
		if len(waferIDs) == 0 {
			return nil, fmt.Errorf("%w: 批次无可用晶圆", domain.ErrValidation)
		}
		busy, err := s.d.Store.BusyWaferIDs(tx)
		if err != nil {
			return nil, err
		}
		if err := rules.CheckWaferAvailable(busy, waferIDs); err != nil {
			return nil, err
		}
		run := &domain.Run{
			ID:              domain.NewID(domain.IDPrefixRun),
			LotID:           lot.ID,
			RouteRevisionID: lot.FrozenRevisionID,
			StationSeq:      lot.CurrentSeq,
			StationID:       st.ID,
			EquipmentID:     eq.ID,
			ChamberID:       ch.ID,
			RecipeVersionID: fs.RecipeVersionID,
			RecipeSnapshot:  fs.RecipeSnapshot,
			Status:          domain.RunRunning,
			Judgment:        domain.JudgeNone,
			QualCovered:     true,
			StartedAt:       now,
			CreatedAt:       now,
		}
		if err := run.Validate(); err != nil {
			return nil, err
		}
		if err := s.d.Store.CreateRun(tx, run, waferIDs); err != nil {
			return nil, err
		}
		lot.Status = domain.LotRunning
		lot.EnteredAt = nil
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "run", run.ID, domain.AuditRunCreate, run); err != nil {
			return nil, err
		}
		return run, nil
	})
}

// GetRun 查询运行。
func (s *RunService) GetRun(ctx context.Context, id string) (*domain.Run, error) {
	return s.d.Store.GetRun(ctx, id)
}

// ListRunsByLot 列出批次运行。
func (s *RunService) ListRunsByLot(ctx context.Context, lotID string) ([]domain.Run, error) {
	return s.d.Store.ListRunsByLot(ctx, lotID)
}

// CompleteRun 完工：复核资质完整运行区间覆盖，推进批次到站点间等待（幂等）。
// 资质未覆盖不阻断完工，但标记 qual_covered=false 供后续复判。
func (s *RunService) CompleteRun(ctx context.Context, runID string, idemKey string) (*domain.Run, bool, error) {
	return idempotent(ctx, s.d, "run.complete:"+runID, idemKey, func(tx context.Context) (*domain.Run, error) {
		run, err := s.d.Store.GetRun(tx, runID)
		if err != nil {
			return nil, err
		}
		if !domain.CanRunTransition(run.Status, domain.RunCompleted) {
			return nil, fmt.Errorf("%w: 运行状态 %s 不可完工", domain.ErrInvalidState, run.Status)
		}
		lot, err := s.d.Store.GetLot(tx, run.LotID)
		if err != nil {
			return nil, err
		}
		if lot.Status != domain.LotRunning {
			return nil, fmt.Errorf("%w: 批次状态 %s 不可完工推进", domain.ErrInvalidState, lot.Status)
		}
		now := s.d.Clock.Now()
		eq, err := s.d.Store.GetEquipment(tx, run.EquipmentID)
		if err != nil {
			return nil, err
		}
		quals, err := s.d.Store.QualificationsFor(tx, run.EquipmentID, run.StationID)
		if err != nil {
			return nil, err
		}
		run.QualCovered = rules.CoverageAtCompletion(quals, *eq, run.ChamberID, run.StationID, run.StartedAt, now)
		run.Status = domain.RunCompleted
		run.CompletedAt = &now
		if err := s.d.Store.UpdateRun(tx, run, run.Version); err != nil {
			return nil, err
		}
		run.Version++
		lot.Status = domain.LotWaiting
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "run", run.ID, domain.AuditRunComplete,
			map[string]any{"qual_covered": run.QualCovered}); err != nil {
			return nil, err
		}
		return run, nil
	})
}
