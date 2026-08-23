package service_test

import (
	"context"
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
	"gowork/wafer/internal/service"
)

// auditFaultStore 包装 Store，让第 N 次 CreateAudit 写入失败，用于复现审计故障。
// 通过嵌入 repository.Store 接口，除 CreateAudit 外的全部方法透明委托给底层实现，
// 事务上下文（txKey）经由底层 InTx 与各仓储方法原样流转。
type auditFaultStore struct {
	repository.Store
	failOnAudit int   // 第几次 CreateAudit 调用失败（1 起计）
	auditCalls  int   // 已发生的 CreateAudit 调用次数
	failErr     error // 注入的审计故障错误
}

func (s *auditFaultStore) CreateAudit(ctx context.Context, e *domain.AuditEvent) error {
	s.auditCalls++
	if s.auditCalls == s.failOnAudit {
		return s.failErr
	}
	return s.Store.CreateAudit(ctx, e)
}

// TestSplitAuditFailureRollbackWithIdemKey 复现：带幂等键拆分子批时审计写入失败，
// 子批创建、晶圆迁移与双向审计必须整体回滚，不得残留；后续以同幂等键重试必须干净成功，
// 不得因残留子批报编码冲突。
func TestSplitAuditFailureRollbackWithIdemKey(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()
	parent := e.registerLot("LOT-SPLIT-IDEM")
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)

	// 用在第 2 条审计（子批拆分审计）失败的 Store 重建服务，模拟审计写入故障。
	faulty := &auditFaultStore{
		Store:       e.store,
		failOnAudit: 2,
		failErr:     errors.New("审计写入故障: 磁盘满"),
	}
	e.svc = service.NewServices(service.Deps{Store: faulty, Clock: e.clk})

	_, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-SPLIT-IDEM-C1",
		[]string{wafers[2].ID}, "split-key-1")
	if err == nil {
		t.Fatal("审计故障应导致拆分失败")
	}

	// 子批不得残留。
	if _, err := e.store.FindLotByCode(e.ctx, "LOT-SPLIT-IDEM-C1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("审计失败后子批不得残留: %v", err)
	}
	// 晶圆归属必须仍是父批，不得已迁移。
	w, _, _ := e.svc.Lot.WaferGenealogy(e.ctx, wafers[2].ID)
	if w == nil || w.LotID != parent.ID {
		t.Fatalf("审计失败后晶圆不得已迁移: lot_id=%s", wifLotID(w))
	}
	// 拆分审计不得残留。
	audits, _ := e.store.ListAudit(e.ctx, "lot", parent.ID)
	for _, a := range audits {
		if a.Action == domain.AuditLotSplit {
			t.Fatal("回滚后拆分审计不得残留")
		}
	}

	// 重试：换回正常 Store，使用相同幂等键，必须干净成功，不报编码冲突。
	e.svc = service.NewServices(service.Deps{Store: e.store, Clock: e.clk})
	child, replay, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-SPLIT-IDEM-C1",
		[]string{wafers[2].ID}, "split-key-1")
	if err != nil {
		t.Fatalf("重试应成功，不得报编码冲突: %v", err)
	}
	if replay {
		t.Fatal("重试为首次成功执行，不应标记重放")
	}
	cw, _ := e.svc.Lot.ListWafers(e.ctx, child.ID)
	if len(cw) != 1 || cw[0].ID != wafers[2].ID {
		t.Fatalf("重试后晶圆归属子批错误: %+v", cw)
	}
}

// wifLotID 安全读取可能为 nil 的晶圆归属。
func wifLotID(w *domain.Wafer) string {
	if w == nil {
		return "<nil>"
	}
	return w.LotID
}
