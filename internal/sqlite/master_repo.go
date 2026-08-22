package sqlite

import (
	"context"

	"gowork/wafer/internal/domain"
)

// CreateProductFamily 新建产品族。
func (s *Store) CreateProductFamily(ctx context.Context, p *domain.ProductFamily) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO product_families(id, code, name, created_at) VALUES(?,?,?,?)`,
		p.ID, p.Code, p.Name, ms(p.CreatedAt))
	return err
}

// GetProductFamily 按 ID 查询产品族。
func (s *Store) GetProductFamily(ctx context.Context, id string) (*domain.ProductFamily, error) {
	var p domain.ProductFamily
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, code, name, created_at FROM product_families WHERE id=?`, id).
		Scan(&p.ID, &p.Code, &p.Name, &created)
	if err != nil {
		return nil, notFound(err)
	}
	p.CreatedAt = tm(created)
	return &p, nil
}

// FindProductFamilyByCode 按编码查询产品族。
func (s *Store) FindProductFamilyByCode(ctx context.Context, code string) (*domain.ProductFamily, error) {
	var p domain.ProductFamily
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, code, name, created_at FROM product_families WHERE code=?`, code).
		Scan(&p.ID, &p.Code, &p.Name, &created)
	if err != nil {
		return nil, notFound(err)
	}
	p.CreatedAt = tm(created)
	return &p, nil
}

// ListProductFamilies 列出全部产品族。
func (s *Store) ListProductFamilies(ctx context.Context) ([]domain.ProductFamily, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, code, name, created_at FROM product_families ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ProductFamily
	for rows.Next() {
		var p domain.ProductFamily
		var created int64
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = tm(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateStation 新建站点。
func (s *Store) CreateStation(ctx context.Context, st *domain.Station) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO stations(id, code, name, capability, created_at) VALUES(?,?,?,?,?)`,
		st.ID, st.Code, st.Name, st.Capability, ms(st.CreatedAt))
	return err
}

// GetStation 按 ID 查询站点。
func (s *Store) GetStation(ctx context.Context, id string) (*domain.Station, error) {
	var st domain.Station
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, code, name, capability, created_at FROM stations WHERE id=?`, id).
		Scan(&st.ID, &st.Code, &st.Name, &st.Capability, &created)
	if err != nil {
		return nil, notFound(err)
	}
	st.CreatedAt = tm(created)
	return &st, nil
}

// ListStations 列出全部站点。
func (s *Store) ListStations(ctx context.Context) ([]domain.Station, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, code, name, capability, created_at FROM stations ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Station
	for rows.Next() {
		var st domain.Station
		var created int64
		if err := rows.Scan(&st.ID, &st.Code, &st.Name, &st.Capability, &created); err != nil {
			return nil, err
		}
		st.CreatedAt = tm(created)
		out = append(out, st)
	}
	return out, rows.Err()
}
