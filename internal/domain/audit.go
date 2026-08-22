package domain

import "time"

// AuditEvent 结构化审计事件：在业务事务内同步写入，随事务回滚而消失。
type AuditEvent struct {
	ID        string    `json:"id"`
	Entity    string    `json:"entity"`    // 实体类型，如 lot / run / hold
	EntityID  string    `json:"entity_id"` // 实体 ID
	Action    string    `json:"action"`    // 动作，如 create / split / freeze / complete
	Detail    string    `json:"detail"`    // JSON 详情
	TxTag     string    `json:"tx_tag"`    // 事务标签，关联同一事务内的多条审计
	CreatedAt time.Time `json:"created_at"`
}

// 审计动作常量。
const (
	AuditLotCreate        = "lot.create"
	AuditLotSplit         = "lot.split"
	AuditLotFreeze        = "lot.freeze"
	AuditLotEnter         = "lot.enter"
	AuditLotClose         = "lot.close"
	AuditLotScrap         = "lot.scrap"
	AuditLotRestore       = "lot.restore"
	AuditRunCreate        = "run.create"
	AuditRunComplete      = "run.complete"
	AuditReadingSeal      = "reading.seal"
	AuditHoldCreate       = "hold.create"
	AuditHoldReview       = "hold.review"
	AuditReworkCreate     = "rework.create"
	AuditReleaseCreate    = "release.create"
	AuditTxRollback       = "tx.rollback"
	AuditJobQualification = "job.qualification_scan"
	AuditJobRunTimeout    = "job.run_timeout"
	AuditJobEscalation    = "job.hold_escalation"
	AuditJobRetry         = "job.retry"
)
