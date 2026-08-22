package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// HoldRepo 暂扣仓储。
type HoldRepo interface {
	CreateHold(ctx context.Context, h *domain.Hold) error
	GetHold(ctx context.Context, id string) (*domain.Hold, error)
	// UpdateHold 乐观锁更新。
	UpdateHold(ctx context.Context, h *domain.Hold, expectedVersion int) error
	// HoldsForLots 返回给定批次集合上的全部暂扣。
	HoldsForLots(ctx context.Context, lotIDs []string) ([]domain.Hold, error)
	// LatestHold 返回批次最近一条暂扣。
	LatestHold(ctx context.Context, lotID string) (*domain.Hold, error)
	// ListOpenHolds 返回全部未关闭暂扣（升级扫描用）。
	ListOpenHolds(ctx context.Context) ([]domain.Hold, error)
}
