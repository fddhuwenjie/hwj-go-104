package rules

import (
	"fmt"

	"gowork/wafer/internal/domain"
)

// BuildFreezeSnapshot 在批次首次进站时构建冻结快照：
// 固化路线修订、站点顺序、每站配方快照与量测计划快照。
func BuildFreezeSnapshot(
	route domain.Route,
	rev domain.RouteRevision,
	stations []domain.RouteStation,
	stationByID map[string]domain.Station,
	recipeSnapByStation map[string]domain.RecipeVersion,
	planByID map[string]domain.MetrologyPlan,
) (*domain.FreezeSnapshot, error) {
	if rev.Status != domain.RevActive {
		return nil, fmt.Errorf("%w: 路线修订 %s 未启用", domain.ErrInvalidState, rev.ID)
	}
	if err := domain.ValidateStations(stations); err != nil {
		return nil, err
	}
	snap := &domain.FreezeSnapshot{
		RouteID:         route.ID,
		RouteRevisionID: rev.ID,
		Revision:        rev.Revision,
	}
	for _, rs := range stations {
		st, ok := stationByID[rs.StationID]
		if !ok {
			return nil, fmt.Errorf("%w: 站点 %s", domain.ErrNotFound, rs.StationID)
		}
		rv, ok := recipeSnapByStation[rs.StationID]
		if !ok || rv.Status != domain.RecipeActive || rv.Snapshot == "" {
			return nil, fmt.Errorf("%w: 站点 %s 无已启用配方版本", domain.ErrInvalidState, st.Code)
		}
		plan, ok := planByID[rs.MetrologyPlanID]
		if !ok || plan.Status != domain.PlanActive {
			return nil, fmt.Errorf("%w: 站点 %s 无量测计划或未启用", domain.ErrInvalidState, st.Code)
		}
		planSnap, err := marshalPlan(plan)
		if err != nil {
			return nil, err
		}
		snap.Stations = append(snap.Stations, domain.FreezeStation{
			Seq:             rs.Seq,
			StationID:       st.ID,
			StationCode:     st.Code,
			Capability:      st.Capability,
			RecipeID:        rs.RecipeID,
			RecipeVersionID: rv.ID,
			RecipeSnapshot:  rv.Snapshot,
			MetrologyPlanID: plan.ID,
			PlanSnapshot:    planSnap,
		})
	}
	return snap, nil
}

func marshalPlan(p domain.MetrologyPlan) (string, error) {
	b, err := jsonMarshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
