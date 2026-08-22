package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// ReworkRepo 返工记录仓储。
type ReworkRepo interface {
	CreateReworkRecord(ctx context.Context, r *domain.ReworkRecord) error
	ListReworkRecords(ctx context.Context, lotID string) ([]domain.ReworkRecord, error)
}
