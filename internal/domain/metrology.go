package domain

import "time"

// MetrologyPlan 量测计划：定义抽样位置与最小样本数。
type MetrologyPlan struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Version   int        `json:"version"`
	Status    PlanStatus `json:"status"`
	// SamplePositions 计划要求抽样的晶圆槽位（1 起始）。
	SamplePositions []int `json:"sample_positions"`
	// MinSamples 最小抽样数量。
	MinSamples  int       `json:"min_samples"`
	// PassLimit 判定阈值：读数不超过该值判 PASS。
	PassLimit   float64   `json:"pass_limit"`
	Metric      string    `json:"metric"`
	RowVersion  int       `json:"row_version"`
	CreatedAt   time.Time `json:"created_at"`
}

// Validate 校验量测计划字段。
func (m *MetrologyPlan) Validate() error {
	var fields []FieldError
	if m.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "计划编码不能为空"})
	}
	if len(m.SamplePositions) == 0 {
		fields = append(fields, FieldError{Field: "sample_positions", Message: "抽样位置不能为空"})
	}
	if m.MinSamples <= 0 {
		fields = append(fields, FieldError{Field: "min_samples", Message: "最小抽样数必须大于 0"})
	}
	if m.MinSamples > len(m.SamplePositions) {
		fields = append(fields, FieldError{Field: "min_samples", Message: "最小抽样数不能超过抽样位置数"})
	}
	if m.Metric == "" {
		fields = append(fields, FieldError{Field: "metric", Message: "量测指标不能为空"})
	}
	for _, p := range m.SamplePositions {
		if p <= 0 {
			fields = append(fields, FieldError{Field: "sample_positions", Message: "抽样位置必须为正整数槽位"})
			break
		}
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
