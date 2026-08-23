package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// TestCrossRunReadingRejected 跨运行提交读数必须拒绝：
// 甲运行完工后提交量测时，混入乙运行的一片活跃晶圆，
// 系统不得把读数保存并挂在甲运行下。读数的晶圆必须实际参与目标运行
// （属于该运行的 run_wafers），否则按晶圆与运行追溯会得到矛盾结果。
func TestCrossRunReadingRejected(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	// 甲批次：进站、开工、完工（COMPLETED，待量测）。
	lotA := e.registerLot("LOT-A")
	runA := e.enterAndRun(lotA.ID)
	if _, _, err := e.svc.Run.CompleteRun(e.ctx, runA.ID, ""); err != nil {
		t.Fatalf("甲完工: %v", err)
	}

	// 乙批次：进站、开工（RUNNING），其晶圆处于活跃。
	lotB := e.registerLot("LOT-B")
	runB := e.enterAndRun(lotB.ID)
	bWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotB.ID)
	stray := bWafers[0]
	if stray.Status != domain.WaferActive {
		t.Fatalf("乙晶圆应为 ACTIVE: %s", stray.Status)
	}
	// 乙晶圆确实参与乙运行，但不参与甲运行。
	bRunWafers, _ := e.store.RunWafers(e.ctx, runB.ID)
	if !contains(bRunWafers, stray.ID) {
		t.Fatalf("乙晶圆应参与乙运行")
	}
	aRunWafers, _ := e.store.RunWafers(e.ctx, runA.ID)
	if contains(aRunWafers, stray.ID) {
		t.Fatalf("乙晶圆不应参与甲运行")
	}

	// 跨运行提交：把乙的活跃晶圆读数挂到甲运行下。
	_, _, err := e.svc.Reading.SubmitReadings(e.ctx, runA.ID, []service.ReadingInput{
		{WaferID: stray.ID, Metric: "cd", Value: 5.0},
	}, "")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("跨运行提交活跃晶圆读数必须被拒绝，得到: %v", err)
	}

	// 甲运行下不应残留任何属于乙晶圆的读数（谱系一致性）。
	readings, _ := e.svc.Reading.ListReadings(e.ctx, runA.ID)
	for _, r := range readings {
		if r.WaferID == stray.ID {
			t.Fatalf("甲运行下残留了乙晶圆读数，按晶圆与运行追溯矛盾: %+v", r)
		}
	}

	// 对照：甲运行自身的成员晶圆仍可正常提交（回归保护）。
	aWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotA.ID)
	ownInputs := []service.ReadingInput{
		{WaferID: aWafers[0].ID, Metric: "cd", Value: 5.0},
		{WaferID: aWafers[1].ID, Metric: "cd", Value: 5.0},
	}
	if _, _, err := e.svc.Reading.SubmitReadings(e.ctx, runA.ID, ownInputs, ""); err != nil {
		t.Fatalf("甲运行成员晶圆应可提交: %v", err)
	}

	// 失效（报废）晶圆同样被拒绝：复刻原行为中被状态分支唯一拦截的场景，
	// 修复后改由成员校验统一拦截。
	lotS := e.registerLot("LOT-S")
	e.enterAndRun(lotS.ID)
	sWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotS.ID)
	if _, _, err := e.svc.Lot.Scrap(e.ctx, lotS.ID, "报废", ""); err != nil {
		t.Fatalf("报废批次: %v", err)
	}
	// 报废批次不再有可提交的运行；用甲运行提交报废晶圆读数须被拒绝。
	if _, _, err := e.svc.Reading.SubmitReadings(e.ctx, runA.ID, []service.ReadingInput{
		{WaferID: sWafers[0].ID, Metric: "cd", Value: 5.0},
	}, ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("不属于运行的报废晶圆读数须被拒绝: %v", err)
	}
}

// TestReadingPersistenceConstraint 持久化约束：
// 即便绕过服务层成员校验直接写仓储，数据库外键仍须拒绝不属于运行的晶圆读数。
func TestReadingPersistenceConstraint(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lotA := e.registerLot("LOT-PC-A")
	runA := e.enterAndRun(lotA.ID)

	lotB := e.registerLot("LOT-PC-B")
	bWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotB.ID)
	stray := bWafers[0]

	// 直接走仓储层，绕过服务校验。
	r := &domain.Reading{
		ID:        domain.NewID(domain.IDPrefixReading),
		RunID:     runA.ID,
		WaferID:   stray.ID,
		Slot:      stray.Slot,
		Metric:    "cd",
		Value:     5.0,
		CreatedAt: e.clk.Now(),
	}
	if err := e.store.CreateReading(e.ctx, r); err == nil {
		t.Fatal("持久化约束须拒绝不属于运行的晶圆读数")
	}

	// 对照：运行成员晶圆可直接写入。
	aWafers, _ := e.svc.Lot.ListWafers(e.ctx, lotA.ID)
	ok := &domain.Reading{
		ID:        domain.NewID(domain.IDPrefixReading),
		RunID:     runA.ID,
		WaferID:   aWafers[0].ID,
		Slot:      aWafers[0].Slot,
		Metric:    "cd",
		Value:     5.0,
		CreatedAt: e.clk.Now(),
	}
	if err := e.store.CreateReading(e.ctx, ok); err != nil {
		t.Fatalf("运行成员晶圆读数应可写入: %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
