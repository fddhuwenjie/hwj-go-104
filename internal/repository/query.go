package repository

import (
	"context"
	"time"

	"gowork/wafer/internal/domain"
)

// ExpiredQualificationRun 使用过期资质但尚未复判的运行视图。
type ExpiredQualificationRun struct {
	RunID       string    `json:"run_id"`
	LotID       string    `json:"lot_id"`
	LotCode     string    `json:"lot_code"`
	StationSeq  int       `json:"station_seq"`
	EquipmentID string    `json:"equipment_id"`
	ChamberID   string    `json:"chamber_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// WipLot 在制批次视图：携带冻结路线与最近暂扣原因。
type WipLot struct {
	LotID            string     `json:"lot_id"`
	Code             string     `json:"code"`
	Status           string     `json:"status"`
	CurrentSeq       int        `json:"current_seq"`
	FrozenRevision   *int       `json:"frozen_revision,omitempty"`
	FrozenAt         *time.Time `json:"frozen_at,omitempty"`
	LatestHoldReason string     `json:"latest_hold_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// StationQueueItem 站点队列项：等待超时且存在可用设备。
type StationQueueItem struct {
	StationID        string    `json:"station_id"`
	StationCode      string    `json:"station_code"`
	LotID            string    `json:"lot_id"`
	LotCode          string    `json:"lot_code"`
	QueuedAt         time.Time `json:"queued_at"`
	WaitSeconds      int64     `json:"wait_seconds"`
	CapableEquipment int       `json:"capable_equipment"`
}

// ReworkStat 重复返工聚合：按设备腔体与配方版本分组。
type ReworkStat struct {
	EquipmentID     string `json:"equipment_id"`
	ChamberID       string `json:"chamber_id"`
	RecipeVersionID string `json:"recipe_version_id"`
	ReworkLots      int    `json:"rework_lots"`
}

// GenealogyIssue 父子批审计不一致项。
type GenealogyIssue struct {
	Issue   string `json:"issue"` // STATUS_MISMATCH / WAFER_ORPHAN / WAFER_LOST
	LotID   string `json:"lot_id"`
	Related string `json:"related"` // 关联批次或晶圆
	Detail  string `json:"detail"`
}

// QueryRepo 分析查询仓储。
type QueryRepo interface {
	// ExpiredQualificationRuns 过期资质但尚未复判的运行，稳定分页。
	ExpiredQualificationRuns(ctx context.Context, page domain.Page) ([]ExpiredQualificationRun, error)
	// WipLots 当前在制批次（含冻结路线与最近暂扣原因），稳定分页。
	WipLots(ctx context.Context, page domain.Page) ([]WipLot, error)
	// StationQueues 等待时间超过阈值且设备能力可用的站点队列。
	StationQueues(ctx context.Context, minWait time.Duration, now time.Time) ([]StationQueueItem, error)
	// ReworkStats 按设备腔体与配方版本聚合的重复返工批次。
	ReworkStats(ctx context.Context) ([]ReworkStat, error)
	// GenealogyAudit 父子批状态或晶圆归属不一致审计。
	GenealogyAudit(ctx context.Context) ([]GenealogyIssue, error)
}
