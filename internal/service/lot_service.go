package service

import (
	"context"
	"fmt"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/rules"
)

// LotService 批次服务：登记、拆分、冻结、进站、放行、关闭、报废、恢复。
type LotService struct {
	d Deps
}

// WaferInput 批次登记晶圆输入。
type WaferInput struct {
	Code string `json:"code"`
	Slot int    `json:"slot"`
}

// RegisterLot 批次登记（幂等）。
func (s *LotService) RegisterLot(ctx context.Context, code, productFamilyID, routeID string, wafers []WaferInput, idemKey string) (*domain.Lot, bool, error) {
	return idempotent(ctx, s.d, "lot.register", idemKey, func(tx context.Context) (*domain.Lot, error) {
		lot := &domain.Lot{
			ID:              domain.NewID(domain.IDPrefixLot),
			Code:            code,
			ProductFamilyID: productFamilyID,
			RouteID:         routeID,
			Status:          domain.LotRegistered,
			CreatedAt:       s.d.Clock.Now(),
		}
		if err := lot.Validate(); err != nil {
			return nil, err
		}
		if _, err := s.d.Store.GetProductFamily(tx, productFamilyID); err != nil {
			return nil, err
		}
		route, err := s.d.Store.GetRoute(tx, routeID)
		if err != nil {
			return nil, err
		}
		if route.ProductFamilyID != productFamilyID {
			return nil, domain.NewValidationError(domain.FieldError{Field: "route_id", Message: "工艺路线必须属于批次产品族"})
		}
		var ws []domain.Wafer
		for _, in := range wafers {
			ws = append(ws, domain.Wafer{
				ID:        domain.NewID(domain.IDPrefixWafer),
				Code:      in.Code,
				Slot:      in.Slot,
				Status:    domain.WaferActive,
				CreatedAt: s.d.Clock.Now(),
			})
		}
		if err := domain.ValidateWafers(ws); err != nil {
			return nil, err
		}
		if err := s.d.Store.CreateLot(tx, lot, ws); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "lot", lot.ID, domain.AuditLotCreate, lot); err != nil {
			return nil, err
		}
		return lot, nil
	})
}

// GetLot 查询批次。
func (s *LotService) GetLot(ctx context.Context, id string) (*domain.Lot, error) {
	return s.d.Store.GetLot(ctx, id)
}

// ListLots 分页查询批次。
func (s *LotService) ListLots(ctx context.Context, page domain.Page) ([]domain.Lot, string, error) {
	lots, err := s.d.Store.ListLots(ctx, page)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(lots) == page.Normalize().Limit && len(lots) > 0 {
		last := lots[len(lots)-1]
		next = domain.EncodeCursor(last.CreatedAt, last.ID)
	}
	return lots, next, nil
}

// ListWafers 查询批次晶圆。
func (s *LotService) ListWafers(ctx context.Context, lotID string) ([]domain.Wafer, error) {
	return s.d.Store.ListWafers(ctx, lotID)
}

// WaferGenealogy 晶圆谱系：晶圆信息 + 迁移历史。
func (s *LotService) WaferGenealogy(ctx context.Context, waferID string) (*domain.Wafer, []domain.WaferMove, error) {
	w, err := s.d.Store.GetWafer(ctx, waferID)
	if err != nil {
		return nil, nil, err
	}
	moves, err := s.d.Store.WaferMoves(ctx, waferID)
	if err != nil {
		return nil, nil, err
	}
	return w, moves, nil
}

