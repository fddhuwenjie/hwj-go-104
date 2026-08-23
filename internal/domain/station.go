package domain

import "time"

// Station 站点定义：制程中的一个工序站点。
type Station struct {
	ID         string    `json:"id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Capability string    `json:"capability"` // 站点要求的腔体能力标签
	CreatedAt  time.Time `json:"created_at"`
}

// Validate 校验站点字段。
func (s *Station) Validate() error {
	var fields []FieldError
	if s.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "站点编码不能为空"})
	}
	if s.Name == "" {
		fields = append(fields, FieldError{Field: "name", Message: "站点名称不能为空"})
	}
	if s.Capability == "" {
		fields = append(fields, FieldError{Field: "capability", Message: "站点能力标签不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
