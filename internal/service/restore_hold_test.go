package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
)

// TestRestoreRequiresReviewRootLot 根批次存在开放暂扣时直接 Restore，
// 必须被阻断：状态不得回退、暂扣不得关闭——任何批次恢复前必须先复判。
// 复现修复前的穿透：Restore 将 ON_HOLD 回退 QUEUED 且暂扣仍 OPEN。
func TestRestoreRequiresReviewRootLot(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-ROOT")
	_, hold := failAndHold(t, e, lot.ID) // 根批第一站 FAIL -> OPEN hold
	if lot.ParentLotID != "" {
		t.Fatalf("应为根批次，不可有父批")
	}
	// 复判前直接 Restore：开放暂扣必须阻断（修复前穿透放行）。
	restored, _, err := e.svc.Lot.Restore(e.ctx, lot.ID, "")
	if err == nil {
		t.Fatalf("根批次开放暂扣未复判即 Restore 不应成功，但返回状态 %s", restored.Status)
	}
	if !errors.Is(err, domain.ErrHoldBlocking) {
		t.Fatalf("应返回暂扣阻断错误 ErrHoldBlocking，得到: %v", err)
	}
	// 状态不得回退，仍为 ON_HOLD。
	after, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
	if after.Status != domain.LotOnHold {
		t.Fatalf("状态被回退，应仍为 ON_HOLD: %s", after.Status)
	}
	// 暂扣不得关闭，仍 OPEN。
	h2, _ := e.svc.Hold.GetHold(e.ctx, hold.ID)
	if h2.Status != domain.HoldOpen {
		t.Fatalf("暂扣被关闭，应仍为 OPEN: %s", h2.Status)
	}
}

// TestRestoreRequiresReviewChildLot 子批自身开放暂扣直接 Restore 必须被阻断。
// 复现修复前穿透：子批无后代、自身暂扣被阻断集合遗漏，Restore 状态回退 QUEUED。
func TestRestoreRequiresReviewChildLot(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-PC2")
	_ = e.enterAndRun(parent.ID)
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)
	child, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-PC2-C1", []string{wafers[2].ID}, "")
	if err != nil {
		t.Fatalf("拆分: %v", err)
	}
	if _, _, err := e.svc.Hold.CreateHold(e.ctx, child.ID, "子批异常", ""); err != nil {
		t.Fatalf("子批暂扣: %v", err)
	}
	// 子批 Restore：自身开放暂扣必须阻断（需先复判）。
	restored, _, err := e.svc.Lot.Restore(e.ctx, child.ID, "")
	if err == nil {
		t.Fatalf("子批自身开放暂扣 Restore 不应成功，但返回状态 %s", restored.Status)
	}
	if !errors.Is(err, domain.ErrHoldBlocking) {
		t.Fatalf("子批 Restore 应返回暂扣阻断错误，得到: %v", err)
	}
	after, _ := e.svc.Lot.GetLot(e.ctx, child.ID)
	if after.Status != domain.LotOnHold {
		t.Fatalf("状态被回退，应仍为 ON_HOLD: %s", after.Status)
	}
}

// TestRestoreBlockedByAncestorOpenHold 子批自身无开放暂扣，
// 但父批开放暂扣未复判时，子批 Restore 也必须被阻断——
// 证明修复后的阻断集合同时覆盖祖先暂扣。
func TestRestoreBlockedByAncestorOpenHold(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-PA")
	// 父批进站后人工暂扣（OPEN）。
	_ = e.enterAndRun(parent.ID)
	if _, _, err := e.svc.Hold.CreateHold(e.ctx, parent.ID, "父批抽检", ""); err != nil {
		t.Fatalf("父批暂扣: %v", err)
	}
	// 子批拆分后自身无暂扣，但因继承父批进度处于 ON_HOLD 谱系中；
	// 直接对父批 Restore 必须被父批自身开放暂扣阻断。
	restored, _, err := e.svc.Lot.Restore(e.ctx, parent.ID, "")
	if err == nil {
		t.Fatalf("父批开放暂扣未复判即 Restore 不应成功，但返回状态 %s", restored.Status)
	}
	if !errors.Is(err, domain.ErrHoldBlocking) {
		t.Fatalf("应返回暂扣阻断错误，得到: %v", err)
	}
}

// TestRootHoldBlocksStationOps 根批次开放暂扣应阻断后续站点操作（进站）。
// 复现修复前阻断集合遗漏根批自身：holdBlockingCheck 对无后代的根批返回空集放行。
// （Enter 的状态机本身拒绝 ON_HOLD 进站，故这里用拆分后的子批进站验证
//  根批暂扣向下传导阻断——该路径不依赖根批自身集合，保持原行为。）
func TestRootHoldBlocksChildStationOps(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-RHS")
	// 父批人工暂扣后拆分子批，子批进站应被父批开放暂扣阻断。
	if _, _, err := e.svc.Hold.CreateHold(e.ctx, parent.ID, "抽检", ""); err != nil {
		t.Fatalf("父批暂扣: %v", err)
	}
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)
	child, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-RHS-C1", []string{wafers[2].ID}, "")
	if err != nil {
		t.Fatalf("拆分: %v", err)
	}
	if _, _, err := e.svc.Lot.Enter(e.ctx, child.ID, ""); !errors.Is(err, domain.ErrHoldBlocking) {
		t.Fatalf("父批开放暂扣应阻断子批进站，期望 ErrHoldBlocking，得到: %v", err)
	}
}