// SplitLot 子批拆分：创建子批并迁移晶圆，真实事务，任一步失败整体回滚（幂等）。
func (s *LotService) SplitLot(ctx context.Context, lotID, childCode string, waferIDs []string, idemKey string) (*domain.Lot, bool, error) {
	return idempotent(ctx, s.d, "lot.split:"+lotID, idemKey, func(tx context.Context) (*domain.Lot, error) {
		parent, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		if parent.Status == domain.LotClosed || parent.Status == domain.LotScrapped {
			return nil, fmt.Errorf("%w: 批次已关闭或报废，不可拆分", domain.ErrInvalidState)
		}
		if len(waferIDs) == 0 {
			return nil, domain.NewValidationError(domain.FieldError{Field: "wafer_ids", Message: "拆分必须指定迁移晶圆"})
		}
		// 校验晶圆归属父批。
		parentWafers, err := s.d.Store.ListWafers(tx, lotID)
		if err != nil {
			return nil, err
		}
		owned := map[string]bool{}
		for _, w := range parentWafers {
			owned[w.ID] = true
		}
		for _, id := range waferIDs {
			if !owned[id] {
				return nil, fmt.Errorf("%w: 晶圆 %s 不属于批次", domain.ErrValidation, id)
			}
		}
		if len(waferIDs) == len(parentWafers) {
			return nil, fmt.Errorf("%w: 拆分必须保留至少一片晶圆在父批", domain.ErrValidation)
		}
		child := &domain.Lot{
			ID:              domain.NewID(domain.IDPrefixLot),
			Code:            childCode,
			ProductFamilyID: parent.ProductFamilyID,
			RouteID:         parent.RouteID,
			Status:          domain.LotRegistered,
			ParentLotID:     parent.ID,
			CreatedAt:       s.d.Clock.Now(),
		}
		// 继承父批冻结快照与站点进度，保证谱系内路线一致。
		if parent.IsFrozen() {
			child.FrozenRevisionID = parent.FrozenRevisionID
			child.FreezeSnapshot = parent.FreezeSnapshot
			child.FrozenAt = parent.FrozenAt
			child.CurrentSeq = parent.CurrentSeq
			child.EnteredAt = parent.EnteredAt
			if parent.Status == domain.LotQueued || parent.Status == domain.LotWaiting {
				child.Status = parent.Status
			}
		}
		if err := child.Validate(); err != nil {
			return nil, err
		}
		if err := s.d.Store.CreateLot(tx, child, nil); err != nil {
			return nil, err
		}
		now := s.d.Clock.Now().UnixMilli()
		for _, wid := range waferIDs {
			if err := s.d.Store.MoveWafer(tx, wid, child.ID, now); err != nil {
				return nil, err
			}
		}
		txTag := domain.NewID("tx")
		if err := audit(tx, s.d, txTag, "lot", parent.ID, domain.AuditLotSplit,
			map[string]any{"child_lot_id": child.ID, "wafer_ids": waferIDs}); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, txTag, "lot", child.ID, domain.AuditLotSplit,
			map[string]any{"parent_lot_id": parent.ID, "wafer_ids": waferIDs}); err != nil {
			return nil, err
		}
		return child, nil
	})
}

// freezeIfNeeded 首次进站冻结路线修订、站点顺序、配方快照与量测计划。
func (s *LotService) freezeIfNeeded(ctx context.Context, lot *domain.Lot) error {
	if lot.IsFrozen() {
		return nil
	}
	route, err := s.d.Store.GetRoute(ctx, lot.RouteID)
	if err != nil {
		return err
	}
	rev, err := s.d.Store.ActiveRevision(ctx, lot.RouteID)
	if err != nil {
		return fmt.Errorf("%w: 路线无启用修订", err)
	}
	stations, err := s.d.Store.ListRouteStations(ctx, rev.ID)
	if err != nil {
		return err
	}
	stationByID := map[string]domain.Station{}
	recipeByStation := map[string]domain.RecipeVersion{}
	planByID := map[string]domain.MetrologyPlan{}
	for _, rs := range stations {
		st, err := s.d.Store.GetStation(ctx, rs.StationID)
		if err != nil {
			return err
		}
		stationByID[rs.StationID] = *st
		rv, err := s.d.Store.ActiveVersion(ctx, rs.RecipeID)
		if err != nil {
			return fmt.Errorf("%w: 配方 %s 无启用版本", err, rs.RecipeID)
		}
		recipeByStation[rs.StationID] = *rv
		plan, err := s.d.Store.GetPlan(ctx, rs.MetrologyPlanID)
		if err != nil {
			return err
		}
		planByID[rs.MetrologyPlanID] = *plan
	}
	snap, err := rules.BuildFreezeSnapshot(*route, *rev, stations, stationByID, recipeByStation, planByID)
	if err != nil {
		return err
	}
	raw, err := snap.Encode()
	if err != nil {
		return err
	}
	now := s.d.Clock.Now()
	lot.FrozenRevisionID = rev.ID
	lot.FreezeSnapshot = raw
	lot.FrozenAt = &now
	return nil
}

