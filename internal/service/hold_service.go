package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/rules"
)

// HoldService 暂扣与复判服务。
type HoldService struct {
	d Deps
}

// CreateHold 人工暂扣：置批次 ON_HOLD（幂等）。
func (s *HoldService) CreateHold(ctx context.Context, lotID, reason string, idemKey string) (*domain.Hold, bool, error) {
	return idempotent(ctx, s.d, "hold.create:"+lotID, idemKey, func(tx context.Context) (*domain.Hold, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		blocked := lot.Status == domain.LotClosed || lot.Status == domain.LotScrapped
		if lot.Status == domain.LotScrapped && domain.CanLotTransition(lot.Status, domain.LotOnHold) {
			blocked = false
		}
		if blocked {
			return nil, fmt.Errorf("%w: 批次当前状态不可暂扣", domain.ErrInvalidState)
		}
		hold := &domain.Hold{
			ID:        domain.NewID(domain.IDPrefixHold),
			LotID:     lot.ID,
			Reason:    reason,
			Status:    domain.HoldOpen,
			CreatedAt: s.d.Clock.Now(),
		}
		if err := hold.Validate(); err != nil {
			return nil, err
		}
		if err := s.d.Store.CreateHold(tx, hold); err != nil {
			return nil, err
		}
		lot.Status = domain.LotOnHold
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "hold", hold.ID, domain.AuditHoldCreate, hold); err != nil {
			return nil, err
		}
		return hold, nil
	})
}

// GetHold 查询暂扣。
func (s *HoldService) GetHold(ctx context.Context, id string) (*domain.Hold, error) {
	return s.d.Store.GetHold(ctx, id)
}

// ReviewResult 复判结果。
type ReviewResult struct {
	Hold          *domain.Hold         `json:"hold"`
	Lot           *domain.Lot          `json:"lot"`
	Rework        *domain.ReworkRecord `json:"rework,omitempty"`
	NewRevisionID string               `json:"new_revision_id,omitempty"`
}

// Review 人工复判：关闭暂扣并按处置动作放行 / 返工换版 / 报废。
// 复判关闭与返工换版在同一事务内完成；旧运行、旧量测、旧暂扣与旧快照均不改写（幂等）。
func (s *HoldService) Review(ctx context.Context, holdID string, action domain.ReviewAction, note string, reentrySeq int, idemKey string) (*ReviewResult, bool, error) {
	return idempotent(ctx, s.d, "hold.review:"+holdID, idemKey, func(tx context.Context) (*ReviewResult, error) {
		hold, err := s.d.Store.GetHold(tx, holdID)
		if err != nil {
			return nil, err
		}
		if hold.Status != domain.HoldOpen {
			return nil, fmt.Errorf("%w: 暂扣已关闭，不可重复复判", domain.ErrInvalidState)
		}
		lot, err := s.d.Store.GetLot(tx, hold.LotID)
		if err != nil {
			return nil, err
		}
		now := s.d.Clock.Now()
		hold.ReviewNote = note
		hold.ClosedAt = &now
		res := &ReviewResult{Hold: hold}
		txTag := domain.NewID("tx")

		switch action {
		case domain.ReviewRelease:
			hold.Status = domain.HoldReleased
			if err := s.markRunReviewed(tx, hold); err != nil {
				return nil, err
			}
			// 恢复原路线：当前站点已有判定运行则进入站点间等待，否则重新排队。
			target := domain.LotQueued
			lot.EnteredAt = &now
			runs, err := s.d.Store.ListRunsByLot(tx, lot.ID)
			if err != nil {
				return nil, err
			}
			for _, r := range runs {
				if r.StationSeq == lot.CurrentSeq && r.RouteRevisionID == lot.FrozenRevisionID && r.Status == domain.RunJudged {
					target = domain.LotWaiting
					lot.EnteredAt = nil
				}
			}
			lot.Status = target
			if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
				return nil, err
			}
			lot.Version++
		case domain.ReviewRework:
			hold.Status = domain.HoldReworked
			if err := s.markRunReviewed(tx, hold); err != nil {
				return nil, err
			}
			rec, err := s.rework(tx, lot, hold, reentrySeq, now)
			if err != nil {
				return nil, err
			}
			res.Rework = rec
			res.NewRevisionID = rec.NewRevisionID
		case domain.ReviewScrap:
			hold.Status = domain.HoldScrapped
			if err := s.markRunReviewed(tx, hold); err != nil {
				return nil, err
			}
			if !domain.CanLotTransition(lot.Status, domain.LotScrapped) {
				return nil, fmt.Errorf("%w: 批次状态 %s 不可报废", domain.ErrInvalidState, lot.Status)
			}
			lot.Status = domain.LotScrapped
			lot.ClosedAt = &now
			if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
				return nil, err
			}
			lot.Version++
		default:
			return nil, domain.NewValidationError(domain.FieldError{Field: "action", Message: "非法复判动作"})
		}

		if err := s.d.Store.UpdateHold(tx, hold, hold.Version); err != nil {
			return nil, err
		}
		hold.Version++
		if err := audit(tx, s.d, txTag, "hold", hold.ID, domain.AuditHoldReview,
			map[string]any{"action": action, "note": note, "reentry_seq": reentrySeq}); err != nil {
			return nil, err
		}
		res.Lot = lot
		return res, nil
	})
}

