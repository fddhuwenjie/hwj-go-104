package service_test

import (
	"errors"
	"testing"
	"time"

	"gowork/wafer/internal/domain"
)

// TestQualificationTimeBoundary 设备资质时间边界：
// 开工时刻必须落在资质窗口内（valid_to 等于开工时刻视为失效）；
// 完工时刻超出窗口则标记 qual_covered=false 并进入过期资质查询。
func TestQualificationTimeBoundary(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.activateVersions()
	e.setupEquipment(false) // 不建长窗口资质，避免干扰边界断言

	// 覆盖 [base+1h, base+2h] 的短窗口资质。
	shortFrom := baseTime.Add(time.Hour)
	shortTo := baseTime.Add(2 * time.Hour)
	if _, err := e.svc.Master.CreateQualification(e.ctx, e.eq1.ID, e.ch1.ID, e.st1.ID, shortFrom, shortTo); err != nil {
		t.Fatalf("短窗口资质: %v", err)
	}

	lot := e.registerLot("LOT-QUAL")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}

	// 边界 1：开工时刻 == valid_from（允许）。
	e.clk.Set(shortFrom)
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq1.ID, e.ch1.ID, nil, ""); err != nil {
		t.Fatalf("valid_from 边界开工应允许: %v", err)
	}

	// 完工时刻超过 valid_to（shortTo）→ qual_covered=false。
	e.clk.Set(shortTo.Add(time.Minute))
	run, _ := e.svc.Run.ListRunsByLot(e.ctx, lot.ID)
	completed, _, err := e.svc.Run.CompleteRun(e.ctx, run[0].ID, "")
	if err != nil {
		t.Fatalf("完工: %v", err)
	}
	if completed.QualCovered {
		t.Fatal("资质窗口未覆盖完工时刻，qual_covered 必须为 false")
	}

	// 过期资质运行进入查询，复判后消失。
	items, _, err := e.svc.Query.ExpiredQualificationRuns(e.ctx, domain.Page{Limit: 10})
	if err != nil {
		t.Fatalf("过期资质查询: %v", err)
	}
	if len(items) != 1 || items[0].RunID != completed.ID {
		t.Fatalf("过期资质查询结果错误: %+v", items)
	}
}

// TestCompletionAtQualificationExpiry 完工时刻恰等于资质截止时刻（valid_to）时，
// 完整覆盖区间的截止时刻应包含该次完工：qual_covered 必须为 true，运行不得进入过期复判清单。
// 这是与 TestQualificationTimeBoundary（完工晚于 valid_to → 未覆盖）互补的边界：
// 恰在 valid_to 完工应被覆盖，提前完工同样覆盖，只有超过 valid_to 才标记未覆盖。
func TestCompletionAtQualificationExpiry(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.activateVersions()
	e.setupEquipment(false) // 不建长窗口资质，避免干扰边界断言

	// 覆盖 [base+1h, base+2h] 的短窗口资质。
	shortFrom := baseTime.Add(time.Hour)
	shortTo := baseTime.Add(2 * time.Hour)
	if _, err := e.svc.Master.CreateQualification(e.ctx, e.eq1.ID, e.ch1.ID, e.st1.ID, shortFrom, shortTo); err != nil {
		t.Fatalf("短窗口资质: %v", err)
	}

	lot := e.registerLot("LOT-EXPIRY")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}

	// 开工时刻 == valid_from（允许）。
	e.clk.Set(shortFrom)
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq1.ID, e.ch1.ID, nil, ""); err != nil {
		t.Fatalf("valid_from 边界开工应允许: %v", err)
	}
	run, _ := e.svc.Run.ListRunsByLot(e.ctx, lot.ID)

	// 边界：完工时刻 == valid_to。完整覆盖区间截止时刻应包含该次完工。
	e.clk.Set(shortTo)
	completed, _, err := e.svc.Run.CompleteRun(e.ctx, run[0].ID, "")
	if err != nil {
		t.Fatalf("完工: %v", err)
	}
	if !completed.QualCovered {
		t.Fatal("完工时刻 == valid_to 应被资质窗口覆盖，qual_covered 必须为 true")
	}

	// 已覆盖的运行不得进入过期资质复判清单。
	items, _, err := e.svc.Query.ExpiredQualificationRuns(e.ctx, domain.Page{Limit: 10})
	if err != nil {
		t.Fatalf("过期资质查询: %v", err)
	}
	for _, it := range items {
		if it.RunID == completed.ID {
			t.Fatalf("完工恰在 valid_to 应已覆盖，不得进入过期复判清单: %+v", it)
		}
	}

	// 对照：提前一毫秒完工同样覆盖（证明修复未引入对“提前完工”的回归）。
	lot2 := e.registerLot("LOT-EXPIRY-EARLY")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot2.ID, ""); err != nil {
		t.Fatalf("进站2: %v", err)
	}
	e.clk.Set(shortFrom)
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot2.ID, e.eq1.ID, e.ch1.ID, nil, ""); err != nil {
		t.Fatalf("开工2: %v", err)
	}
	run2, _ := e.svc.Run.ListRunsByLot(e.ctx, lot2.ID)
	e.clk.Set(shortTo.Add(-time.Millisecond))
	completed2, _, err := e.svc.Run.CompleteRun(e.ctx, run2[0].ID, "")
	if err != nil {
		t.Fatalf("完工2: %v", err)
	}
	if !completed2.QualCovered {
		t.Fatal("完工时刻早于 valid_to 应被覆盖，qual_covered 必须为 true")
	}
}