// Enter 进站排队：首次进站触发冻结；校验站点顺序与暂扣阻断（幂等）。
func (s *LotService) Enter(ctx context.Context, lotID string, idemKey string) (*domain.Lot, bool, error) {
	return idempotent(ctx, s.d, "lot.enter:"+lotID, idemKey, func(tx context.Context) (*domain.Lot, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		// 先校验暂扣阻断，保证阻断错误码优先暴露。
		if err := holdBlockingCheck(tx, s.d, lot.ID); err != nil {
			return nil, err
		}
		if lot.Status != domain.LotRegistered && lot.Status != domain.LotWaiting {
			return nil, fmt.Errorf("%w: 批次状态 %s 不可进站", domain.ErrInvalidState, lot.Status)
		}
		if err := s.freezeIfNeeded(tx, lot); err != nil {
			return nil, err
		}
		snap, err := loadFreeze(lot)
		if err != nil {
			return nil, err
		}
		next := lot.CurrentSeq + 1
		if err := rules.CheckStationOrder(lot.CurrentSeq, next, snap); err != nil {
			return nil, err
		}
		now := s.d.Clock.Now()
		lot.CurrentSeq = next
		lot.Status = domain.LotQueued
		lot.EnteredAt = &now
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		lot.Version++
		txTag := domain.NewID("tx")
		action := domain.AuditLotEnter
		if lot.FrozenAt != nil && lot.FrozenAt.Equal(now) {
			action = domain.AuditLotFreeze
		}
		if err := audit(tx, s.d, txTag, "lot", lot.ID, action,
			map[string]any{"seq": next, "revision_id": lot.FrozenRevisionID}); err != nil {
			return nil, err
		}
		return lot, nil
	})
}

// ReleaseNext 下一站放行：生成放行记录并推进到下一站点排队（幂等）。
func (s *LotService) ReleaseNext(ctx context.Context, lotID, note string, idemKey string) (*domain.Release, bool, error) {
	return idempotent(ctx, s.d, "lot.release:"+lotID, idemKey, func(tx context.Context) (*domain.Release, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		if lot.Status != domain.LotWaiting {
			return nil, fmt.Errorf("%w: 批次状态 %s 不可放行", domain.ErrInvalidState, lot.Status)
		}
		if err := holdBlockingCheck(tx, s.d, lot.ID); err != nil {
			return nil, err
		}
		snap, err := loadFreeze(lot)
		if err != nil {
			return nil, err
		}
		// 当前站点运行必须已判定 PASS。
		runs, err := s.d.Store.ListRunsByLot(tx, lot.ID)
		if err != nil {
			return nil, err
		}
		var current *domain.Run
		for i := range runs {
			if runs[i].StationSeq == lot.CurrentSeq && runs[i].RouteRevisionID == lot.FrozenRevisionID {
				current = &runs[i]
			}
		}
		// 当前站点运行必须已封存判定；判定失败但被复判放行的批次，
		// 因未关闭暂扣校验已通过，允许继续放行（人工复判结论优先）。
		if current == nil || current.Status != domain.RunJudged {
			return nil, fmt.Errorf("%w: 当前站点运行未判定，不可放行", domain.ErrInvalidState)
		}
		if current.Judgment == domain.JudgeNone {
			return nil, fmt.Errorf("%w: 当前站点运行无判定结论，不可放行", domain.ErrInvalidState)
		}
		now := s.d.Clock.Now()
		rel := &domain.Release{
			ID:        domain.NewID(domain.IDPrefixRelease),
			LotID:     lot.ID,
			FromSeq:   lot.CurrentSeq,
			Kind:      domain.ReleaseNextStation,
			Note:      note,
			CreatedAt: now,
		}
		if lot.CurrentSeq >= snap.LastSeq() {
			// 末站放行：批次完成。
			rel.ToSeq = 0
			lot.Status = domain.LotCompleted
			lot.EnteredAt = nil
		} else {
			rel.ToSeq = lot.CurrentSeq + 1
			lot.CurrentSeq++
			lot.Status = domain.LotQueued
			lot.EnteredAt = &now
		}
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		if err := s.d.Store.CreateRelease(tx, rel); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "lot", lot.ID, domain.AuditReleaseCreate, rel); err != nil {
			return nil, err
		}
		return rel, nil
	})
}

