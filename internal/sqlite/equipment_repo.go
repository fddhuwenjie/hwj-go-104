package sqlite

import (
	"context"

	"gowork/wafer/internal/domain"
)

// CreateEquipment 新建设备。
func (s *Store) CreateEquipment(ctx context.Context, e *domain.Equipment) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO equipment(id, code, name, family, station_id, status, version, created_at) VALUES(?,?,?,?,?,?,1,?)`,
		e.ID, e.Code, e.Name, e.Family, e.StationID, e.Status, ms(e.CreatedAt))
	return err
}

// GetEquipment 按 ID 查询设备。
func (s *Store) GetEquipment(ctx context.Context, id string) (*domain.Equipment, error) {
	var e domain.Equipment
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, code, name, family, station_id, status, version, created_at FROM equipment WHERE id=?`, id).
		Scan(&e.ID, &e.Code, &e.Name, &e.Family, &e.StationID, &e.Status, &e.Version, &created)
	if err != nil {
		return nil, notFound(err)
	}
	e.CreatedAt = tm(created)
	return &e, nil
}

// ListEquipment 列出全部设备。
func (s *Store) ListEquipment(ctx context.Context) ([]domain.Equipment, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, code, name, family, station_id, status, version, created_at FROM equipment ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Equipment
	for rows.Next() {
		var e domain.Equipment
		var created int64
		if err := rows.Scan(&e.ID, &e.Code, &e.Name, &e.Family, &e.StationID, &e.Status, &e.Version, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = tm(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateEquipmentStatus 乐观锁更新设备状态：version 不匹配返回 ErrConflict。
func (s *Store) UpdateEquipmentStatus(ctx context.Context, id string, to domain.EquipmentStatus, expectedVersion int) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE equipment SET status=?, version=version+1 WHERE id=? AND version=?`,
		to, id, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// CreateChamber 新建腔体。
func (s *Store) CreateChamber(ctx context.Context, c *domain.Chamber) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO chambers(id, equipment_id, code, capability, status, created_at) VALUES(?,?,?,?,?,?)`,
		c.ID, c.EquipmentID, c.Code, c.Capability, c.Status, ms(c.CreatedAt))
	return err
}

// GetChamber 按 ID 查询腔体。
func (s *Store) GetChamber(ctx context.Context, id string) (*domain.Chamber, error) {
	var c domain.Chamber
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, equipment_id, code, capability, status, created_at FROM chambers WHERE id=?`, id).
		Scan(&c.ID, &c.EquipmentID, &c.Code, &c.Capability, &c.Status, &created)
	if err != nil {
		return nil, notFound(err)
	}
	c.CreatedAt = tm(created)
	return &c, nil
}

// ListChambers 列出设备全部腔体。
func (s *Store) ListChambers(ctx context.Context, equipmentID string) ([]domain.Chamber, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, equipment_id, code, capability, status, created_at FROM chambers WHERE equipment_id=? ORDER BY code`,
		equipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Chamber
	for rows.Next() {
		var c domain.Chamber
		var created int64
		if err := rows.Scan(&c.ID, &c.EquipmentID, &c.Code, &c.Capability, &c.Status, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = tm(created)
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateQualification 新建资质窗口。
func (s *Store) CreateQualification(ctx context.Context, q *domain.Qualification) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO qualifications(id, equipment_id, chamber_id, station_id, valid_from, valid_to, status, created_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		q.ID, q.EquipmentID, q.ChamberID, q.StationID, ms(q.ValidFrom), ms(q.ValidTo), q.Status, ms(q.CreatedAt))
	return err
}

// GetQualification 按 ID 查询资质。
func (s *Store) GetQualification(ctx context.Context, id string) (*domain.Qualification, error) {
	var q domain.Qualification
	var from, to, created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, equipment_id, chamber_id, station_id, valid_from, valid_to, status, created_at
		 FROM qualifications WHERE id=?`, id).
		Scan(&q.ID, &q.EquipmentID, &q.ChamberID, &q.StationID, &from, &to, &q.Status, &created)
	if err != nil {
		return nil, notFound(err)
	}
	q.ValidFrom, q.ValidTo, q.CreatedAt = tm(from), tm(to), tm(created)
	return &q, nil
}

// QualificationsFor 查询设备在站点上的全部资质。
func (s *Store) QualificationsFor(ctx context.Context, equipmentID, stationID string) ([]domain.Qualification, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, equipment_id, chamber_id, station_id, valid_from, valid_to, status, created_at
		 FROM qualifications WHERE equipment_id=? AND station_id=? ORDER BY valid_from`, equipmentID, stationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQualifications(rows)
}

// ListQualifications 列出全部资质。
func (s *Store) ListQualifications(ctx context.Context) ([]domain.Qualification, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, equipment_id, chamber_id, station_id, valid_from, valid_to, status, created_at
		 FROM qualifications ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQualifications(rows)
}

func scanQualifications(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]domain.Qualification, error) {
	var out []domain.Qualification
	for rows.Next() {
		var q domain.Qualification
		var from, to, created int64
		if err := rows.Scan(&q.ID, &q.EquipmentID, &q.ChamberID, &q.StationID, &from, &to, &q.Status, &created); err != nil {
			return nil, err
		}
		q.ValidFrom, q.ValidTo, q.CreatedAt = tm(from), tm(to), tm(created)
		out = append(out, q)
	}
	return out, rows.Err()
}

// UpdateQualificationStatus 更新资质状态（到期扫描置为 REVOKED）。
func (s *Store) UpdateQualificationStatus(ctx context.Context, id string, to domain.QualStatus) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`UPDATE qualifications SET status=? WHERE id=?`, to, id)
	return err
}