// TestStartBeforeQualification 开工时刻早于资质生效时间必须被拒绝。
func TestStartBeforeQualification(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.activateVersions()
	e.setupEquipment(false)

	futureFrom := baseTime.Add(10 * time.Hour)
	futureTo := baseTime.Add(20 * time.Hour)
	// 仅存在未来生效的腔体级资质，且站点级资质先吊销——用新设备避免干扰。
	eq, err := e.svc.Master.CreateEquipment(e.ctx, "EQ-ETCH-2", "刻蚀机2", "FAMILY-A", e.st1.ID)
	if err != nil {
		t.Fatalf("设备: %v", err)
	}
	ch, err := e.svc.Master.CreateChamber(e.ctx, eq.ID, "CH-A", "etch")
	if err != nil {
		t.Fatalf("腔体: %v", err)
	}
	if _, err := e.svc.Master.CreateQualification(e.ctx, eq.ID, ch.ID, e.st1.ID, futureFrom, futureTo); err != nil {
		t.Fatalf("资质: %v", err)
	}
	lot := e.registerLot("LOT-EARLY")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, eq.ID, ch.ID, nil, ""); !errors.Is(err, domain.ErrQualification) {
		t.Fatalf("资质未生效开工应拒绝: %v", err)
	}
	// 边界：开工时刻 == valid_to，视为已失效，必须拒绝。
	e.clk.Set(futureTo)
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, eq.ID, ch.ID, nil, ""); !errors.Is(err, domain.ErrQualification) {
		t.Fatalf("valid_to 边界开工应拒绝: %v", err)
	}
}

// TestQualificationRejectsCrossEntityReferences 保证资质不能把设备、站点和腔体跨实体拼接。
func TestQualificationRejectsCrossEntityReferences(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.setupEquipment(false)
	from, to := baseTime.Add(-time.Hour), baseTime.Add(time.Hour)

	if _, err := e.svc.Master.CreateQualification(e.ctx, e.eq1.ID, e.ch2.ID, e.st1.ID, from, to); err == nil {
		t.Fatal("其他设备的腔体不应挂到当前设备资质")
	}
	if _, err := e.svc.Master.CreateQualification(e.ctx, e.eq1.ID, "", e.st2.ID, from, to); err == nil {
		t.Fatal("设备资质不应挂到其他站点")
	}
	if _, err := e.svc.Master.CreateQualification(e.ctx, e.eq1.ID, e.ch1.ID, e.st1.ID, from, to); err != nil {
		t.Fatalf("一致的设备、腔体和站点应允许建档: %v", err)
	}
}

// TestRecipeFamilyMismatch 配方只能在适配设备族上执行。
func TestRecipeFamilyMismatch(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	// 其它设备族设备，挂在同一站点且资质齐全。
	eq, _ := e.svc.Master.CreateEquipment(e.ctx, "EQ-OTHER", "异族机", "FAMILY-B", e.st1.ID)
	ch, _ := e.svc.Master.CreateChamber(e.ctx, eq.ID, "CH-A", "etch")
	from := baseTime.Add(-time.Hour)
	to := baseTime.Add(24 * time.Hour)
	if _, err := e.svc.Master.CreateQualification(e.ctx, eq.ID, "", e.st1.ID, from, to); err != nil {
		t.Fatalf("资质: %v", err)
	}
	lot := e.registerLot("LOT-FAM")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, eq.ID, ch.ID, nil, ""); !errors.Is(err, domain.ErrRecipeFamily) {
		t.Fatalf("异族设备开工应拒绝: %v", err)
	}
}

// TestCapabilityMismatch 腔体能力不匹配站点必须被拒绝。
func TestCapabilityMismatch(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	eq, _ := e.svc.Master.CreateEquipment(e.ctx, "EQ-NOCAP", "无能力机", "FAMILY-A", e.st1.ID)
	ch, _ := e.svc.Master.CreateChamber(e.ctx, eq.ID, "CH-X", "implant") // 不含 etch
	from := baseTime.Add(-time.Hour)
	to := baseTime.Add(24 * time.Hour)
	if _, err := e.svc.Master.CreateQualification(e.ctx, eq.ID, "", e.st1.ID, from, to); err != nil {
		t.Fatalf("资质: %v", err)
	}
	lot := e.registerLot("LOT-CAP")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	if _, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, eq.ID, ch.ID, nil, ""); !errors.Is(err, domain.ErrCapability) {
		t.Fatalf("能力不匹配应拒绝: %v", err)
	}
}

// TestWaferBusy 同一晶圆不得同时处于两个运行。
func TestWaferBusy(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lotA := e.registerLot("LOT-A")
	lotB := e.registerLot("LOT-B")
	e.enterAndRun(lotA.ID)
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotB.ID, ""); err != nil {
		t.Fatalf("B 进站: %v", err)
	}
	// B 指定使用 A 的晶圆（不属于 B，应先报归属错误）。
	aWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotA.ID)
	_, _, err := e.svc.Run.CreateRun(e.ctx, lotB.ID, e.eq1.ID, e.ch1.ID, []string{aWafers[0].ID}, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("跨批晶圆应拒绝: %v", err)
	}

	// 同批晶圆占用：子批继承冻结后共用晶圆场景通过 BusyWaferIDs 防御。
	// 构造：A 运行中，把 A 的晶圆迁到 B（绕过拆分限制不可行），
	// 直接验证 BusyWaferIDs 内容。
	busy, err := e.store.BusyWaferIDs(e.ctx)
	if err != nil {
		t.Fatalf("BusyWaferIDs: %v", err)
	}
	if !busy[aWafers[0].ID] {
		t.Fatal("运行中晶圆必须标记占用")
	}
}