// Close 批次关闭：COMPLETED -> CLOSED，写入关闭放行记录（幂等）。
func (s *LotService) Close(ctx context.Context, lotID string, idemKey string) (*domain.Lot, bool, error) {
	return idempotent(ctx, s.d, "lot.close:"+lotID, idemKey, func(tx context.Context) (*domain.Lot, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		if !domain.CanLotTransition(lot.Status, domain.LotClosed) {
			return nil, fmt.Errorf("%w: 批次状态 %s 不可关闭", domain.ErrInvalidState, lot.Status)
		}
		now := s.d.Clock.Now()
		lot.Status = domain.LotClosed
		lot.ClosedAt = &now
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		lot.Version++
		rel := &domain.Release{
			ID:        domain.NewID(domain.IDPrefixRelease),
			LotID:     lot.ID,
			FromSeq:   lot.CurrentSeq,
			ToSeq:     0,
			Kind:      domain.ReleaseClose,
			CreatedAt: now,
		}
		if err := s.d.Store.CreateRelease(tx, rel); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "lot", lot.ID, domain.AuditLotClose, nil); err != nil {
			return nil, err
		}
		return lot, nil
	})
}

// Scrap 批次报废（幂等）。
func (s *LotService) Scrap(ctx context.Context, lotID, reason string, idemKey string) (*domain.Lot, bool, error) {
	return idempotent(ctx, s.d, "lot.scrap:"+lotID, idemKey, func(tx context.Context) (*domain.Lot, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		if !domain.CanLotTransition(lot.Status, domain.LotScrapped) {
			return nil, fmt.Errorf("%w: 批次状态 %s 不可报废", domain.ErrInvalidState, lot.Status)
		}
		now := s.d.Clock.Now()
		lot.Status = domain.LotScrapped
		lot.ClosedAt = &now
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		lot.Version++
		if err := audit(tx, s.d, domain.NewID("tx"), "lot", lot.ID, domain.AuditLotScrap,
			map[string]string{"reason": reason}); err != nil {
			return nil, err
		}
		return lot, nil
	})
}

// Restore 恢复原路线：复判放行后批次回到当前站点排队（幂等）。
// 任何批次必须先完成复判（关闭其自身及谱系内的全部开放暂扣）才能恢复，
// 因此恢复前对根批与子批一律校验暂扣阻断，存在开放暂扣即拒绝。
func (s *LotService) Restore(ctx context.Context, lotID string, idemKey string) (*domain.Lot, bool, error) {
	return idempotent(ctx, s.d, "lot.restore:"+lotID, idemKey, func(tx context.Context) (*domain.Lot, error) {
		lot, err := s.d.Store.GetLot(tx, lotID)
		if err != nil {
			return nil, err
		}
		if lot.Status != domain.LotOnHold {
			return nil, fmt.Errorf("%w: 仅暂扣批次可恢复原路线", domain.ErrInvalidState)
		}
		// 先校验暂扣阻断，保证未复判的开放暂扣（自身或谱系内）阻止状态回退。
		if err := holdBlockingCheck(tx, s.d, lot.ID); err != nil {
			return nil, err
		}
		now := s.d.Clock.Now()
		lot.Status = domain.LotQueued
		lot.EnteredAt = &now
		if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
			return nil, err
		}
		lot.Version++
		rel := &domain.Release{
			ID:        domain.NewID(domain.IDPrefixRelease),
			LotID:     lot.ID,
			FromSeq:   lot.CurrentSeq,
			ToSeq:     lot.CurrentSeq,
			Kind:      domain.ReleaseRestore,
			CreatedAt: now,
		}
		if err := s.d.Store.CreateRelease(tx, rel); err != nil {
			return nil, err
		}
		if err := audit(tx, s.d, domain.NewID("tx"), "lot", lot.ID, domain.AuditLotRestore, rel); err != nil {
			return nil, err
		}
		return lot, nil
	})
}
