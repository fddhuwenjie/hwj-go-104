package domain

import "time"

// Run 制程运行：批次在某站点、某设备腔体上按冻结快照执行的一次制程。
type Run struct {
	ID              string `json:"id"`
	LotID           string `json:"lot_id"`
	RouteRevisionID string `json:"route_revision_id"` // 冻结修订
	StationSeq      int    `json:"station_seq"`
	StationID       string `json:"station_id"`
	EquipmentID     string `json:"equipment_id"`
	ChamberID       string `json:"chamber_id"`
	RecipeVersionID string `json:"recipe_version_id"`
	// RecipeSnapshot 运行实际使用的不可变配方快照副本。
	RecipeSnapshot string    `json:"recipe_snapshot"`
	Status         RunStatus `json:"status"`
	Judgment       Judgment  `json:"judgment"`
	// QualCovered 完工时资质窗口是否覆盖完整运行区间。
	QualCovered bool `json:"qual_covered"`
	// Reviewed 过期资质运行是否已完成复判。
	Reviewed    bool       `json:"reviewed"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Version     int        `json:"version"` // 乐观锁
	CreatedAt   time.Time  `json:"created_at"`
}

// RunWafer 运行与晶圆的关联。
type RunWafer struct {
	RunID   string `json:"run_id"`
	WaferID string `json:"wafer_id"`
}

// Validate 校验运行创建字段。
func (r *Run) Validate() error {
	var fields []FieldError
	if r.LotID == "" {
		fields = append(fields, FieldError{Field: "lot_id", Message: "批次不能为空"})
	}
	if r.EquipmentID == "" {
		fields = append(fields, FieldError{Field: "equipment_id", Message: "设备不能为空"})
	}
	if r.ChamberID == "" {
		fields = append(fields, FieldError{Field: "chamber_id", Message: "腔体不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
