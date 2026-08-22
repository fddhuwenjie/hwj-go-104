package domain

import "time"

// Release 放行记录：站点间放行与批次关闭的不可变凭证。
type Release struct {
	ID        string    `json:"id"`
	LotID     string    `json:"lot_id"`
	FromSeq   int       `json:"from_seq"` // 放行来源站点顺序号，0 表示入厂放行
	ToSeq     int       `json:"to_seq"`   // 放行目标站点顺序号，0 表示出厂/关闭
	Kind      string    `json:"kind"`     // NEXT_STATION / CLOSE / RESTORE
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// 放行记录种类。
const (
	ReleaseNextStation = "NEXT_STATION"
	ReleaseClose       = "CLOSE"
	ReleaseRestore     = "RESTORE"
)
