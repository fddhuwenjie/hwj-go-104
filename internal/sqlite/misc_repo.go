package sqlite

import (
	"context"
	"database/sql"
	"time"

	"gowork/wafer/internal/domain"
)

// --- 放行记录 ---

// CreateRelease 写入放行记录。
func (s *Store) CreateRelease(ctx context.Context, r *domain.Release) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO releases(id, lot_id, from_seq, to_seq, kind, note, created_at) VALUES(?,?,?,?,?,?,?)`,
		r.ID, r.LotID, r.FromSeq, r.ToSeq, r.Kind, r.Note, ms(r.CreatedAt))
	return err
}

// ListReleases 列出批次放行记录。
func (s *Store) ListReleases(ctx context.Context, lotID string) ([]domain.Release, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, lot_id, from_seq, to_seq, kind, note, created_at FROM releases WHERE lot_id=? ORDER BY created_at, id`,
		lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Release
	for rows.Next() {
		var r domain.Release
		var created int64
		if err := rows.Scan(&r.ID, &r.LotID, &r.FromSeq, &r.ToSeq, &r.Kind, &r.Note, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = tm(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- 返工记录 ---

// CreateReworkRecord 写入返工记录。
func (s *Store) CreateReworkRecord(ctx context.Context, r *domain.ReworkRecord) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO rework_records(id, lot_id, hold_id, new_revision_id, reentry_seq, created_at) VALUES(?,?,?,?,?,?)`,
		r.ID, r.LotID, r.HoldID, r.NewRevisionID, r.ReentrySeq, ms(r.CreatedAt))
	return err
}

// ListReworkRecords 列出批次返工记录。
func (s *Store) ListReworkRecords(ctx context.Context, lotID string) ([]domain.ReworkRecord, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, lot_id, hold_id, new_revision_id, reentry_seq, created_at FROM rework_records WHERE lot_id=? ORDER BY created_at, id`,
		lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ReworkRecord
	for rows.Next() {
		var r domain.ReworkRecord
		var created int64
		if err := rows.Scan(&r.ID, &r.LotID, &r.HoldID, &r.NewRevisionID, &r.ReentrySeq, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = tm(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- 审计事件 ---

// CreateAudit 写入结构化审计事件。
func (s *Store) CreateAudit(ctx context.Context, e *domain.AuditEvent) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO audit_events(id, entity, entity_id, action, detail, tx_tag, created_at) VALUES(?,?,?,?,?,?,?)`,
		e.ID, e.Entity, e.EntityID, e.Action, e.Detail, e.TxTag, ms(e.CreatedAt))
	return err
}

// ListAudit 查询实体审计事件。
func (s *Store) ListAudit(ctx context.Context, entity, entityID string) ([]domain.AuditEvent, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, entity, entity_id, action, detail, tx_tag, created_at
		 FROM audit_events WHERE entity=? AND entity_id=? ORDER BY created_at, id`, entity, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var created int64
		if err := rows.Scan(&e.ID, &e.Entity, &e.EntityID, &e.Action, &e.Detail, &e.TxTag, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = tm(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- 幂等键 ---

// GetIdempotency 查询幂等响应。
func (s *Store) GetIdempotency(ctx context.Context, scope, key string) (string, error) {
	var resp string
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT response FROM idempotency_keys WHERE scope=? AND key=?`, scope, key).Scan(&resp)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return resp, nil
}

// PutIdempotency 写入幂等响应。
func (s *Store) PutIdempotency(ctx context.Context, scope, key, response string, at time.Time) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO idempotency_keys(scope, key, response, created_at) VALUES(?,?,?,?)`,
		scope, key, response, ms(at))
	return err
}

// --- 后台作业 ---

// CreateJob 创建后台作业。
func (s *Store) CreateJob(ctx context.Context, j *domain.Job) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO jobs(id, kind, payload, status, attempts, max_attempts, run_at, last_error, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.Kind, j.Payload, j.Status, j.Attempts, j.MaxAttempts, ms(j.RunAt), j.LastError, ms(j.CreatedAt), ms(j.UpdatedAt))
	return err
}

// EnqueueIfAbsent 幂等入队：同 kind+payload 存在未完成作业时跳过。
func (s *Store) EnqueueIfAbsent(ctx context.Context, j *domain.Job) error {
	var n int
	if err := s.q(ctx).QueryRowContext(ctx,
		`SELECT COUNT(1) FROM jobs WHERE kind=? AND payload=? AND status IN (?,?,?)`,
		j.Kind, j.Payload, domain.JobPending, domain.JobRunning, domain.JobFailed).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	return s.CreateJob(ctx, j)
}

// GetJob 按 ID 查询作业。
func (s *Store) GetJob(ctx context.Context, id string) (*domain.Job, error) {
	j, err := scanJob(s.q(ctx).QueryRowContext(ctx,
		`SELECT id, kind, payload, status, attempts, max_attempts, run_at, last_error, created_at, updated_at
		 FROM jobs WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return j, nil
}

func scanJob(row interface{ Scan(...any) error }) (*domain.Job, error) {
	var j domain.Job
	var runAt, created, updated int64
	err := row.Scan(&j.ID, &j.Kind, &j.Payload, &j.Status, &j.Attempts, &j.MaxAttempts,
		&runAt, &j.LastError, &created, &updated)
	if err != nil {
		return nil, err
	}
	j.RunAt = tm(runAt)
	j.CreatedAt = tm(created)
	j.UpdatedAt = tm(updated)
	return &j, nil
}

const jobCols = `id, kind, payload, status, attempts, max_attempts, run_at, last_error, created_at, updated_at`

// DueJobs 查询到期可执行作业。载荷只携带业务参数，不能改变调度时间，
// 故此处仅按 run_at<=now 判断到期。
func (s *Store) DueJobs(ctx context.Context, now time.Time, limit int) ([]domain.Job, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+jobCols+` FROM jobs WHERE status=? AND run_at<=? ORDER BY run_at, id LIMIT ?`,
		domain.JobPending, ms(now), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// ClaimJob 抢占作业。
func (s *Store) ClaimJob(ctx context.Context, id string, now time.Time) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE jobs SET status=?, attempts=attempts+1, updated_at=? WHERE id=? AND status=?`,
		domain.JobRunning, ms(now), id, domain.JobPending)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// CompleteJob 完成作业。
func (s *Store) CompleteJob(ctx context.Context, id string, now time.Time) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`UPDATE jobs SET status=?, updated_at=? WHERE id=?`, domain.JobDone, ms(now), id)
	return err
}

// FailJob 失败处理：未超限置 FAILED 等待重试，否则 DEAD。
func (s *Store) FailJob(ctx context.Context, id, errMsg string, now time.Time) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`UPDATE jobs SET status=CASE WHEN attempts>=max_attempts THEN ? ELSE ? END,
		 last_error=?, updated_at=? WHERE id=?`,
		domain.JobDead, domain.JobFailed, errMsg, ms(now), id)
	return err
}

// RetryableJobs 查询可重试的失败作业。
func (s *Store) RetryableJobs(ctx context.Context) ([]domain.Job, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+jobCols+` FROM jobs WHERE status=? AND attempts<max_attempts ORDER BY updated_at, id`,
		domain.JobFailed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// RequeueJob 把失败作业重新排队为 PENDING。
func (s *Store) RequeueJob(ctx context.Context, id string, now time.Time) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE jobs SET status=?, run_at=?, updated_at=? WHERE id=? AND status=? AND attempts<max_attempts`,
		domain.JobPending, ms(now), ms(now), id, domain.JobFailed)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// ResetRunningJobs 重启恢复：遗留 RUNNING 作业重置为 PENDING。
func (s *Store) ResetRunningJobs(ctx context.Context, now time.Time) (int, error) {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE jobs SET status=?, updated_at=? WHERE status=?`, domain.JobPending, ms(now), domain.JobRunning)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
