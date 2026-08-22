package service_test

import (
	"encoding/json"
	"strings"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// TestSnapshotIsolation 版本快照隔离：
// 批次 A 冻结配方 v1 后，配方升级到 v2 并启用；
// 批次 A 的运行仍使用 v1 快照，新批次 B 冻结 v2 快照，互不影响。
func TestSnapshotIsolation(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lotA := e.registerLot("LOT-V1")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotA.ID, ""); err != nil {
		t.Fatalf("A 进站: %v", err)
	}
	runA, _, err := e.svc.Run.CreateRun(e.ctx, lotA.ID, e.eq1.ID, e.ch1.ID, nil, "")
	if err != nil {
		t.Fatalf("A 开工: %v", err)
	}
	if !strings.Contains(runA.RecipeSnapshot, `"version":1`) {
		t.Fatalf("A 运行应使用 v1 快照: %s", runA.RecipeSnapshot)
	}

	// 配方升级到 v2 并启用（v1 退役）。
	v2, err := e.svc.Master.CreateRecipeVersion(e.ctx, e.rc1.ID, json.RawMessage(`{"temp":200}`))
	if err != nil {
		t.Fatalf("v2 草稿: %v", err)
	}
	if _, err := e.svc.Master.ActivateRecipeVersion(e.ctx, v2.ID, v2.RowVersion); err != nil {
		t.Fatalf("启用 v2: %v", err)
	}

	lotB := e.registerLot("LOT-V2")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotB.ID, ""); err != nil {
		t.Fatalf("B 进站: %v", err)
	}
	runB, _, err := e.svc.Run.CreateRun(e.ctx, lotB.ID, e.eq1.ID, e.ch1.ID, nil, "")
	if err != nil {
		t.Fatalf("B 开工: %v", err)
	}
	if !strings.Contains(runB.RecipeSnapshot, `"version":2`) {
		t.Fatalf("B 运行应使用 v2 快照: %s", runB.RecipeSnapshot)
	}

	// 版本升级后 A 的冻结快照不可变：仍指向 v1。
	afterA, _ := e.svc.Lot.GetLot(e.ctx, lotA.ID)
	snapA, err := domain.DecodeFreezeSnapshot(afterA.FreezeSnapshot)
	if err != nil {
		t.Fatalf("快照解码: %v", err)
	}
	if !strings.Contains(snapA.Stations[0].RecipeSnapshot, `"version":1`) {
		t.Fatal("批次 A 冻结快照在配方升级后必须保持 v1")
	}
	// A 完工判定不受版本升级影响。
	sealed := e.completeAndSeal(runA.ID, 5.0)
	if sealed.Judgment != "PASS" {
		t.Fatalf("A 判定错误: %s", sealed.Judgment)
	}
}

// TestLateReading 迟到量测：判定后补录读数附着原运行，
// 标记 late 且不覆盖当前有效判定。
func TestLateReading(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-LATE")
	run := e.enterAndRun(lot.ID)
	sealed := e.completeAndSeal(run.ID, 5.0) // PASS
	if sealed.Judgment != "PASS" {
		t.Fatalf("判定错误: %s", sealed.Judgment)
	}

	// 迟到量测：超大值，若覆盖判定会变 FAIL。
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, lot.ID)
	inputs := []service.ReadingInput{{WaferID: wafers[0].ID, Metric: "cd", Value: 999.0}}
	readings, _, err := e.svc.Reading.SubmitReadings(e.ctx, run.ID, inputs, "")
	if err != nil {
		t.Fatalf("迟到量测应允许: %v", err)
	}
	if !readings[0].Late {
		t.Fatal("判定后提交的读数必须标记 late")
	}
	after, _ := e.svc.Run.GetRun(e.ctx, run.ID)
	if after.Judgment != "PASS" {
		t.Fatalf("迟到量测不得覆盖有效判定: %s", after.Judgment)
	}
}
