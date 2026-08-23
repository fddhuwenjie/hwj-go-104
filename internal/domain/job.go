package domain

import "time"

// Job 后台作业：支持持久化、失败重试与重启恢复。
type Job struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Payload     string    `json:"payload"` // JSON 参数
	Status      JobStatus `json:"status"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	RunAt       time.Time `json:"run_at"` // 下次可执行时间
	LastError   string    `json:"last_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CanRun 判断作业到期可执行。载荷只携带业务参数，不能改变调度时间：
// 任何作业都必须等到 RunAt 到期方可运行。
func (j Job) CanRun(now time.Time) bool {
	return j.Status == JobPending && !j.RunAt.After(now)
}

// Retryable 判断失败作业是否可重试。
func (j Job) Retryable() bool {
	return j.Status == JobFailed && j.Attempts < j.MaxAttempts
}
