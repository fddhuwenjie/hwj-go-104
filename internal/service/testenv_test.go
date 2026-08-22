package service_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
	"gowork/wafer/internal/sqlite"
)

// testEnv 测试环境：真实临时 SQLite 文件 + 手动时钟。
type testEnv struct {
	t     *testing.T
	db    string
	store *sqlite.Store
	clk   *clock.Manual
	svc   *service.Services
	ctx   context.Context

	pf    *domain.ProductFamily
	st1   *domain.Station
	st2   *domain.Station
	rc1   *domain.Recipe
	rc2   *domain.Recipe
	plan1 *domain.MetrologyPlan
	plan2 *domain.MetrologyPlan
	route *domain.Route
	rev   *domain.RouteRevision
	eq1   *domain.Equipment
	ch1   *domain.Chamber
	eq2   *domain.Equipment
	ch2   *domain.Chamber
}

var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// newTestEnv 打开独立临时数据库并构建主数据：
// 两个站点、两个配方（设备族 FAMILY-A）、两个量测计划、路线修订启用、
// 两台设备各一个腔体、覆盖 [base-1h, base+24h] 的资质窗口。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := filepath.Join(t.TempDir(), "test.db")
	return openEnv(t, db)
}

func openEnv(t *testing.T, db string) *testEnv {
	t.Helper()
	store, err := sqlite.Open(db)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	clk := clock.NewManual(baseTime)
	e := &testEnv{
		t:     t,
		db:    db,
		store: store,
		clk:   clk,
		ctx:   context.Background(),
	}
	e.svc = service.NewServices(service.Deps{Store: store, Clock: clk})
	return e
}

// setupMaster 构建主数据并启用路线修订。
func (e *testEnv) setupMaster() {
	t := e.t
	ctx := e.ctx
	m := e.svc.Master
	var err error

	e.pf, err = m.CreateProductFamily(ctx, "PF-LOGIC", "逻辑芯片")
	if err != nil {
		t.Fatalf("产品族: %v", err)
	}
	e.st1, err = m.CreateStation(ctx, "ETCH", "刻蚀", "etch")
	if err != nil {
		t.Fatalf("站点1: %v", err)
	}
	e.st2, err = m.CreateStation(ctx, "CLEAN", "清洗", "clean")
	if err != nil {
		t.Fatalf("站点2: %v", err)
	}
	e.rc1, err = m.CreateRecipe(ctx, "RCP-ETCH", "刻蚀配方", "FAMILY-A")
	if err != nil {
		t.Fatalf("配方1: %v", err)
	}
	e.rc2, err = m.CreateRecipe(ctx, "RCP-CLEAN", "清洗配方", "FAMILY-A")
	if err != nil {
		t.Fatalf("配方2: %v", err)
	}
	e.plan1, err = m.CreatePlan(ctx, "MP-ETCH", "刻蚀量测", "cd", []int{1, 2}, 2, 10.0)
	if err != nil {
		t.Fatalf("计划1: %v", err)
	}
	e.plan2, err = m.CreatePlan(ctx, "MP-CLEAN", "清洗量测", "particle", []int{1}, 1, 5.0)
	if err != nil {
		t.Fatalf("计划2: %v", err)
	}
	if _, err = m.ActivatePlan(ctx, e.plan1.ID, e.plan1.RowVersion); err != nil {
		t.Fatalf("启用计划1: %v", err)
	}
	if _, err = m.ActivatePlan(ctx, e.plan2.ID, e.plan2.RowVersion); err != nil {
		t.Fatalf("启用计划2: %v", err)
	}
	e.route, err = m.CreateRoute(ctx, e.pf.ID, "RT-MAIN", "主路线")
	if err != nil {
		t.Fatalf("路线: %v", err)
	}
	e.rev, err = m.CreateRevision(ctx, e.route.ID, []domain.RouteStation{
		{Seq: 1, StationID: e.st1.ID, RecipeID: e.rc1.ID, MetrologyPlanID: e.plan1.ID},
		{Seq: 2, StationID: e.st2.ID, RecipeID: e.rc2.ID, MetrologyPlanID: e.plan2.ID},
	})
	if err != nil {
		t.Fatalf("修订: %v", err)
	}
}

