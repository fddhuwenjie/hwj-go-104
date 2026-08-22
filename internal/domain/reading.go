package domain

import "time"

// Reading 晶圆位置读数：量测采集结果，附着于运行。
type Reading struct {
	ID      string  `json:"id"`
	RunID   string  `json:"run_id"`
	WaferID string  `json:"wafer_id"`
	Slot    int     `json:"slot"`   // 晶圆槽位，用于抽样覆盖校验
	Metric  string  `json:"metric"` // 量测指标
	Value   float64 `json:"value"`
	// Late 迟到量测：运行判定后补录，仅附着原运行，不覆盖有效判定。
	Late bool `json:"late"`
	// Sealed 是否已随量测封存。
	Sealed    bool      `json:"sealed"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate 校验读数字段。
func (r *Reading) Validate() error {
	var fields []FieldError
	if r.WaferID == "" {
		fields = append(fields, FieldError{Field: "wafer_id", Message: "晶圆不能为空"})
	}
	if r.Metric == "" {
		fields = append(fields, FieldError{Field: "metric", Message: "量测指标不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
