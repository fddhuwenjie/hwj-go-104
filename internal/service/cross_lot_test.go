package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
)

// TestCreateRunRejectsCrossLotWafer 开工晶圆必须属于目标批次：
// 甲批次开工时显式选择乙批次的一片活跃且空闲的晶圆，必须被拒绝，
// 且不得创建运行、不得把该晶圆写入甲批次的运行清单。
// 否则两个批次的在制追溯相互矛盾：运行关联乙晶圆，而晶圆归属仍为乙批次。
func TestCreateRunRejectsCrossLotWafer(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lotA := e.registerLot("LOT-X-A")
	lotB := e.registerLot("LOT-X-B")

	// 两个批次都进站排队，均未开工（乙批晶圆空闲）。
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotA.ID, ""); err != nil {
		t.Fatalf("A 进站: %v", err)
	}
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotB.ID, ""); err != nil {
		t.Fatalf("B 进站: %v", err)
	}

	bWafers, err := e.svc.Lot.ListWafers(e.ctx, lotB.ID)
	if err != nil {
		t.Fatalf("B 晶圆: %v", err)
	}
	// 选取乙批次第一片活跃晶圆（空闲、非占用）。
	foreign := bWafers[0]
	if foreign.Status != domain.WaferActive {
		t.Fatalf("乙批晶圆应活跃: %s", foreign.Status)
	}

	// 甲批次开工并显式选择乙批次的晶圆。
	run, _, err := e.svc.Run.CreateRun(e.ctx, lotA.ID, e.eq1.ID, e.ch1.ID, []string{foreign.ID}, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("跨批选择乙晶圆应被拒绝，got: %v", err)
	}
	if run != nil {
		t.Fatalf("拒绝后不得创建运行，got: %+v", run)
	}

	// 甲批次必须保持 QUEUED（未被置为 RUNNING）。
	aLot, _ := e.svc.Lot.GetLot(e.ctx, lotA.ID)
	if aLot.Status != domain.LotQueued {
		t.Fatalf("甲批次状态应保持 QUEUED: %s", aLot.Status)
	}

	// 甲批次不得存在任何运行（运行清单不得包含乙晶圆）。
	aRuns, _ := e.svc.Run.ListRunsByLot(e.ctx, lotA.ID)
	if len(aRuns) != 0 {
		t.Fatalf("甲批次不应残留运行: %+v", aRuns)
	}

	// 乙批晶圆不得进入任何运行清单（在制追溯一致性）。
	busy, err := e.store.BusyWaferIDs(e.ctx)
	if err != nil {
		t.Fatalf("BusyWaferIDs: %v", err)
	}
	if busy[foreign.ID] {
		t.Fatal("乙批晶圆不得被标记为甲批次运行占用")
	}
}

// TestCreateRunAcceptsOwnLotWafers 保证修复后正常路径仍可显式选择本批晶圆。
func TestCreateRunAcceptsOwnLotWafers(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-OWN")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	own, _ := e.svc.Lot.ListWafers(e.ctx, lot.ID)
	run, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq1.ID, e.ch1.ID, []string{own[0].ID, own[1].ID}, "")
	if err != nil {
		t.Fatalf("显式选择本批晶圆应允许: %v", err)
	}
	ids, err := e.store.RunWafers(e.ctx, run.ID)
	if err != nil {
		t.Fatalf("查询运行晶圆: %v", err)
	}
	want := map[string]bool{own[0].ID: true, own[1].ID: true}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("运行清单出现非本批晶圆: %s", id)
		}
	}
}

// TestCreateRunRejectsMixOfOwnAndForeignWafer 本批与跨批晶圆混合选择必须整体拒绝：
// 任一晶圆不属于目标批次即不得写入运行清单。
func TestCreateRunRejectsMixOfOwnAndForeignWafer(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lotA := e.registerLot("LOT-M-A")
	lotB := e.registerLot("LOT-M-B")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotA.ID, ""); err != nil {
		t.Fatalf("A 进站: %v", err)
	}
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotB.ID, ""); err != nil {
		t.Fatalf("B 进站: %v", err)
	}
	aWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotA.ID)
	bWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotB.ID)

	// 甲批次开工，清单中混入一片乙批晶圆。
	_, _, err := e.svc.Run.CreateRun(e.ctx, lotA.ID, e.eq1.ID, e.ch1.ID,
		[]string{aWafers[0].ID, bWafers[0].ID}, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("混合选择含跨批晶圆应被拒绝: %v", err)
	}
	aRuns, _ := e.svc.Run.ListRunsByLot(e.ctx, lotA.ID)
	if len(aRuns) != 0 {
		t.Fatalf("甲批次不应残留运行: %+v", aRuns)
	}
}
