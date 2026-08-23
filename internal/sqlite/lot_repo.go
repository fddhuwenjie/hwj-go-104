package sqlite

import (
	"context"
	"database/sql"

	"gowork/wafer/internal/domain"
)

// CreateLot 登记批次并写入晶圆清单（同一事务由调用方保证）。
func (s *Store) CreateLot(ctx context.Context, l *domain.Lot, wafers []domain.Wafer) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO lots(id, code, product_family_id, route_id, status, current_seq,
		  frozen_revision_id, freeze_snapshot, frozen_at, parent_lot_id, entered_at, version, created_at, closed_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,1,?,NULL)`,
		l.ID, l.Code, l.ProductFamilyID, l.RouteID, l.Status, l.CurrentSeq,
		l.FrozenRevisionID, l.FreezeSnapshot, nullMs(l.FrozenAt), l.ParentLotID, nullMs(l.EnteredAt), ms(l.CreatedAt))
	if err != nil {
		return err
	}
	for i := range wafers {
		w := wafers[i]
		w.LotID = l.ID
		if err := s.CreateWafer(ctx, &w); err != nil {
			return err
		}
	}
	return nil
}

func scanLot(row interface{ Scan(...any) error }) (*domain.Lot, error) {
	var l domain.Lot
	var frozenAt, closedAt, enteredAt sql.NullInt64
	var created int64
	err := row.Scan(&l.ID, &l.Code, &l.ProductFamilyID, &l.RouteID, &l.Status, &l.CurrentSeq,
		&l.FrozenRevisionID, &l.FreezeSnapshot, &frozenAt, &l.ParentLotID, &enteredAt, &l.Version, &created, &closedAt)
	if err != nil {
		return nil, err
	}
	l.FrozenAt = tmPtr(frozenAt)
	l.ClosedAt = tmPtr(closedAt)
	l.EnteredAt = tmPtr(enteredAt)
	l.CreatedAt = tm(created)
	return &l, nil
}

const lotCols = `id, code, product_family_id, route_id, status, current_seq,
  frozen_revision_id, freeze_snapshot, frozen_at, parent_lot_id, entered_at, version, created_at, closed_at`

// GetLot 按 ID 查询批次。
func (s *Store) GetLot(ctx context.Context, id string) (*domain.Lot, error) {
	l, err := scanLot(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+lotCols+` FROM lots WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return l, nil
}

// FindLotByCode 按编码查询批次。
func (s *Store) FindLotByCode(ctx context.Context, code string) (*domain.Lot, error) {
	l, err := scanLot(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+lotCols+` FROM lots WHERE code=?`, code))
	if err != nil {
		return nil, notFound(err)
	}
	return l, nil
}

// UpdateLot 乐观锁更新批次：version 不匹配返回 ErrConflict。
func (s *Store) UpdateLot(ctx context.Context, l *domain.Lot, expectedVersion int) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE lots SET status=?, current_seq=?, frozen_revision_id=?, freeze_snapshot=?,
		  frozen_at=?, parent_lot_id=?, entered_at=?, closed_at=?, version=version+1
		 WHERE id=? AND version=?`,
		l.Status, l.CurrentSeq, l.FrozenRevisionID, l.FreezeSnapshot,
		nullMs(l.FrozenAt), l.ParentLotID, nullMs(l.EnteredAt), nullMs(l.ClosedAt), l.ID, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// ListLots 稳定分页列出批次（按 created_at, id）。
func (s *Store) ListLots(ctx context.Context, page domain.Page) ([]domain.Lot, error) {
	page = page.Normalize()
	key, err := domain.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	query := `SELECT ` + lotCols + ` FROM lots`
	args := []any{}
	if page.Cursor != "" {
		query += ` WHERE (created_at > ?) OR (created_at = ? AND id > ?)`
		args = append(args, ms(key.CreatedAt), ms(key.CreatedAt), key.ID)
	}
	query += ` ORDER BY created_at, id LIMIT ?`
	args = append(args, page.Limit)
	rows, err := s.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Lot
	for rows.Next() {
		l, err := scanLot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

// CreateWafer 写入单片晶圆。
func (s *Store) CreateWafer(ctx context.Context, w *domain.Wafer) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO wafers(id, code, lot_id, slot, status, created_at) VALUES(?,?,?,?,?,?)`,
		w.ID, w.Code, w.LotID, w.Slot, w.Status, ms(w.CreatedAt))
	return err
}

// GetWafer 按 ID 查询晶圆。
func (s *Store) GetWafer(ctx context.Context, id string) (*domain.Wafer, error) {
	var w domain.Wafer
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, code, lot_id, slot, status, created_at FROM wafers WHERE id=?`, id).
		Scan(&w.ID, &w.Code, &w.LotID, &w.Slot, &w.Status, &created)
	if err != nil {
		return nil, notFound(err)
	}
	w.CreatedAt = tm(created)
	return &w, nil
}

// ListWafers 列出批次晶圆（按槽位）。
func (s *Store) ListWafers(ctx context.Context, lotID string) ([]domain.Wafer, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, code, lot_id, slot, status, created_at FROM wafers WHERE lot_id=? ORDER BY slot`, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Wafer
	for rows.Next() {
		var w domain.Wafer
		var created int64
		if err := rows.Scan(&w.ID, &w.Code, &w.LotID, &w.Slot, &w.Status, &created); err != nil {
			return nil, err
		}
		w.CreatedAt = tm(created)
		out = append(out, w)
	}
	return out, rows.Err()
}

// MoveWafer 晶圆迁移：更新归属并写入不可变迁移记录。
func (s *Store) MoveWafer(ctx context.Context, waferID, toLotID string, at int64) error {
	w, err := s.GetWafer(ctx, waferID)
	if err != nil {
		return err
	}
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE wafers SET lot_id=? WHERE id=? AND lot_id=?`, toLotID, waferID, w.LotID)
	if err != nil {
		return err
	}
	if err := conflictIfNoRows(res); err != nil {
		return err
	}
	_, err = s.q(ctx).ExecContext(ctx,
		`INSERT INTO wafer_moves(id, wafer_id, from_lot_id, to_lot_id, created_at) VALUES(?,?,?,?,?)`,
		domain.NewID(domain.IDPrefixWafer), waferID, w.LotID, toLotID, at)
	return err
}

// WaferMoves 查询晶圆迁移历史。
func (s *Store) WaferMoves(ctx context.Context, waferID string) ([]domain.WaferMove, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, wafer_id, from_lot_id, to_lot_id, created_at FROM wafer_moves WHERE wafer_id=? ORDER BY created_at, id`,
		waferID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.WaferMove
	for rows.Next() {
		var m domain.WaferMove
		var created int64
		if err := rows.Scan(&m.ID, &m.WaferID, &m.FromLotID, &m.ToLotID, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = tm(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// DescendantLotIDs 递归查询全部后代批次 ID。
func (s *Store) DescendantLotIDs(ctx context.Context, lotID string) ([]string, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`WITH RECURSIVE desc(id) AS (
		   SELECT id FROM lots WHERE parent_lot_id=?
		   UNION ALL
		   SELECT l.id FROM lots l JOIN desc d ON l.parent_lot_id=d.id
		 ) SELECT id FROM desc`, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ChildLots 查询直接子批。
func (s *Store) ChildLots(ctx context.Context, lotID string) ([]domain.Lot, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+lotCols+` FROM lots WHERE parent_lot_id=? ORDER BY created_at, id`, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Lot
	for rows.Next() {
		l, err := scanLot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}
