package service

import (
	"context"
	"time"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
)

// QueryService 分析查询服务：全部提供稳定分页或确定性排序。
type QueryService struct {
	d Deps
}

// ExpiredQualificationRuns 使用过期资质但尚未复判的运行（稳定分页）。
func (s *QueryService) ExpiredQualificationRuns(ctx context.Context, page domain.Page) ([]repository.ExpiredQualificationRun, string, error) {
	items, err := s.d.Store.ExpiredQualificationRuns(ctx, page)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) == page.Normalize().Limit && len(items) > 0 {
		last := items[len(items)-1]
		if items[0].CreatedAt.Equal(last.CreatedAt) {
			last = items[0]
		}
		next = domain.EncodeCursor(last.CreatedAt, last.RunID)
	}
	return items, next, nil
}

// WipLots 当前在制批次（含冻结路线与最近暂扣原因，稳定分页）。
func (s *QueryService) WipLots(ctx context.Context, page domain.Page) ([]repository.WipLot, string, error) {
	items, err := s.d.Store.WipLots(ctx, page)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) == page.Normalize().Limit && len(items) > 0 {
		last := items[len(items)-1]
		next = domain.EncodeCursor(last.CreatedAt, last.LotID)
	}
	return items, next, nil
}

// StationQueues 等待时间超过阈值且设备能力可用的站点队列。
func (s *QueryService) StationQueues(ctx context.Context, minWait time.Duration) ([]repository.StationQueueItem, error) {
	if minWait < 0 {
		return nil, domain.NewValidationError(domain.FieldError{Field: "min_wait", Message: "等待阈值不能为负"})
	}
	return s.d.Store.StationQueues(ctx, minWait, s.d.Clock.Now())
}

// ReworkStats 按设备腔体与配方版本聚合的重复返工批次。
func (s *QueryService) ReworkStats(ctx context.Context) ([]repository.ReworkStat, error) {
	return s.d.Store.ReworkStats(ctx)
}

// GenealogyAudit 父子批状态或晶圆归属不一致审计。
func (s *QueryService) GenealogyAudit(ctx context.Context) ([]repository.GenealogyIssue, error) {
	return s.d.Store.GenealogyAudit(ctx)
}

// ListReleases 批次放行记录。
func (s *QueryService) ListReleases(ctx context.Context, lotID string) ([]domain.Release, error) {
	return s.d.Store.ListReleases(ctx, lotID)
}

// ListAudit 实体审计事件。
func (s *QueryService) ListAudit(ctx context.Context, entity, entityID string) ([]domain.AuditEvent, error) {
	return s.d.Store.ListAudit(ctx, entity, entityID)
}
