package sqlite

import (
	"context"

	"gowork/wafer/internal/domain"
)

// CreateReading 写入一条晶圆位置读数。
// (run_id, wafer_id) 受 run_wafers 复合外键约束：晶圆必须实际参与目标运行，
// 否则外键校验拒绝写入，保证读数与运行成员一致、谱系可追溯。
func (s *Store) CreateReading(ctx context.Context, r *domain.Reading) error {
	if r.RunID == "" || r.WaferID == "" {
		return domain.NewValidationError(domain.FieldError{Field: "run_id/wafer_id", Message: "运行与晶圆不能为空"})
	}
	late, sealed := 0, 0
	if r.Late {
		late = 1
	}
	if r.Sealed {
		sealed = 1
	}
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO readings(id, run_id, wafer_id, slot, metric, value, late, sealed, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		r.ID, r.RunID, r.WaferID, r.Slot, r.Metric, r.Value, late, sealed, ms(r.CreatedAt))
	return err
}

// ListReadings 列出运行全部读数。
func (s *Store) ListReadings(ctx context.Context, runID string) ([]domain.Reading, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, run_id, wafer_id, slot, metric, value, late, sealed, created_at
		 FROM readings WHERE run_id=? ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Reading
	for rows.Next() {
		var r domain.Reading
		var late, sealed int
		var created int64
		if err := rows.Scan(&r.ID, &r.RunID, &r.WaferID, &r.Slot, &r.Metric, &r.Value, &late, &sealed, &created); err != nil {
			return nil, err
		}
		r.Late = late != 0
		r.Sealed = sealed != 0
		r.CreatedAt = tm(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SealReadings 封存运行全部读数。
func (s *Store) SealReadings(ctx context.Context, runID string) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`UPDATE readings SET sealed=1 WHERE run_id=?`, runID)
	return err
}
