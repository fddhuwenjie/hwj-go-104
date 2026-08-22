package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// RecipeRepo 配方仓储。
type RecipeRepo interface {
	CreateRecipe(ctx context.Context, r *domain.Recipe) error
	GetRecipe(ctx context.Context, id string) (*domain.Recipe, error)
	ListRecipes(ctx context.Context) ([]domain.Recipe, error)

	NextVersionNumber(ctx context.Context, recipeID string) (int, error)
	CreateVersion(ctx context.Context, v *domain.RecipeVersion) error
	GetVersion(ctx context.Context, id string) (*domain.RecipeVersion, error)
	ActiveVersion(ctx context.Context, recipeID string) (*domain.RecipeVersion, error)
	ListVersions(ctx context.Context, recipeID string) ([]domain.RecipeVersion, error)
	// ActivateVersion 启用版本并写入不可变快照，乐观锁保护。
	ActivateVersion(ctx context.Context, id, snapshot string, expectedVersion int, at int64) error
	UpdateVersionStatus(ctx context.Context, id string, to domain.RecipeStatus, expectedVersion int) error
}
