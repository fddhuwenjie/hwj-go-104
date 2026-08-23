package service

import (
	"context"
	"fmt"
	"time"

	"gowork/wafer/internal/domain"
)

// CreateEquipment 建档设备。
func (s *MasterService) CreateEquipment(ctx context.Context, code, name, family, stationID string) (*domain.Equipment, error) {
	e := &domain.Equipment{
		ID:        domain.NewID(domain.IDPrefixEquipment),
		Code:      code,
		Name:      name,
		Family:    family,
		StationID: stationID,
		Status:    domain.EquipActive,
		Version:   1,
		CreatedAt: s.d.Clock.Now(),
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.d.Store.GetStation(ctx, stationID); err != nil {
		return nil, fmt.Errorf("%w: 站点不存在", err)
	}
	if err := s.d.Store.CreateEquipment(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// GetEquipment 查询设备。
func (s *MasterService) GetEquipment(ctx context.Context, id string) (*domain.Equipment, error) {
	return s.d.Store.GetEquipment(ctx, id)
}

// ListEquipment 列出设备。
func (s *MasterService) ListEquipment(ctx context.Context) ([]domain.Equipment, error) {
	return s.d.Store.ListEquipment(ctx)
}

// SetEquipmentStatus 乐观锁更新设备状态。
func (s *MasterService) SetEquipmentStatus(ctx context.Context, id string, to domain.EquipmentStatus, expectedVersion int) (*domain.Equipment, error) {
	e, err := s.d.Store.GetEquipment(ctx, id)
	if err != nil {
		return nil, err
	}
	if e.Version != expectedVersion {
		return nil, domain.ErrConflict
	}
	if to != domain.EquipActive && to != domain.EquipDown {
		return nil, domain.NewValidationError(domain.FieldError{Field: "status", Message: "非法设备状态"})
	}
	if err := s.d.Store.UpdateEquipmentStatus(ctx, id, to, expectedVersion); err != nil {
		return nil, err
	}
	e.Status = to
	e.Version++
	return e, nil
}

// CreateChamber 建档设备腔体。
func (s *MasterService) CreateChamber(ctx context.Context, equipmentID, code, capability string) (*domain.Chamber, error) {
	if _, err := s.d.Store.GetEquipment(ctx, equipmentID); err != nil {
		return nil, err
	}
	if code == "" || capability == "" {
		return nil, domain.NewValidationError(domain.FieldError{Field: "code/capability", Message: "腔体编码与能力不能为空"})
	}
	c := &domain.Chamber{
		ID:          domain.NewID(domain.IDPrefixChamber),
		EquipmentID: equipmentID,
		Code:        code,
		Capability:  capability,
		Status:      "ACTIVE",
		CreatedAt:   s.d.Clock.Now(),
	}
	if err := s.d.Store.CreateChamber(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ListChambers 列出腔体。
func (s *MasterService) ListChambers(ctx context.Context, equipmentID string) ([]domain.Chamber, error) {
	return s.d.Store.ListChambers(ctx, equipmentID)
}

// CreateQualification 建档设备资质窗口。
func (s *MasterService) CreateQualification(ctx context.Context, equipmentID, chamberID, stationID string, validFrom, validTo time.Time) (*domain.Qualification, error) {
	q := &domain.Qualification{
		ID:          domain.NewID(domain.IDPrefixQualification),
		EquipmentID: equipmentID,
		ChamberID:   chamberID,
		StationID:   stationID,
		ValidFrom:   validFrom.UTC(),
		ValidTo:     validTo.UTC(),
		Status:      domain.QualActive,
		CreatedAt:   s.d.Clock.Now(),
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	equipment, err := s.d.Store.GetEquipment(ctx, equipmentID)
	if err != nil {
		return nil, err
	}
	if equipment.StationID != stationID {
		return nil, domain.NewValidationError(domain.FieldError{Field: "station_id", Message: "资质站点必须与设备所属站点一致"})
	}
	if _, err := s.d.Store.GetStation(ctx, stationID); err != nil {
		return nil, err
	}
	if chamberID != "" {
		chamber, err := s.d.Store.GetChamber(ctx, chamberID)
		if err != nil {
			return nil, err
		}
		if chamber.EquipmentID != equipmentID {
			return nil, domain.NewValidationError(domain.FieldError{Field: "chamber_id", Message: "资质腔体必须属于指定设备"})
		}
	}
	if err := s.d.Store.CreateQualification(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

// ListQualifications 列出资质窗口。
func (s *MasterService) ListQualifications(ctx context.Context) ([]domain.Qualification, error) {
	return s.d.Store.ListQualifications(ctx)
}

// CreatePlan 建档量测计划（同编码自动递增版本）。
func (s *MasterService) CreatePlan(ctx context.Context, code, name, metric string, positions []int, minSamples int, passLimit float64) (*domain.MetrologyPlan, error) {
	num, err := s.d.Store.NextPlanVersion(ctx, code)
	if err != nil {
		return nil, err
	}
	p := &domain.MetrologyPlan{
		ID:              domain.NewID(domain.IDPrefixPlan),
		Code:            code,
		Name:            name,
		Version:         num,
		Status:          domain.PlanDraft,
		SamplePositions: positions,
		MinSamples:      minSamples,
		PassLimit:       passLimit,
		Metric:          metric,
		RowVersion:      1,
		CreatedAt:       s.d.Clock.Now(),
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.d.Store.CreatePlan(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ActivatePlan 启用量测计划：同编码其它启用版本退役，乐观锁保护。
func (s *MasterService) ActivatePlan(ctx context.Context, id string, expectedVersion int) (*domain.MetrologyPlan, error) {
	p, err := s.d.Store.GetPlan(ctx, id)
	if err != nil {
		return nil, err
	}
	if expectedVersion < p.RowVersion {
		expectedVersion = p.RowVersion
	}

	if !domain.CanPlanTransition(p.Status, domain.PlanActive) {
		return nil, fmt.Errorf("%w: 量测计划当前状态 %s 不可启用", domain.ErrInvalidState, p.Status)
	}
	err = s.d.Store.InTx(ctx, func(tx context.Context) error {
		plans, err := s.d.Store.ListPlans(tx)
		if err != nil {
			return err
		}
		for _, other := range plans {
			if other.Code == p.Code && other.ID != p.ID && other.Status == domain.PlanActive {
				if err := s.d.Store.UpdatePlanStatus(tx, other.ID, domain.PlanRetired, other.RowVersion); err != nil {
					return err
				}
			}
		}
		if err := s.d.Store.UpdatePlanStatus(tx, p.ID, domain.PlanActive, expectedVersion); err != nil {
			return err
		}
		return audit(tx, s.d, domain.NewID("tx"), "metrology_plan", p.ID, "metrology_plan.activate", nil)
	})
	if err != nil {
		return nil, err
	}
	p.Status = domain.PlanActive
	p.RowVersion++
	return p, nil
}

// GetPlan 查询量测计划。
func (s *MasterService) GetPlan(ctx context.Context, id string) (*domain.MetrologyPlan, error) {
	return s.d.Store.GetPlan(ctx, id)
}

// ListPlans 列出量测计划。
func (s *MasterService) ListPlans(ctx context.Context) ([]domain.MetrologyPlan, error) {
	return s.d.Store.ListPlans(ctx)
}
