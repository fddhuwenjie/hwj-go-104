package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// RouteRepo 工艺路线仓储。
type RouteRepo interface {
	CreateRoute(ctx context.Context, r *domain.Route) error
	GetRoute(ctx context.Context, id string) (*domain.Route, error)
	ListRoutes(ctx context.Context, productFamilyID string) ([]domain.Route, error)

	// NextRevisionNumber 返回路线的下一个修订号。
	NextRevisionNumber(ctx context.Context, routeID string) (int, error)
	CreateRevision(ctx context.Context, rev *domain.RouteRevision, stations []domain.RouteStation) error
	GetRevision(ctx context.Context, id string) (*domain.RouteRevision, error)
	// ActiveRevision 返回路线当前启用修订，无则 ErrNotFound。
	ActiveRevision(ctx context.Context, routeID string) (*domain.RouteRevision, error)
	ListRevisions(ctx context.Context, routeID string) ([]domain.RouteRevision, error)
	// UpdateRevisionStatus 乐观锁状态更新。
	UpdateRevisionStatus(ctx context.Context, id string, to domain.RevisionStatus, expectedVersion int) error
	ListRouteStations(ctx context.Context, revisionID string) ([]domain.RouteStation, error)
}
