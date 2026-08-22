package domain

import "time"

// ReworkRecord 返工记录：复判返工时创建，关联新路线修订。
type ReworkRecord struct {
	ID            string    `json:"id"`
	LotID         string    `json:"lot_id"`
	HoldID        string    `json:"hold_id"`
	NewRevisionID string    `json:"new_revision_id"` // 返工创建的新路线修订
	ReentrySeq    int       `json:"reentry_seq"`     // 重入站点顺序号
	CreatedAt     time.Time `json:"created_at"`
}

// Validate 校验返工参数。
func (r *ReworkRecord) Validate() error {
	var fields []FieldError
	if r.ReentrySeq <= 0 {
		fields = append(fields, FieldError{Field: "reentry_seq", Message: "重入站点顺序号必须大于 0"})
	}
	if len(fields) > 0 {
		return NewValidationError(fields...)
	}
	return nil
}