// markRunReviewed 把暂扣关联运行标记为已复判。
func (s *HoldService) markRunReviewed(tx context.Context, hold *domain.Hold) error {
	if hold.RunID == "" {
		return nil
	}
	run, err := s.d.Store.GetRun(tx, hold.RunID)
	if err != nil {
		return err
	}
	run.Reviewed = true
	return s.d.Store.UpdateRun(tx, run, run.Version)
}

// rework 返工换版：基于冻结快照创建新路线修订并从指定站点重入。
// 批次重置到新修订的起点排队，旧修订、旧运行、旧量测、旧暂扣保持不可变。
func (s *HoldService) rework(tx context.Context, lot *domain.Lot, hold *domain.Hold, reentrySeq int, now time.Time) (*domain.ReworkRecord, error) {
	snap, err := loadFreeze(lot)
	if err != nil {
		return nil, err
	}
	if err := rules.CheckReworkReentry(snap, reentrySeq, lot.CurrentSeq); err != nil {
		return nil, err
	}
	newStations := rules.BuildReworkRevision(snap, reentrySeq)
	if len(newStations) == 0 {
		return nil, fmt.Errorf("%w: 返工修订无站点", domain.ErrInvalidState)
	}
	num, err := s.d.Store.NextRevisionNumber(tx, lot.RouteID)
	if err != nil {
		return nil, err
	}
	rev := &domain.RouteRevision{
		ID:               domain.NewID(domain.IDPrefixRouteRev),
		RouteID:          lot.RouteID,
		Revision:         num,
		Status:           domain.RevDraft,
		ReworkFromHoldID: hold.ID,
		ReentrySeq:       reentrySeq,
		CreatedAt:        now,
	}
	for i := range newStations {
		newStations[i].ID = domain.NewID(domain.IDPrefixRouteRev)
	}
	if err := s.d.Store.CreateRevision(tx, rev, newStations); err != nil {
		return nil, err
	}
	// 直接启用返工修订（站点与配方继承冻结快照，视为已校验）。
	if err := s.d.Store.UpdateRevisionStatus(tx, rev.ID, domain.RevActive, 1); err != nil {
		return nil, err
	}
	rev.Status = domain.RevActive
	// 基于新修订重建批次冻结快照：配方取当前启用版本（返工换版语义），
	// 量测计划沿用冻结快照中的计划快照。
	route, err := s.d.Store.GetRoute(tx, lot.RouteID)
	if err != nil {
		return nil, err
	}
	stationByID := map[string]domain.Station{}
	recipeByStation := map[string]domain.RecipeVersion{}
	planByID := map[string]domain.MetrologyPlan{}
	oldPlans := map[string]string{} // station_id -> plan snapshot
	for _, fs := range snap.Stations {
		oldPlans[fs.StationID] = fs.PlanSnapshot
	}
	for _, rs := range newStations {
		st, err := s.d.Store.GetStation(tx, rs.StationID)
		if err != nil {
			return nil, err
		}
		stationByID[rs.StationID] = *st
		rv, err := s.d.Store.ActiveVersion(tx, rs.RecipeID)
		if err != nil {
			return nil, fmt.Errorf("%w: 配方 %s 无启用版本", err, rs.RecipeID)
		}
		recipeByStation[rs.StationID] = *rv
		var plan domain.MetrologyPlan
		if raw, ok := oldPlans[rs.StationID]; ok {
			if err := json.Unmarshal([]byte(raw), &plan); err != nil {
				return nil, err
			}
		} else {
			p, err := s.d.Store.GetPlan(tx, rs.MetrologyPlanID)
			if err != nil {
				return nil, err
			}
			plan = *p
		}
		plan.Status = domain.PlanActive
		planByID[plan.ID] = plan
	}
	newSnap, err := rules.BuildFreezeSnapshot(*route, *rev, newStations, stationByID, recipeByStation, planByID)
	if err != nil {
		return nil, err
	}
	raw, err := newSnap.Encode()
	if err != nil {
		return nil, err
	}
	lot.FrozenRevisionID = rev.ID
	lot.FreezeSnapshot = raw
	lot.FrozenAt = &now
	lot.CurrentSeq = 0
	lot.Status = domain.LotRegistered
	lot.EnteredAt = nil
	if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
		return nil, err
	}
	lot.Version++
	rec := &domain.ReworkRecord{
		ID:            domain.NewID("rw"),
		LotID:         lot.ID,
		HoldID:        hold.ID,
		NewRevisionID: rev.ID,
		ReentrySeq:    reentrySeq,
		CreatedAt:     now,
	}
	if err := s.d.Store.CreateReworkRecord(tx, rec); err != nil {
		return nil, err
	}
	if err := audit(tx, s.d, domain.NewID("tx"), "lot", lot.ID, domain.AuditReworkCreate, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
