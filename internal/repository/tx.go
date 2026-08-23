package repository

import (
	"context"
)

// TxManager 事务管理器：fn 内的所有 Repository 调用共享同一事务，
// fn 返回错误则整体回滚。
type TxManager interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Store 聚合全部仓储接口，由 SQLite 实现提供。
type Store interface {
	TxManager
	MasterRepo
	RouteRepo
	RecipeRepo
	EquipmentRepo
	MetrologyRepo
	LotRepo
	RunRepo
	ReadingRepo
	HoldRepo
	ReleaseRepo
	ReworkRepo
	AuditRepo
	IdempotencyRepo
	JobRepo
	QueryRepo
}
