package domain

import "time"

// Recipe 配方：按设备族适配，拥有多个版本。
type Recipe struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	EquipmentFamily string    `json:"equipment_family"` // 只能在该设备族上执行
	CreatedAt       time.Time `json:"created_at"`
}

// RecipeVersion 配方版本：启用时生成不可变快照。
type RecipeVersion struct {
	ID          string       `json:"id"`
	RecipeID    string       `json:"recipe_id"`
	Version     int          `json:"version"`
	Status      RecipeStatus `json:"status"`
	ParamsJSON  string       `json:"params_json"`  // 草稿参数，启用后冻结
	Snapshot    string       `json:"snapshot"`     // 不可变快照（启用时生成）
	ActivatedAt *time.Time   `json:"activated_at,omitempty"`
	RowVersion  int          `json:"row_version"` // 乐观锁
	CreatedAt   time.Time    `json:"created_at"`
}

// Validate 校验配方字段。
func (r *Recipe) Validate() error {
	var fields []FieldError
	if r.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "配方编码不能为空"})
	}
	if r.EquipmentFamily == "" {
		fields = append(fields, FieldError{Field: "equipment_family", Message: "适配设备族不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
