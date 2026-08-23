package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// ReadingRepo 晶圆位置读数仓储。
type ReadingRepo interface {
	CreateReading(ctx context.Context, r *domain.Reading) error
	ListReadings(ctx context.Context, runID string) ([]domain.Reading, error)
	// SealReadings 封存运行的全部读数（不可逆）。
	SealReadings(ctx context.Context, runID string) error
}
