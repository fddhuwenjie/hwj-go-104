package domain

import "time"

// Hold 暂扣：阻断批次及其子批的后续站点。
type Hold struct {
	ID     string     `json:"id"`
	LotID  string     `json:"lot_id"`
	RunID  string     `json:"run_id,omitempty"` // 触发暂扣的运行（可为空，人工暂扣）
	Reason string     `json:"reason"`
	Status HoldStatus `json:"status"`
	// Escalated 暂扣是否已被后台作业升级。
	Escalated bool       `json:"escalated"`
	Version   int        `json:"version"` // 乐观锁
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	// ReviewNote 复判结论说明。
	ReviewNote string `json:"review_note,omitempty"`
}

// IsOpen 判断暂扣是否未关闭。
func (h Hold) IsOpen() bool { return h.Status == HoldOpen }

// ReviewAction 复判处置动作。
type ReviewAction string

const (
	ReviewRelease ReviewAction = "RELEASE" // 放行
	ReviewRework  ReviewAction = "REWORK"  // 返工换版
	ReviewScrap   ReviewAction = "SCRAP"   // 报废
)

// Validate 校验暂扣字段。
func (h *Hold) Validate() error {
	var fields []FieldError
	if h.LotID == "" {
		fields = append(fields, FieldError{Field: "lot_id", Message: "批次不能为空"})
	}
	if h.Reason == "" {
		fields = append(fields, FieldError{Field: "reason", Message: "暂扣原因不能为空"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
