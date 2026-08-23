package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
)

// Deps 服务层共享依赖。
type Deps struct {
	Store repository.Store
	Clock clock.Clock
}

// Services 服务编排入口集合。
type Services struct {
	Master  *MasterService
	Lot     *LotService
	Run     *RunService
	Reading *ReadingService
	Hold    *HoldService
	Query   *QueryService
}

// NewServices 构建全部服务。
func NewServices(d Deps) *Services {
	return &Services{
		Master:  &MasterService{d},
		Lot:     &LotService{d},
		Run:     &RunService{d},
		Reading: &ReadingService{d},
		Hold:    &HoldService{d},
		Query:   &QueryService{d},
	}
}

// idempotent 业务幂等执行器：
// 相同 scope+key 的重复请求直接返回首次存储的响应，不重复执行副作用。
// 首次执行与幂等响应写入必须在同一事务内完成：fn 拿到的是事务 tx，
// 任一步失败整体回滚，绝不留下部分提交的副作用；PutIdempotency 亦写入该 tx，
// 故 fn 失败时幂等响应不落库，重试以同 key 仍按首次执行处理。
func idempotent[T any](
	ctx context.Context,
	d Deps,
	scope, key string,
	fn func(ctx context.Context) (T, error),
) (T, bool, error) {
	var zero T
	// 不传幂等键：仅以单事务保证 fn 内部写入的原子性，不登记幂等响应。
	if key == "" {
		var out T
		err := d.Store.InTx(ctx, func(tx context.Context) error {
			var innerErr error
			out, innerErr = fn(tx)
			return innerErr
		})
		if err != nil {
			return zero, false, err
		}
		return out, false, nil
	}
	// 幂等键已命中：直接返回首次存储的响应，不重复执行副作用。
	raw, err := d.Store.GetIdempotency(ctx, scope, key)
	if err == nil {
		var cached T
		if uerr := json.Unmarshal([]byte(raw), &cached); uerr != nil {
			return zero, false, fmt.Errorf("幂等响应解码失败: %w", uerr)
		}
		return cached, true, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return zero, false, err
	}
	// 首次执行：fn 与幂等响应登记在同一事务内完成，整体提交或整体回滚。
	// 注意必须把事务 tx 传给 fn——若传原始 ctx，fn 内的每条写入各自自动提交，
	// 后续步骤（如审计）失败将无法回滚已提交的前序写入，造成部分提交。
	var out T
	err = d.Store.InTx(ctx, func(tx context.Context) error {
		var ferr error
		out, ferr = fn(tx)
		if ferr != nil {
			return ferr
		}
		payload, merr := json.Marshal(out)
		if merr != nil {
			return merr
		}
		return d.Store.PutIdempotency(tx, scope, key, string(payload), d.Clock.Now())
	})
	if err != nil {
		return zero, false, err
	}
	return out, false, nil
}

// audit 在事务内写入结构化审计事件。
func audit(ctx context.Context, d Deps, txTag, entity, entityID, action string, detail any) error {
	raw := ""
	if detail != nil {
		b, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	return d.Store.CreateAudit(ctx, &domain.AuditEvent{
		ID:        domain.NewID(domain.IDPrefixAudit),
		Entity:    entity,
		EntityID:  entityID,
		Action:    action,
		Detail:    raw,
		TxTag:     txTag,
		CreatedAt: d.Clock.Now(),
	})
}

// holdBlockingCheck 校验批次及其全部后代、全部祖先不存在未关闭暂扣：
// 父批暂扣阻断子批，子批暂扣阻断自身及其后代。
func holdBlockingCheck(ctx context.Context, d Deps, lotID string) error {
	ids, err := d.Store.DescendantLotIDs(ctx, lotID)
	if err != nil {
		return err
	}
	ids = append([]string{lotID}, ids...)
	// 沿父链向上收集祖先。
	current := lotID
	for depth := 0; depth < 100; depth++ {
		lot, err := d.Store.GetLot(ctx, current)
		if err != nil {
			return err
		}
		if lot.ParentLotID == "" {
			break
		}
		ids = append(ids, lot.ParentLotID)
		current = lot.ParentLotID
	}
	holds, err := d.Store.HoldsForLots(ctx, ids)
	if err != nil {
		return err
	}
	for _, h := range holds {
		if h.IsOpen() {
			return domain.ErrHoldBlocking
		}
	}
	return nil
}

// loadFreeze 加载并解码批次冻结快照。
func loadFreeze(lot *domain.Lot) (*domain.FreezeSnapshot, error) {
	if !lot.IsFrozen() {
		return nil, domain.ErrNotFrozen
	}
	return domain.DecodeFreezeSnapshot(lot.FreezeSnapshot)
}
