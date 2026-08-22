package domain

import "time"

// Lot 晶圆批次：核心在制实体，支持父子批谱系。
type Lot struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	ProductFamilyID string    `json:"product_family_id"`
	RouteID         string    `json:"route_id"`
	Status          LotStatus `json:"status"`
	// CurrentSeq 当前站点顺序号，0 表示尚未进站。
	CurrentSeq int `json:"current_seq"`
	// FrozenRevisionID 首次进站冻结的路线修订。
	FrozenRevisionID string `json:"frozen_revision_id,omitempty"`
	// FreezeSnapshot 冻结快照 JSON：站点顺序、配方快照与量测计划。
	FreezeSnapshot string     `json:"freeze_snapshot,omitempty"`
	FrozenAt       *time.Time `json:"frozen_at,omitempty"`
	// ParentLotID 父批次；拆分产生的子批记录父批形成谱系。
	ParentLotID string `json:"parent_lot_id,omitempty"`
	// EnteredAt 当前站点进站排队时间。
	EnteredAt   *time.Time `json:"entered_at,omitempty"`
	Version     int    `json:"version"` // 乐观锁
	CreatedAt   time.Time `json:"created_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

// IsFrozen 判断批次是否已冻结路线。
func (l Lot) IsFrozen() bool { return l.FrozenRevisionID != "" }

// Validate 校验批次字段。
func (l *Lot) Validate() error {
	var fields []FieldError
	if l.Code == "" {
		fields = append(fields, FieldError{Field: "code", Message: "批次编码不能为空"})
	}
	if l.ProductFamilyID == "" {
		fields = append(fields, FieldError{Field: "product_family_id", Message: "产品族不能为空"})
	}
	if l.RouteID == "" {
		fields = append(fields, FieldError{Field: "route_id", Message: "工艺路线不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
