package service_test

import (
	"context"
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
	"gowork/wafer/internal/sqlite"
)

// failingAuditStore 包裹真实 SQLite Store，仅对 CreateAudit 注入故障：
// 前 failAudit 次 CreateAudit 调用返回错误，之后透传。
// 嵌入 *sqlite.Store 以复用其余全部仓储方法（含 InTx），从而精确模拟
// “审计存储临时失败、其余存储正常”的场景。
type failingAuditStore struct {
	*sqlite.Store
	failAudit int
}

func (s *failingAuditStore) CreateAudit(ctx context.Context, e *domain.AuditEvent) error {
	if s.failAudit > 0 {
		s.failAudit--
		return errors.New("injected audit storage failure")
	}
	return s.Store.CreateAudit(ctx, e)
}

// TestAuditFailureRollsBackRegistration 复现：登记时审计存储临时失败，
// 批次/晶圆/审计/幂等键必须同成同败——审计失败则整体回滚，不得残留批次，
// 也不得以“批次编码重复”阻断后续重新登记。
func TestAuditFailureRollsBackRegistration(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	// 用审计故障注入仓储包裹真实 store，首次登记的审计写入将失败。
	failing := &failingAuditStore{Store: e.store, failAudit: 1}
	svc := service.NewServices(service.Deps{Store: failing, Clock: e.clk})

	const code = "LOT-AUDIT-FAIL"
	_, _, err := svc.Lot.RegisterLot(e.ctx, code, e.pf.ID, e.route.ID, waferInputs(code, 3), "audit-fail-key")
	if err == nil {
		t.Fatalf("审计故障时登记应返回错误")
	}

	// 同成同败：审计失败必须整体回滚，批次不得残留（否则后续同编码登记
	// 将被 lots.code UNIQUE 拦截，呈现为“重复”错误）。
	if _, err := e.store.FindLotByCode(e.ctx, code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("审计失败后批次残留（违反同成同败）: %v", err)
	}

	// 审计恢复后用同编码再次登记应成功（晶圆 code 同样受 UNIQUE 约束，
	// 若残留晶圆此处会因 wafers.code 冲突失败），并写回完整审计事件。
	lot, replay, err := svc.Lot.RegisterLot(e.ctx, code, e.pf.ID, e.route.ID, waferInputs(code, 3), "audit-fail-key-2")
	if err != nil {
		t.Fatalf("审计恢复后重新登记应成功，而非提示重复: %v", err)
	}
	if replay {
		t.Fatalf("新幂等键不应命中重放")
	}

	events, err := e.store.ListAudit(e.ctx, "lot", lot.ID)
	if err != nil {
		t.Fatalf("查询审计: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("成功登记必须写入审计事件")
	}
	if events[0].Action != domain.AuditLotCreate {
		t.Fatalf("审计动作错误: %s", events[0].Action)
	}
}
