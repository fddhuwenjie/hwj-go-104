package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// LotRepo 批次与晶圆谱系仓储。
type LotRepo interface {
	CreateLot(ctx context.Context, l *domain.Lot, wafers []domain.Wafer) error
	GetLot(ctx context.Context, id string) (*domain.Lot, error)
	FindLotByCode(ctx context.Context, code string) (*domain.Lot, error)
	// UpdateLot 乐观锁更新：expectedVersion 不匹配返回 ErrConflict。
	UpdateLot(ctx context.Context, l *domain.Lot, expectedVersion int) error
	ListLots(ctx context.Context, page domain.Page) ([]domain.Lot, error)

	CreateWafer(ctx context.Context, w *domain.Wafer) error
	GetWafer(ctx context.Context, id string) (*domain.Wafer, error)
	ListWafers(ctx context.Context, lotID string) ([]domain.Wafer, error)
	// MoveWafer 晶圆迁移：更新晶圆归属批次并写入迁移记录（同一事务）。
	MoveWafer(ctx context.Context, waferID, toLotID string, at int64) error
	WaferMoves(ctx context.Context, waferID string) ([]domain.WaferMove, error)
	// DescendantLotIDs 返回批次全部后代 ID（不含自身），用于暂扣阻断与谱系审计。
	DescendantLotIDs(ctx context.Context, lotID string) ([]string, error)
	// ChildLots 返回直接子批。
	ChildLots(ctx context.Context, lotID string) ([]domain.Lot, error)
}
