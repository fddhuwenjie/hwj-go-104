package sqlite

import (
	"context"
	"database/sql"

	"gowork/wafer/internal/domain"
)

// CreateHold 创建暂扣。
func (s *Store) CreateHold(ctx context.Context, h *domain.Hold) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO holds(id, lot_id, run_id, reason, status, escalated, review_note, version, created_at, closed_at)
		 VALUES(?,?,?,?,?,0,'',1,?,NULL)`,
		h.ID, h.LotID, h.RunID, h.Reason, h.Status, ms(h.CreatedAt))
	return err
}

func scanHold(row interface{ Scan(...any) error }) (*domain.Hold, error) {
	var h domain.Hold
	var escalated int
	var closed sql.NullInt64
	var created int64
	err := row.Scan(&h.ID, &h.LotID, &h.RunID, &h.Reason, &h.Status, &escalated,
		&h.ReviewNote, &h.Version, &created, &closed)
	if err != nil {
		return nil, err
	}
	h.Escalated = escalated != 0
	h.CreatedAt = tm(created)
	h.ClosedAt = tmPtr(closed)
	return &h, nil
}

const holdCols = `id, lot_id, run_id, reason, status, escalated, review_note, version, created_at, closed_at`

// GetHold 按 ID 查询暂扣。
func (s *Store) GetHold(ctx context.Context, id string) (*domain.Hold, error) {
	h, err := scanHold(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+holdCols+` FROM holds WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return h, nil
}

// UpdateHold 乐观锁更新暂扣。
func (s *Store) UpdateHold(ctx context.Context, h *domain.Hold, expectedVersion int) error {
	esc := 0
	if h.Escalated {
		esc = 1
	}
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE holds SET status=?, escalated=?, review_note=?, closed_at=?, version=version+1
		 WHERE id=? AND version=?`,
		h.Status, esc, h.ReviewNote, nullMs(h.ClosedAt), h.ID, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// HoldsForLots 查询批次集合上的全部暂扣。
func (s *Store) HoldsForLots(ctx context.Context, lotIDs []string) ([]domain.Hold, error) {
	if len(lotIDs) == 0 {
		return nil, nil
	}
	args := make([]any, len(lotIDs))
	for i, id := range lotIDs {
		args[i] = id
	}
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+holdCols+` FROM holds WHERE lot_id IN `+placeholders(len(lotIDs))+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHoldRows(rows)
}

// LatestHold 查询批次最近一条暂扣。
func (s *Store) LatestHold(ctx context.Context, lotID string) (*domain.Hold, error) {
	h, err := scanHold(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+holdCols+` FROM holds WHERE lot_id=? ORDER BY created_at DESC, id DESC LIMIT 1`, lotID))
	if err != nil {
		return nil, notFound(err)
	}
	return h, nil
}

// ListOpenHolds 列出全部未关闭暂扣。
func (s *Store) ListOpenHolds(ctx context.Context) ([]domain.Hold, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+holdCols+` FROM holds WHERE status=? ORDER BY created_at, id`, domain.HoldOpen)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHoldRows(rows)
}

func scanHoldRows(rows *sql.Rows) ([]domain.Hold, error) {
	var out []domain.Hold
	for rows.Next() {
		h, err := scanHold(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}
