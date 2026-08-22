package sqlite

import (
	"context"
	"database/sql"

	"gowork/wafer/internal/domain"
)

// CreateRecipe 新建配方。
func (s *Store) CreateRecipe(ctx context.Context, r *domain.Recipe) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO recipes(id, code, name, equipment_family, created_at) VALUES(?,?,?,?,?)`,
		r.ID, r.Code, r.Name, r.EquipmentFamily, ms(r.CreatedAt))
	return err
}

// GetRecipe 按 ID 查询配方。
func (s *Store) GetRecipe(ctx context.Context, id string) (*domain.Recipe, error) {
	var r domain.Recipe
	var created int64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT id, code, name, equipment_family, created_at FROM recipes WHERE id=?`, id).
		Scan(&r.ID, &r.Code, &r.Name, &r.EquipmentFamily, &created)
	if err != nil {
		return nil, notFound(err)
	}
	r.CreatedAt = tm(created)
	return &r, nil
}

// ListRecipes 列出全部配方。
func (s *Store) ListRecipes(ctx context.Context) ([]domain.Recipe, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT id, code, name, equipment_family, created_at FROM recipes ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Recipe
	for rows.Next() {
		var r domain.Recipe
		var created int64
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.EquipmentFamily, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = tm(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// NextVersionNumber 下一个配方版本号。
func (s *Store) NextVersionNumber(ctx context.Context, recipeID string) (int, error) {
	var n sql.NullInt64
	err := s.q(ctx).QueryRowContext(ctx,
		`SELECT MAX(version) FROM recipe_versions WHERE recipe_id=?`, recipeID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// CreateVersion 创建配方版本草稿。
func (s *Store) CreateVersion(ctx context.Context, v *domain.RecipeVersion) error {
	_, err := s.q(ctx).ExecContext(ctx,
		`INSERT INTO recipe_versions(id, recipe_id, version, status, params_json, snapshot, activated_at, row_version, created_at)
		 VALUES(?,?,?,?,?,'',NULL,1,?)`,
		v.ID, v.RecipeID, v.Version, v.Status, v.ParamsJSON, ms(v.CreatedAt))
	return err
}

func scanVersion(row interface{ Scan(...any) error }) (*domain.RecipeVersion, error) {
	var v domain.RecipeVersion
	var activated sql.NullInt64
	var created int64
	err := row.Scan(&v.ID, &v.RecipeID, &v.Version, &v.Status, &v.ParamsJSON,
		&v.Snapshot, &activated, &v.RowVersion, &created)
	if err != nil {
		return nil, err
	}
	v.ActivatedAt = tmPtr(activated)
	v.CreatedAt = tm(created)
	return &v, nil
}

const versionCols = `id, recipe_id, version, status, params_json, snapshot, activated_at, row_version, created_at`

// GetVersion 按 ID 查询配方版本。
func (s *Store) GetVersion(ctx context.Context, id string) (*domain.RecipeVersion, error) {
	v, err := scanVersion(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+versionCols+` FROM recipe_versions WHERE id=?`, id))
	if err != nil {
		return nil, notFound(err)
	}
	return v, nil
}

// ActiveVersion 查询配方当前启用版本。
func (s *Store) ActiveVersion(ctx context.Context, recipeID string) (*domain.RecipeVersion, error) {
	v, err := scanVersion(s.q(ctx).QueryRowContext(ctx,
		`SELECT `+versionCols+` FROM recipe_versions WHERE recipe_id=? AND status=? ORDER BY version DESC LIMIT 1`,
		recipeID, domain.RecipeActive))
	if err != nil {
		return nil, notFound(err)
	}
	return v, nil
}

// ListVersions 列出配方全部版本。
func (s *Store) ListVersions(ctx context.Context, recipeID string) ([]domain.RecipeVersion, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT `+versionCols+` FROM recipe_versions WHERE recipe_id=? ORDER BY version`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RecipeVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// ActivateVersion 启用版本：写入不可变快照并置为 ACTIVE，乐观锁保护。
func (s *Store) ActivateVersion(ctx context.Context, id, snapshot string, expectedVersion int, at int64) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE recipe_versions SET status=?, snapshot=?, activated_at=?, row_version=row_version+1
		 WHERE id=? AND row_version=?`,
		domain.RecipeActive, snapshot, at, id, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}

// UpdateVersionStatus 乐观锁更新版本状态。
func (s *Store) UpdateVersionStatus(ctx context.Context, id string, to domain.RecipeStatus, expectedVersion int) error {
	res, err := s.q(ctx).ExecContext(ctx,
		`UPDATE recipe_versions SET status=?, row_version=row_version+1 WHERE id=? AND row_version=?`,
		to, id, expectedVersion)
	if err != nil {
		return err
	}
	return conflictIfNoRows(res)
}
