package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// RunRepo 制程运行仓储。
type RunRepo interface {
	CreateRun(ctx context.Context, r *domain.Run, waferIDs []string) error
	GetRun(ctx context.Context, id string) (*domain.Run, error)
	// UpdateRun 乐观锁更新。
	UpdateRun(ctx context.Context, r *domain.Run, expectedVersion int) error
	ListRunsByLot(ctx context.Context, lotID string) ([]domain.Run, error)
	RunWafers(ctx context.Context, runID string) ([]string, error)
	// BusyWaferIDs 返回当前处于 RUNNING 运行中的晶圆 ID 集合。
	BusyWaferIDs(ctx context.Context) (map[string]bool, error)
	// RunningRuns 返回全部运行中的运行（超时检查用）。
	RunningRuns(ctx context.Context) ([]domain.Run, error)
	// HasRunAtStation 判断批次在指定修订的站点顺序号是否已有非中止运行。
	HasRunAtStation(ctx context.Context, lotID, revisionID string, seq int) (bool, error)
}
