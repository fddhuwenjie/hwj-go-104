package service_test

import (
	"testing"

	"gowork/wafer/internal/domain"
)

// TestWipLotsCrossVersionNoContamination 跨版本并列查询污染复现：
// 旧批冻结于修订1，启用修订2后登记并冻结新批于修订2；并列查询两条在制记录时，
// 每批必须展示各自冻结的修订号——旧批=1、新批=2。
// 缺陷表现：旧批冻结信息被改写为当前启用修订2（仓储按 ACTIVE 路线修订关联，
// 且服务层把所有记录的冻结修订号覆盖为首条），单条时无对照不易发现。
func TestWipLotsCrossVersionNoContamination(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()
	// 此刻 e.rev（Revision=1）为 ACTIVE。

	// 旧批 LOT-A：进站冻结于修订1。
	lotA := e.registerLot("LOT-A")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotA.ID, ""); err != nil {
		t.Fatalf("进站A: %v", err)
	}

	// 启用修订2（同事务退役修订1，修订2 成为当前 ACTIVE）。
	rev2, err := e.svc.Master.CreateRevision(e.ctx, e.route.ID, []domain.RouteStation{
		{Seq: 1, StationID: e.st1.ID, RecipeID: e.rc1.ID, MetrologyPlanID: e.plan1.ID},
		{Seq: 2, StationID: e.st2.ID, RecipeID: e.rc2.ID, MetrologyPlanID: e.plan2.ID},
	})
	if err != nil {
		t.Fatalf("创建修订2: %v", err)
	}
	if rev2, err = e.svc.Master.ActivateRevision(e.ctx, rev2.ID, rev2.Version); err != nil {
		t.Fatalf("启用修订2: %v", err)
	}

	// 新批 LOT-B：进站冻结于修订2。
	lotB := e.registerLot("LOT-B")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotB.ID, ""); err != nil {
		t.Fatalf("进站B: %v", err)
	}

	// 跨版本并列查询在制批次。
	items, _, err := e.svc.Query.WipLots(e.ctx, domain.Page{Limit: 10})
	if err != nil {
		t.Fatalf("在制查询: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("在制数量错误: %d", len(items))
	}

	// 每批必须展示自己的冻结修订号。
	byCode := map[string]int{}
	for _, it := range items {
		if it.FrozenRevision == nil {
			t.Fatalf("在制视图缺少冻结修订号: %s", it.Code)
		}
		byCode[it.Code] = *it.FrozenRevision
	}
	if byCode[lotA.Code] != 1 {
		t.Fatalf("旧批 %s 应展示冻结修订 1，得到 %d（跨版本污染）", lotA.Code, byCode[lotA.Code])
	}
	if byCode[lotB.Code] != 2 {
		t.Fatalf("新批 %s 应展示冻结修订 2，得到 %d", lotB.Code, byCode[lotB.Code])
	}

	// 单批查询同样不得被当前启用版本污染（回归对照）。
	one, _, err := e.svc.Query.WipLots(e.ctx, domain.Page{Limit: 1})
	if err != nil {
		t.Fatalf("单批在制查询: %v", err)
	}
	if len(one) != 1 || one[0].Code != lotA.Code || one[0].FrozenRevision == nil || *one[0].FrozenRevision != 1 {
		t.Fatalf("单批查询旧批冻结修订错误: %+v", one)
	}
}
