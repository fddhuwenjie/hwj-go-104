package domain

import "time"

// Route 工艺路线：属于一个产品族，拥有多个修订。
type Route struct {
	ID              string    `json:"id"`
	ProductFamilyID string    `json:"product_family_id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	CreatedAt       time.Time `json:"created_at"`
}

// RouteRevision 工艺路线修订：版本化，启用后不可修改。
type RouteRevision struct {
	ID       string         `json:"id"`
	RouteID  string         `json:"route_id"`
	Revision int            `json:"revision"`
	Status   RevisionStatus `json:"status"`
	// ReworkFromHoldID 非空表示该修订由返工复判创建。
	ReworkFromHoldID string `json:"rework_from_hold_id,omitempty"`
	// ReentrySeq 返工重入站点顺序号（仅返工修订使用）。
	ReentrySeq int       `json:"reentry_seq,omitempty"`
	Version    int       `json:"version"` // 乐观锁
	CreatedAt  time.Time `json:"created_at"`
}

// RouteStation 路线站点：修订内按 seq 排序的站点定义。
type RouteStation struct {
	ID              string `json:"id"`
	RouteRevisionID string `json:"route_revision_id"`
	Seq             int    `json:"seq"`
	StationID       string `json:"station_id"`
	RecipeID        string `json:"recipe_id"`
	MetrologyPlanID string `json:"metrology_plan_id"`
}

// Validate 校验路线字段。
func (r *Route) Validate() error {
	var fields []FieldError
	if r.ProductFamilyID == "" {
		fields = append(fields, FieldError{Field: "product_family_id", Message: "产品族不能为空"})
	}
	if r.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "路线编码不能为空"})
	}
	if r.Name == "" {
		fields = append(fields, FieldError{Field: "name", Message: "路线名称不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}

// ValidateStations 校验修订站点序列：顺序号必须从 1 开始连续递增。
func ValidateStations(stations []RouteStation) error {
	if len(stations) == 0 {
		return NewValidationError(FieldError{Field: "stations", Message: "路线至少包含一个站点"})
	}
	for i, s := range stations {
		if s.Seq != i+1 {
			return NewValidationError(FieldError{Field: "stations", Message: "站点顺序号必须从 1 开始连续递增"})
		}
		if s.StationID == "" || s.RecipeID == "" || s.MetrologyPlanID == "" {
			return NewValidationError(FieldError{Field: "stations", Message: "站点必须绑定站点、配方与量测计划"})
		}
	}
	return nil
}
