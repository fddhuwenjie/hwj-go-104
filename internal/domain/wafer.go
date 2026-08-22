package domain

import "time"

// Wafer 单片晶圆：通过 WaferMove 记录批次间迁移形成谱系。
type Wafer struct {
	ID        string      `json:"id"`
	Code      string      `json:"code"`
	LotID     string      `json:"lot_id"`
	Slot      int         `json:"slot"` // 批内槽位，抽样位置按槽位匹配
	Status    WaferStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

// WaferMove 晶圆迁移记录：谱系的不可变组成部分。
type WaferMove struct {
	ID        string    `json:"id"`
	WaferID   string    `json:"wafer_id"`
	FromLotID string    `json:"from_lot_id"`
	ToLotID   string    `json:"to_lot_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidateWafers 校验批次登记时的晶圆清单。
func ValidateWafers(wafers []Wafer) error {
	if len(wafers) == 0 {
		return NewValidationError(FieldError{Field: "wafers", Message: "批次至少包含一片晶圆"})
	}
	seen := map[int]bool{}
	for _, w := range wafers {
		if w.Slot <= 0 {
			return NewValidationError(FieldError{Field: "wafers", Message: "晶圆槽位必须为正整数"})
		}
		if seen[w.Slot] {
			return NewValidationError(FieldError{Field: "wafers", Message: "晶圆槽位重复"})
		}
		seen[w.Slot] = true
	}
	return nil
}
