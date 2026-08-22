package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
)

// TestSplitLotGenealogy 子批拆分与晶圆谱系：
// 子批继承冻结快照，晶圆迁移写入不可变记录，谱系可完整追溯。
func TestSplitLotGenealogy(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-P")
	e.enterAndRun(parent.ID)                       // 冻结并进入 RUNNING
	parent, _ = e.svc.Lot.GetLot(e.ctx, parent.ID) // 重新读取冻结后的父批

	wafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)
	child, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-P-C1", []string{wafers[2].ID}, "")
	if err != nil {
		t.Fatalf("拆分: %v", err)
	}
	if child.ParentLotID != parent.ID {
		t.Fatalf("子批父指针错误: %s", child.ParentLotID)
	}
	// 父批冻结过，子批必须继承同一冻结修订。
	if child.FrozenRevisionID != parent.FrozenRevisionID || child.FreezeSnapshot == "" {
		t.Fatal("子批必须继承父批冻结快照")
	}

	// 晶圆归属迁移。
	w, _, err := e.svc.Lot.WaferGenealogy(e.ctx, wafers[2].ID)
	if err != nil || w == nil {
		t.Fatalf("谱系查询失败: %v", err)
	}
	if w.LotID != child.ID {
		t.Fatalf("晶圆归属错误: %s", w.LotID)
	}

	// 晶圆当前归属子批。
	childWafers, _ := e.svc.Lot.ListWafers(e.ctx, child.ID)
	if len(childWafers) != 1 || childWafers[0].ID != wafers[2].ID {
		t.Fatalf("子批晶圆错误: %+v", childWafers)
	}
	parentWafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)
	if len(parentWafers) != 2 {
		t.Fatalf("父批晶圆错误: %d", len(parentWafers))
	}

	// 迁移记录不可变且完整。
	_, moves, err := e.svc.Lot.WaferGenealogy(e.ctx, wafers[2].ID)
	if err != nil || len(moves) != 1 {
		t.Fatalf("迁移记录错误: %v %d", err, len(moves))
	}
	if moves[0].FromLotID != parent.ID || moves[0].ToLotID != child.ID {
		t.Fatalf("迁移记录内容错误: %+v", moves[0])
	}

	// 谱系审计：正常拆分不产生 WAFER_NO_MOVE 问题。
	issues, err := e.svc.Query.GenealogyAudit(e.ctx)
	if err != nil {
		t.Fatalf("谱系审计: %v", err)
	}
	for _, is := range issues {
		if is.Issue == "WAFER_NO_MOVE" && is.LotID == child.ID {
			t.Fatalf("正常拆分不应出现谱系断裂: %+v", is)
		}
	}
}

// TestSplitRollback 拆分事务回滚：迁移不存在的晶圆导致整事务回滚，
// 子批、迁移记录与审计均不残留。
func TestSplitRollback(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-RB")
	_, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-RB-C1", []string{"wf_ghost"}, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("应返回校验错误: %v", err)
	}
	// 子批不得存在。
	if _, err := e.svc.Lot.GetLot(e.ctx, "lot_nonexistent"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("预期 NotFound: %v", err)
	}
	lots, _, _ := e.svc.Lot.ListLots(e.ctx, domain.Page{Limit: 100})
	for _, l := range lots {
		if l.Code == "LOT-RB-C1" {
			t.Fatal("回滚后子批不应存在")
		}
	}
	audits, _ := e.svc.Query.ListAudit(e.ctx, "lot", parent.ID)
	for _, a := range audits {
		if a.Action == domain.AuditLotSplit {
			t.Fatal("回滚后拆分审计不应存在")
		}
	}
}

// TestStationSkip 非法越站：未放行时进站下一站点被拒绝。
func TestStationSkip(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-SKIP")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("首次进站: %v", err)
	}
	// QUEUED 状态不可重复进站。
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("重复进站应失败: %v", err)
	}
	// 运行中不可进站下一站。
	run, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq1.ID, e.ch1.ID, nil, "")
	if err != nil {
		t.Fatalf("开工: %v", err)
	}
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("运行中进站应失败: %v", err)
	}
	_ = run
}