// activateVersions 启用配方 v1 与路线修订。
func (e *testEnv) activateVersions() {
	t := e.t
	ctx := e.ctx
	m := e.svc.Master

	v1, err := m.CreateRecipeVersion(ctx, e.rc1.ID, json.RawMessage(`{"temp":100}`))
	if err != nil {
		t.Fatalf("配方1版本: %v", err)
	}
	if _, err := m.ActivateRecipeVersion(ctx, v1.ID, v1.RowVersion); err != nil {
		t.Fatalf("启用配方1版本: %v", err)
	}
	v2, err := m.CreateRecipeVersion(ctx, e.rc2.ID, json.RawMessage(`{"flow":20}`))
	if err != nil {
		t.Fatalf("配方2版本: %v", err)
	}
	if _, err := m.ActivateRecipeVersion(ctx, v2.ID, v2.RowVersion); err != nil {
		t.Fatalf("启用配方2版本: %v", err)
	}
	if _, err := m.ActivateRevision(ctx, e.rev.ID, e.rev.Version); err != nil {
		t.Fatalf("启用修订: %v", err)
	}
}

// setupEquipment 创建设备与腔体；withLongQual 为真时创建覆盖 [base-1h, base+24h] 的资质。
func (e *testEnv) setupEquipment(withLongQual bool) {
	t := e.t
	ctx := e.ctx
	m := e.svc.Master
	var err error

	e.eq1, err = m.CreateEquipment(ctx, "EQ-ETCH-1", "刻蚀机1", "FAMILY-A", e.st1.ID)
	if err != nil {
		t.Fatalf("设备1: %v", err)
	}
	e.ch1, err = m.CreateChamber(ctx, e.eq1.ID, "CH-A", "etch")
	if err != nil {
		t.Fatalf("腔体1: %v", err)
	}
	e.eq2, err = m.CreateEquipment(ctx, "EQ-CLEAN-1", "清洗机1", "FAMILY-A", e.st2.ID)
	if err != nil {
		t.Fatalf("设备2: %v", err)
	}
	e.ch2, err = m.CreateChamber(ctx, e.eq2.ID, "CH-A", "clean")
	if err != nil {
		t.Fatalf("腔体2: %v", err)
	}
	if !withLongQual {
		return
	}
	from := baseTime.Add(-time.Hour)
	to := baseTime.Add(24 * time.Hour)
	if _, err := m.CreateQualification(ctx, e.eq1.ID, "", e.st1.ID, from, to); err != nil {
		t.Fatalf("资质1: %v", err)
	}
	if _, err := m.CreateQualification(ctx, e.eq2.ID, "", e.st2.ID, from, to); err != nil {
		t.Fatalf("资质2: %v", err)
	}
}

// setupAll 主数据 + 启用 + 设备 + 长窗口资质。
func (e *testEnv) setupAll() {
	e.setupMaster()
	e.activateVersions()
	e.setupEquipment(true)
}

// registerLot 登记 3 片晶圆的批次。
func (e *testEnv) registerLot(code string) *domain.Lot {
	t := e.t
	lot, _, err := e.svc.Lot.RegisterLot(e.ctx, code, e.pf.ID, e.route.ID, []service.WaferInput{
		{Code: code + "-W1", Slot: 1},
		{Code: code + "-W2", Slot: 2},
		{Code: code + "-W3", Slot: 3},
	}, "")
	if err != nil {
		t.Fatalf("登记批次: %v", err)
	}
	return lot
}

// enterAndRun 进站并开工。
func (e *testEnv) enterAndRun(lotID string) *domain.Run {
	t := e.t
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	run, _, err := e.svc.Run.CreateRun(e.ctx, lotID, e.eq1.ID, e.ch1.ID, nil, "")
	if err != nil {
		t.Fatalf("开工: %v", err)
	}
	return run
}

// completeAndSeal 完工、量测、封存。
func (e *testEnv) completeAndSeal(runID string, value float64) *domain.Run {
	t := e.t
	run, _, err := e.svc.Run.CompleteRun(e.ctx, runID, "")
	if err != nil {
		t.Fatalf("完工: %v", err)
	}
	lotWafers, err := e.svc.Lot.ListWafers(e.ctx, run.LotID)
	if err != nil {
		t.Fatalf("查询晶圆: %v", err)
	}
	var inputs []service.ReadingInput
	for _, w := range lotWafers {
		if w.Slot == 1 || w.Slot == 2 {
			inputs = append(inputs, service.ReadingInput{WaferID: w.ID, Metric: "cd", Value: value})
		}
	}
	if _, _, err := e.svc.Reading.SubmitReadings(e.ctx, runID, inputs, ""); err != nil {
		t.Fatalf("量测: %v", err)
	}
	sealed, _, err := e.svc.Reading.Seal(e.ctx, runID, "")
	if err != nil {
		t.Fatalf("封存: %v", err)
	}
	return sealed
}
