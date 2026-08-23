package sqlite

import (
	"context"
	"database/sql"

	"gowork/wafer/internal/domain"
)

// CreateRoute 新建工艺路线。
func (s *Store) CreateRoute(ctx context.Context, r *domain.Route) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO routes(id, product_family_id, code, name, created_at) VALUES(?,?,?,?,?)`,
		r.ID, r.ProductFamilyID, r.Code, r.Name, ms(r.CreatedAt))
	return err
}

// GetRoute 按 ID 查询路线。
func (s *Store) GetRoute(ctx context.Context, id string) (*domain.Route, error) {
	var r domain.Route
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, product_family_id, code, name, created_at FROM routes WHERE id=?`, id).
		Scan(&r.ID, &r.ProductFamilyID, &r.Code, &r.Name, &created)
	if err != nil {
		return nil, notFound(err)
	}
	r.CreatedAt = tm(created)
	return &r, nil
}

// ListRoutes 按产品族列出路线（空串表示全部）。
func (s *Store) ListRoutes(ctx context.Context, productFamilyID string) ([]domain.Route, error) {
	query := `SELECT id, product_family_id, code, name, created_at FROM routes`
	args := []any{}
	if productFamilyID != "" {
		query += ` WHERE product_family_id=?`
		args = append(args, productFamilyID)
	}
	query += ` ORDER BY created_at, id`
	rows, err := s.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Route
	for rows.Next() {
		var r domain.Route
		var created int64
		if err := rows.Scan(&r.ID, &r.ProductFamilyID, &r.Code, &r.Name, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = tm(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// NextRevisionNumber 下一个修订号。
func (s *Store) NextRevisionNumber(ctx context.Context, routeID string) (int, error) {
	var n sql.NullInt64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT MAX(revision) FROM route_revisions WHERE route_id=?`, routeID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// CreateRevision 创建修订及其站点序列。
func (s *Store) CreateRevision(ctx context.Context, rev *domain.RouteRevision, stations []domain.RouteStation) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO route_revisions(id, route_id, revision, status, rework_from_hold_id, reentry_seq, version, created_at)
		 VALUES(?,?,?,?,?,?,1,?)`,
		rev.ID, rev.RouteID, rev.Revision, rev.Status, rev.ReworkFromHoldID, rev.ReentrySeq, ms(rev.CreatedAt))
	if err != nil {
		return err
	}
	for _, st := range stations {
		_, err := s.q(ctx).ExecContext(ctx,
			`INSERT INTO route_stations(id, route_revision_id, seq, station_id, recipe_id, metrology_plan_id)
			 VALUES(?,?,?,?,?,?)`,
			st.ID, rev.ID, st.Seq, st.StationID, st.RecipeID, st.MetrologyPlanID)
		if err != nil {
			return err
		}
	}
	return nil
}

func scanRevision(row interface{ Scan(...any) error }) (*domain.RouteRevision, error) {
	var rev domain.RouteRevision
	var created int64
	err := row.Scan(&rev.ID, &rev.RouteID, &rev.Revision, &rev.Status,
		&rev.ReworkFromHoldID, &rev.ReentrySeq, &rev.Version, &created)
	if err != nil {
		return nil, err
	}
	rev.CreatedAt = tm(created)
	return &rev, nil
}

const revisionCols = `id, route_id, revision, status, rework_from_hold_id, reentry_seq, version, created_at`

// GetRevision 按 ID 查询修订。
func (s *Store) GetRevision(ctx context.Context, id string) (*domain.RouteRevision, error) {
	rev, err := scanRevision(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+revisionCols+` FROM route_revisions WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return rev, nil
}

// ActiveRevision 查询路线当前启用修订。
func (s *Store) ActiveRevision(ctx context.Context, routeID string) (*domain.RouteRevision, error) {
	rev, err := scanRevision(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+revisionCols+` FROM route_revisions WHERE route_id=? AND status=? ORDER BY revision DESC LIMIT 1`,
		routeID, domain.RevActive))
	if err != nil {
		return nil, notFound(err)
	}
	return rev, nil
}

// ListRevisions 列出路线全部修订。
func (s *Store) ListRevisions(ctx context.Context, routeID string) ([]domain.RouteRevision, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+revisionCols+` FROM route_revisions WHERE route_id=? ORDER BY revision`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RouteRevision
	for rows.Next() {
		rev, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rev)
	}
	return out, rows.Err()
}

// UpdateRevisionStatus 乐观锁更新修订状态。
func (s *Store) UpdateRevisionStatus(ctx context.Context, id string, to domain.RevisionStatus, expectedVersion int) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE route_revisions SET status=?, version=version+1 WHERE id=? AND version=?`,
		to, id, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// ListRouteStations 按顺序列出修订站点。
func (s *Store) ListRouteStations(ctx context.Context, revisionID string) ([]domain.RouteStation, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, route_revision_id, seq, station_id, recipe_id, metrology_plan_id
		 FROM route_stations WHERE route_revision_id=? ORDER BY seq`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RouteStation
	for rows.Next() {
		var st domain.RouteStation
		if err := rows.Scan(&st.ID, &st.RouteRevisionID, &st.Seq, &st.StationID, &st.RecipeID, &st.MetrologyPlanID); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
