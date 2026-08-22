package domain

// LotStatus 批次状态。
type LotStatus string

const (
	LotRegistered LotStatus = "REGISTERED" // 已登记，未进站
	LotQueued     LotStatus = "QUEUED"     // 已进站排队
	LotRunning    LotStatus = "RUNNING"    // 站点运行中
	LotWaiting    LotStatus = "WAITING"    // 站点间等待放行/下一站点
	LotOnHold     LotStatus = "ON_HOLD"    // 存在未关闭暂扣
	LotCompleted  LotStatus = "COMPLETED"  // 全部站点完成
	LotClosed     LotStatus = "CLOSED"     // 正常关闭
	LotScrapped   LotStatus = "SCRAPPED"   // 已报废
)

// RevisionStatus 路线修订状态。
type RevisionStatus string

const (
	RevDraft   RevisionStatus = "DRAFT"
	RevActive  RevisionStatus = "ACTIVE"
	RevRetired RevisionStatus = "RETIRED"
)

// RecipeStatus 配方版本状态。
type RecipeStatus string

const (
	RecipeDraft   RecipeStatus = "DRAFT"
	RecipeActive  RecipeStatus = "ACTIVE"
	RecipeRetired RecipeStatus = "RETIRED"
)

// PlanStatus 量测计划状态。
type PlanStatus string

const (
	PlanDraft   PlanStatus = "DRAFT"
	PlanActive  PlanStatus = "ACTIVE"
	PlanRetired PlanStatus = "RETIRED"
)

// EquipmentStatus 设备状态。
type EquipmentStatus string

const (
	EquipActive EquipmentStatus = "ACTIVE"
	EquipDown   EquipmentStatus = "DOWN"
)

// QualStatus 资质窗口状态。
type QualStatus string

const (
	QualActive  QualStatus = "ACTIVE"
	QualRevoked QualStatus = "REVOKED"
)

// RunStatus 制程运行状态。
type RunStatus string

const (
	RunRunning   RunStatus = "RUNNING"   // 已开工
	RunCompleted RunStatus = "COMPLETED" // 已完工，待量测判定
	RunJudged    RunStatus = "JUDGED"    // 量测已封存并判定
	RunAborted   RunStatus = "ABORTED"   // 中止
)

// Judgment 判定结论。
type Judgment string

const (
	JudgeNone Judgment = "NONE"
	JudgePass Judgment = "PASS"
	JudgeFail Judgment = "FAIL"
)

// HoldStatus 暂扣状态。
type HoldStatus string

const (
	HoldOpen     HoldStatus = "OPEN"     // 未关闭，阻断批次
	HoldReleased HoldStatus = "RELEASED" // 复判放行
	HoldReworked HoldStatus = "REWORKED" // 复判返工
	HoldScrapped HoldStatus = "SCRAPPED" // 复判报废
)

// JobStatus 后台作业状态。
type JobStatus string

const (
	JobPending JobStatus = "PENDING"
	JobRunning JobStatus = "RUNNING"
	JobDone    JobStatus = "DONE"
	JobFailed  JobStatus = "FAILED" // 可重试
	JobDead    JobStatus = "DEAD"   // 超过最大重试次数
)

// WaferStatus 单片晶圆状态。
type WaferStatus string

const (
	WaferActive   WaferStatus = "ACTIVE"
	WaferScrapped WaferStatus = "SCRAPPED"
)

// 后台作业种类。
const (
	JobKindQualificationScan = "QUALIFICATION_SCAN" // 资质到期扫描
	JobKindRunTimeout        = "RUN_TIMEOUT"        // 超时运行检查
	JobKindHoldEscalation    = "HOLD_ESCALATION"    // 暂扣升级
	JobKindRetry             = "JOB_RETRY"          // 失败作业重试
)
