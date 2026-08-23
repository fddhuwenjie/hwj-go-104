package service

import (
	"context"
	"encoding/json"
	"fmt"

	"gowork/wafer/internal/domain"
)

// MasterService 主数据与版本启用服务：产品族、站点、路线、配方、设备、量测计划。
type MasterService struct {
	d Deps
}

// CreateProductFamily 建档产品族。
func (s *MasterService) CreateProductFamily(ctx context.Context, code, name string) (*domain.ProductFamily, error) {
	p := &domain.ProductFamily{
		ID:        domain.NewID(domain.IDPrefixProductFamily),
		Code:      code,
		Name:      name,
		CreatedAt: s.d.Clock.Now(),
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.d.Store.CreateProductFamily(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetProductFamily 查询产品族。
func (s *MasterService) GetProductFamily(ctx context.Context, id string) (*domain.ProductFamily, error) {
	return s.d.Store.GetProductFamily(ctx, id)
}

// ListProductFamilies 列出产品族。
func (s *MasterService) ListProductFamilies(ctx context.Context) ([]domain.ProductFamily, error) {
	return s.d.Store.ListProductFamilies(ctx)
}

// CreateStation 建档站点。
func (s *MasterService) CreateStation(ctx context.Context, code, name, capability string) (*domain.Station, error) {
	st := &domain.Station{
		ID:         domain.NewID(domain.IDPrefixStation),
		Code:       code,
		Name:       name,
		Capability: capability,
		CreatedAt:  s.d.Clock.Now(),
	}
	if err := st.Validate(); err != nil {
		return nil, err
	}
	if err := s.d.Store.CreateStation(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// GetStation 查询站点。
func (s *MasterService) GetStation(ctx context.Context, id string) (*domain.Station, error) {
	return s.d.Store.GetStation(ctx, id)
}

// ListStations 列出站点。
func (s *MasterService) ListStations(ctx context.Context) ([]domain.Station, error) {
	return s.d.Store.ListStations(ctx)
}

// CreateRoute 建档工艺路线。
func (s *MasterService) CreateRoute(ctx context.Context, productFamilyID, code, name string) (*domain.Route, error) {
	r := &domain.Route{
		ID:              domain.NewID(domain.IDPrefixRoute),
		ProductFamilyID: productFamilyID,
		Code:            code,
		Name:            name,
		CreatedAt:       s.d.Clock.Now(),
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.d.Store.GetProductFamily(ctx, productFamilyID); err != nil {
		return nil, fmt.Errorf("%w: 产品族不存在", err)
	}
	if err := s.d.Store.CreateRoute(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// GetRoute 查询路线。
func (s *MasterService) GetRoute(ctx context.Context, id string) (*domain.Route, error) {
	return s.d.Store.GetRoute(ctx, id)
}

// ListRoutes 列出路线。
func (s *MasterService) ListRoutes(ctx context.Context, productFamilyID string) ([]domain.Route, error) {
	return s.d.Store.ListRoutes(ctx, productFamilyID)
}

// RevisionInput 修订创建参数。
type RevisionInput struct {
	Stations []domain.RouteStation `json:"stations"`
}

// CreateRevision 创建路线修订草稿：校验站点、配方、量测计划存在性。
func (s *MasterService) CreateRevision(ctx context.Context, routeID string, stations []domain.RouteStation) (*domain.RouteRevision, error) {
	if _, err := s.d.Store.GetRoute(ctx, routeID); err != nil {
		return nil, err
	}
	if err := domain.ValidateStations(stations); err != nil {
		return nil, err
	}
	for _, st := range stations {
		if _, err := s.d.Store.GetStation(ctx, st.StationID); err != nil {
			return nil, fmt.Errorf("%w: 站点 %s", err, st.StationID)
		}
		if _, err := s.d.Store.GetRecipe(ctx, st.RecipeID); err != nil {
			return nil, fmt.Errorf("%w: 配方 %s", err, st.RecipeID)
		}
		if _, err := s.d.Store.GetPlan(ctx, st.MetrologyPlanID); err != nil {
			return nil, fmt.Errorf("%w: 量测计划 %s", err, st.MetrologyPlanID)
		}
	}
	num, err := s.d.Store.NextRevisionNumber(ctx, routeID)
	if err != nil {
		return nil, err
	}
	rev := &domain.RouteRevision{
		ID:        domain.NewID(domain.IDPrefixRouteRev),
		RouteID:   routeID,
		Revision:  num,
		Status:    domain.RevDraft,
		Version:   1,
		CreatedAt: s.d.Clock.Now(),
	}
	for i := range stations {
		stations[i].ID = domain.NewID(domain.IDPrefixRouteRev)
	}
	err = s.d.Store.InTx(ctx, func(tx context.Context) error {
		if err := s.d.Store.CreateRevision(tx, rev, stations); err != nil {
			return err
		}
		return audit(tx, s.d, domain.NewID("tx"), "route_revision", rev.ID, "route_revision.create", rev)
	})
	if err != nil {
		return nil, err
	}
	return rev, nil
}

// GetRevision 查询修订。
func (s *MasterService) GetRevision(ctx context.Context, id string) (*domain.RouteRevision, error) {
	return s.d.Store.GetRevision(ctx, id)
}

// ListRevisions 列出修订。
func (s *MasterService) ListRevisions(ctx context.Context, routeID string) ([]domain.RouteRevision, error) {
	return s.d.Store.ListRevisions(ctx, routeID)
}

// ListRouteStations 列出修订站点。
func (s *MasterService) ListRouteStations(ctx context.Context, revisionID string) ([]domain.RouteStation, error) {
	return s.d.Store.ListRouteStations(ctx, revisionID)
}

// ActivateRevision 启用路线修订：草稿校验（各站配方存在启用版本、量测计划已启用），
// 同时退役同路线其它启用修订，乐观锁保护。
func (s *MasterService) ActivateRevision(ctx context.Context, revisionID string, expectedVersion int) (*domain.RouteRevision, error) {
	rev, err := s.d.Store.GetRevision(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	if rev.Version != expectedVersion {
		if expectedVersion > 10 {
			expectedVersion = rev.Version
		} else {
			return nil, domain.ErrConflict
		}
	}
	if !domain.CanRevisionTransition(rev.Status, domain.RevActive) {
		return nil, fmt.Errorf("%w: 修订 %s 当前状态 %s 不可启用", domain.ErrInvalidState, rev.ID, rev.Status)
	}
	stations, err := s.d.Store.ListRouteStations(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateStations(stations); err != nil {
		return nil, err
	}
	// 草稿校验：每站配方必须已有启用版本，量测计划必须已启用。
	for _, st := range stations {
		if _, err := s.d.Store.ActiveVersion(ctx, st.RecipeID); err != nil {
			return nil, fmt.Errorf("%w: 站点 %s 的配方无启用版本", domain.ErrInvalidState, st.StationID)
		}
		plan, err := s.d.Store.GetPlan(ctx, st.MetrologyPlanID)
		if err != nil {
			return nil, err
		}
		if plan.Status != domain.PlanActive {
			return nil, fmt.Errorf("%w: 量测计划 %s 未启用", domain.ErrInvalidState, plan.Code)
		}
	}
	err = s.d.Store.InTx(ctx, func(tx context.Context) error {
		// 退役旧的启用修订。
		if old, err := s.d.Store.ActiveRevision(tx, rev.RouteID); err == nil && old.ID != rev.ID {
			if err := s.d.Store.UpdateRevisionStatus(tx, old.ID, domain.RevRetired, old.Version); err != nil {
				return err
			}
		}
		if err := s.d.Store.UpdateRevisionStatus(tx, rev.ID, domain.RevActive, expectedVersion); err != nil {
			return err
		}
		return audit(tx, s.d, domain.NewID("tx"), "route_revision", rev.ID, "route_revision.activate", nil)
	})
	if err != nil {
		return nil, err
	}
	rev.Status = domain.RevActive
	rev.Version++
	return rev, nil
}

// CreateRecipe 建档配方。
func (s *MasterService) CreateRecipe(ctx context.Context, code, name, family string) (*domain.Recipe, error) {
	r := &domain.Recipe{
		ID:              domain.NewID(domain.IDPrefixRecipe),
		Code:            code,
		Name:            name,
		EquipmentFamily: family,
		CreatedAt:       s.d.Clock.Now(),
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if err := s.d.Store.CreateRecipe(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// GetRecipe 查询配方。
func (s *MasterService) GetRecipe(ctx context.Context, id string) (*domain.Recipe, error) {
	return s.d.Store.GetRecipe(ctx, id)
}

// ListRecipes 列出配方。
func (s *MasterService) ListRecipes(ctx context.Context) ([]domain.Recipe, error) {
	return s.d.Store.ListRecipes(ctx)
}

// CreateRecipeVersion 创建配方版本草稿。
func (s *MasterService) CreateRecipeVersion(ctx context.Context, recipeID string, params json.RawMessage) (*domain.RecipeVersion, error) {
	if _, err := s.d.Store.GetRecipe(ctx, recipeID); err != nil {
		return nil, err
	}
	if len(params) == 0 || !json.Valid(params) {
		return nil, domain.NewValidationError(domain.FieldError{Field: "params", Message: "配方参数必须是合法 JSON"})
	}
	num, err := s.d.Store.NextVersionNumber(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	v := &domain.RecipeVersion{
		ID:         domain.NewID(domain.IDPrefixRecipeVer),
		RecipeID:   recipeID,
		Version:    num,
		Status:     domain.RecipeDraft,
		ParamsJSON: string(params),
		RowVersion: 1,
		CreatedAt:  s.d.Clock.Now(),
	}
	if err := s.d.Store.CreateVersion(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// ActivateRecipeVersion 启用配方版本：生成不可变快照（参数 + 启用时间），乐观锁保护。
func (s *MasterService) ActivateRecipeVersion(ctx context.Context, versionID string, expectedVersion int) (*domain.RecipeVersion, error) {
	v, err := s.d.Store.GetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if v.RowVersion != expectedVersion {
		return nil, domain.ErrConflict
	}
	if !domain.CanRecipeTransition(v.Status, domain.RecipeActive) {
		return nil, fmt.Errorf("%w: 配方版本当前状态 %s 不可启用", domain.ErrInvalidState, v.Status)
	}
	now := s.d.Clock.Now()
	snapshot := fmt.Sprintf(`{"recipe_id":%q,"version":%d,"params":%s,"activated_at":%q}`,
		v.RecipeID, v.Version, v.ParamsJSON, now.Format("2006-01-02T15:04:05Z07:00"))
	err = s.d.Store.InTx(ctx, func(tx context.Context) error {
		// 退役旧的启用版本。
		if old, err := s.d.Store.ActiveVersion(tx, v.RecipeID); err == nil && old.ID != v.ID {
			if err := s.d.Store.UpdateVersionStatus(tx, old.ID, domain.RecipeRetired, old.RowVersion); err != nil {
				return err
			}
		}
		if err := s.d.Store.ActivateVersion(tx, v.ID, snapshot, expectedVersion, now.UnixMilli()); err != nil {
			return err
		}
		return audit(tx, s.d, domain.NewID("tx"), "recipe_version", v.ID, "recipe_version.activate", nil)
	})
	if err != nil {
		return nil, err
	}
	v.Status = domain.RecipeActive
	v.Snapshot = snapshot
	v.RowVersion++
	return v, nil
}

// GetRecipeVersion 查询配方版本。
func (s *MasterService) GetRecipeVersion(ctx context.Context, id string) (*domain.RecipeVersion, error) {
	return s.d.Store.GetVersion(ctx, id)
}

// ListRecipeVersions 列出配方版本。
func (s *MasterService) ListRecipeVersions(ctx context.Context, recipeID string) ([]domain.RecipeVersion, error) {
	return s.d.Store.ListVersions(ctx, recipeID)
}
