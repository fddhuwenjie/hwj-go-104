package sqlite

import (
	"context"
	"database/sql"

	"gowork/wafer/internal/domain"
)

// CreateRun 创建运行并关联晶圆。
func (s *Store) CreateRun(ctx context.Context, r *domain.Run, waferIDs []string) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO runs(id, lot_id, route_revision_id, station_seq, station_id, equipment_id, chamber_id,
		  recipe_version_id, recipe_snapshot, status, judgment, qual_covered, reviewed, started_at, completed_at, version, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,'NONE',1,0,?,NULL,1,?)`,
		r.ID, r.LotID, r.RouteRevisionID, r.StationSeq, r.StationID, r.EquipmentID, r.ChamberID,
		r.RecipeVersionID, r.RecipeSnapshot, r.Status, ms(r.StartedAt), ms(r.CreatedAt))
	if err != nil {
		return err
	}
	for _, wid := range waferIDs {
		if _, err := s.q(ctx).ExecContext(ctx,
			`INSERT INTO run_wafers(run_id, wafer_id) VALUES(?,?)`, r.ID, wid); err != nil {
			return err
		}
	}
	return nil
}

func scanRun(row interface{ Scan(...any) error }) (*domain.Run, error) {
	var r domain.Run
	var completed sql.NullInt64
	var started, created int64
	var qualCovered, reviewed int
	err := row.Scan(&r.ID, &r.LotID, &r.RouteRevisionID, &r.StationSeq, &r.StationID,
		&r.EquipmentID, &r.ChamberID, &r.RecipeVersionID, &r.RecipeSnapshot,
		&r.Status, &r.Judgment, &qualCovered, &reviewed, &started, &completed, &r.Version, &created)
	if err != nil {
		return nil, err
	}
	r.QualCovered = qualCovered != 0
	r.Reviewed = reviewed != 0
	r.StartedAt = tm(started)
	r.CompletedAt = tmPtr(completed)
	r.CreatedAt = tm(created)
	return &r, nil
}

const runCols = `id, lot_id, route_revision_id, station_seq, station_id, equipment_id, chamber_id,
  recipe_version_id, recipe_snapshot, status, judgment, qual_covered, reviewed, started_at, completed_at, version, created_at`

// GetRun 按 ID 查询运行。
func (s *Store) GetRun(ctx context.Context, id string) (*domain.Run, error) {
	r, err := scanRun(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+runCols+` FROM runs WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return r, nil
}

// UpdateRun 乐观锁更新运行。
func (s *Store) UpdateRun(ctx context.Context, r *domain.Run, expectedVersion int) error {
	qual := 0
	if r.QualCovered {
		qual = 1
	}
	reviewed := 0
	if r.Reviewed {
		reviewed = 1
	}
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE runs SET status=?, judgment=?, qual_covered=?, reviewed=?, completed_at=?, version=version+1
		 WHERE id=? AND version=?`,
		r.Status, r.Judgment, qual, reviewed, nullMs(r.CompletedAt), r.ID, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// ListRunsByLot 列出批次全部运行（按创建时间）。
func (s *Store) ListRunsByLot(ctx context.Context, lotID string) ([]domain.Run, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+runCols+` FROM runs WHERE lot_id=? ORDER BY created_at, id`, lotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// RunWafers 返回运行关联的晶圆 ID。
func (s *Store) RunWafers(ctx context.Context, runID string) ([]string, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT wafer_id FROM run_wafers WHERE run_id=? ORDER BY wafer_id`, runID)
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

// BusyWaferIDs 返回当前处于 RUNNING 运行中的晶圆集合。
func (s *Store) BusyWaferIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT DISTINCT rw.wafer_id FROM run_wafers rw
		 JOIN runs r ON r.id = rw.run_id WHERE r.status=?`, domain.RunRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// RunningRuns 返回全部运行中的运行。
func (s *Store) RunningRuns(ctx context.Context) ([]domain.Run, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+runCols+` FROM runs WHERE status=? ORDER BY started_at`, domain.RunRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// HasRunAtStation 判断批次在指定修订的站点顺序号是否已有非中止运行。
func (s *Store) HasRunAtStation(ctx context.Context, lotID, revisionID string, seq int) (bool, error) {
	var n int
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT COUNT(1) FROM runs WHERE lot_id=? AND route_revision_id=? AND station_seq=? AND status<>?`,
		lotID, revisionID, seq, domain.RunAborted).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
