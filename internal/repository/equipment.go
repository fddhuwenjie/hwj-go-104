package repository

import (
	"context"

	"gowork/wafer/internal/domain"
)

// EquipmentRepo 设备、腔体与资质仓储。
type EquipmentRepo interface {
	CreateEquipment(ctx context.Context, e *domain.Equipment) error
	GetEquipment(ctx context.Context, id string) (*domain.Equipment, error)
	ListEquipment(ctx context.Context) ([]domain.Equipment, error)
	UpdateEquipmentStatus(ctx context.Context, id string, to domain.EquipmentStatus, expectedVersion int) error

	CreateChamber(ctx context.Context, c *domain.Chamber) error
	GetChamber(ctx context.Context, id string) (*domain.Chamber, error)
	ListChambers(ctx context.Context, equipmentID string) ([]domain.Chamber, error)

	CreateQualification(ctx context.Context, q *domain.Qualification) error
	GetQualification(ctx context.Context, id string) (*domain.Qualification, error)
	// QualificationsFor 返回设备在站点上的全部资质窗口。
	QualificationsFor(ctx context.Context, equipmentID, stationID string) ([]domain.Qualification, error)
	ListQualifications(ctx context.Context) ([]domain.Qualification, error)
	UpdateQualificationStatus(ctx context.Context, id string, to domain.QualStatus) error
}
