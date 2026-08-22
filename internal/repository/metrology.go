package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// MetrologyRepo 量测计划仓储。
type MetrologyRepo interface {
	CreatePlan(ctx context.Context, p *domain.MetrologyPlan) error
	GetPlan(ctx context.Context, id string) (*domain.MetrologyPlan, error)
	ListPlans(ctx context.Context) ([]domain.MetrologyPlan, error)
	NextPlanVersion(ctx context.Context, code string) (int, error)
	UpdatePlanStatus(ctx context.Context, id string, to domain.PlanStatus, expectedVersion int) error
}
