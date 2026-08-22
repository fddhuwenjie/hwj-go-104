package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// MasterRepo 主数据仓储：产品族与站点。
type MasterRepo interface {
	CreateProductFamily(ctx context.Context, p *domain.ProductFamily) error
	GetProductFamily(ctx context.Context, id string) (*domain.ProductFamily, error)
	FindProductFamilyByCode(ctx context.Context, code string) (*domain.ProductFamily, error)
	ListProductFamilies(ctx context.Context) ([]domain.ProductFamily, error)

	CreateStation(ctx context.Context, s *domain.Station) error
	GetStation(ctx context.Context, id string) (*domain.Station, error)
	ListStations(ctx context.Context) ([]domain.Station, error)
}
