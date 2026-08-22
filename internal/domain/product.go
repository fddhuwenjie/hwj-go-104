package domain

import "time"

// ProductFamily 产品族：晶圆批次归属的产品分类。
type ProductFamily struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Validate 校验产品族字段。
func (p *ProductFamily) Validate() error {
	var fields []FieldError
	if p.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "产品族编码不能为空"})
	}
	if len(p.Code) > 64 {
		fields = append(fields, FieldError{Field: "code", Message: "产品族编码过长"})
	}
	if p.Name == "" {
		fields = append(fields, FieldError{Field: "name", Message: "产品族名称不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
