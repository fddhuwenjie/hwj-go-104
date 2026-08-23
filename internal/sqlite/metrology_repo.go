package sqlite

import (
	"context"
	"database/sql"

	"gowork/wafer/internal/domain"
)

// CreatePlan 新建量测计划。
func (s *Store) CreatePlan(ctx context.Context, p *domain.MetrologyPlan) error {
	positions, err := marshalJSON(p.SamplePositions)
	if err != nil {
		return err
	}
	_, err = s.q(ctx).ExecContext(ctx,
		`INSERT INTO metrology_plans(id, code, name, version, status, sample_positions, min_samples, pass_limit, metric, row_version, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,1,?)`,
		p.ID, p.Code, p.Name, p.Version, p.Status, positions, p.MinSamples, p.PassLimit, p.Metric, ms(p.CreatedAt))
	return err
}

func scanPlan(row interface{ Scan(...any) error }) (*domain.MetrologyPlan, error) {
	var p domain.MetrologyPlan
	var positions string
	var created int64
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.Version, &p.Status, &positions,
		&p.MinSamples, &p.PassLimit, &p.Metric, &p.RowVersion, &created); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(positions, &p.SamplePositions); err != nil {
		return nil, err
	}
	p.CreatedAt = tm(created)
	return &p, nil
}

const planCols = `id, code, name, version, status, sample_positions, min_samples, pass_limit, metric, row_version, created_at`

// GetPlan 按 ID 查询量测计划。
func (s *Store) GetPlan(ctx context.Context, id string) (*domain.MetrologyPlan, error) {
	p, err := scanPlan(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+planCols+` FROM metrology_plans WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return p, nil
}

// ListPlans 列出全部量测计划。
func (s *Store) ListPlans(ctx context.Context) ([]domain.MetrologyPlan, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+planCols+` FROM metrology_plans ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.MetrologyPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// NextPlanVersion 同一编码计划的下一个版本号。
func (s *Store) NextPlanVersion(ctx context.Context, code string) (int, error) {
	var n sql.NullInt64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT MAX(version) FROM metrology_plans WHERE code=?`, code).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// UpdatePlanStatus 乐观锁更新计划状态。
// 必须复用调用方事务（s.q(ctx)）：退役旧计划、启用新计划、写入审计
// 三者须在 InTx 的同一事务内整体提交；若退役写入改用 context.Background()
// 会逃逸到 s.db 立即提交，审计失败时旧计划已被退役而新计划仍是草稿，
// 生产线将找不到有效计划。
func (s *Store) UpdatePlanStatus(ctx context.Context, id string, to domain.PlanStatus, expectedVersion int) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE metrology_plans SET status=?, row_version=row_version+1 WHERE id=? AND row_version=?`,
		to, id, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}
