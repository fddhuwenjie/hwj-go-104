package service_test

import (
	"errors"
	"fmt"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// TestEmptyIdempotencyKeyStillUsesTransaction 验证不传幂等键时业务写入仍保持原子性。
func TestEmptyIdempotencyKeyStillUsesTransaction(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()
	_, _, err := e.svc.Lot.RegisterLot(e.ctx, "ATOMIC-NO-KEY", e.pf.ID, e.route.ID, []service.WaferInput{
		{Code: "DUPLICATE-WAFER", Slot: 1},
		{Code: "DUPLICATE-WAFER", Slot: 2},
	}, "")
	if err == nil {
		t.Fatal("重复晶圆编码应导致登记失败")
	}
	if _, err := e.store.FindLotByCode(e.ctx, "ATOMIC-NO-KEY"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("失败登记不应残留批次: %v", err)
	}
}

// TestIdempotentReplay 幂等重放：相同幂等键的重复请求返回同一结果，
// 不产生重复副作用（批次、晶圆、运行数量不翻倍）。
func TestIdempotentReplay(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	first, replay1, err := e.svc.Lot.RegisterLot(e.ctx, "LOT-IDEM", e.pf.ID, e.route.ID, waferInputs("LOT-IDEM", 3), "key-1")
	if err != nil {
		t.Fatalf("首次登记: %v", err)
	}
	if replay1 {
		t.Fatal("首次请求不应标记重放")
	}
	second, replay2, err := e.svc.Lot.RegisterLot(e.ctx, "LOT-IDEM", e.pf.ID, e.route.ID, waferInputs("LOT-IDEM", 3), "key-1")
	if err != nil {
		t.Fatalf("重放登记: %v", err)
	}
	if !replay2 {
		t.Fatal("重复幂等键必须标记重放")
	}
	if first.ID != second.ID {
		t.Fatalf("重放必须返回同一批次: %s vs %s", first.ID, second.ID)
	}
	// 无重复副作用。
	lots, _, _ := e.svc.Lot.ListLots(e.ctx, domain.Page{Limit: 100})
	count := 0
	for _, l := range lots {
		if l.Code == "LOT-IDEM" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("重复登记产生副作用: %d 个批次", count)
	}
	ws, _ := e.svc.Lot.ListWafers(e.ctx, first.ID)
	if len(ws) != 3 {
		t.Fatalf("晶圆数量错误: %d", len(ws))
	}

	// 进站幂等重放。
	if _, _, err := e.svc.Lot.Enter(e.ctx, first.ID, "enter-key"); err != nil {
		t.Fatalf("进站: %v", err)
	}
	entered, replay, err := e.svc.Lot.Enter(e.ctx, first.ID, "enter-key")
	if err != nil {
		t.Fatalf("进站重放: %v", err)
	}
	if !replay || entered.CurrentSeq != 1 {
		t.Fatalf("进站重放错误: replay=%v seq=%d", replay, entered.CurrentSeq)
	}

	// 开工幂等重放。
	run1, _, err := e.svc.Run.CreateRun(e.ctx, first.ID, e.eq1.ID, e.ch1.ID, nil, "run-key")
	if err != nil {
		t.Fatalf("开工: %v", err)
	}
	run2, replay, err := e.svc.Run.CreateRun(e.ctx, first.ID, e.eq1.ID, e.ch1.ID, nil, "run-key")
	if err != nil {
		t.Fatalf("开工重放: %v", err)
	}
	if !replay || run1.ID != run2.ID {
		t.Fatalf("开工重放必须返回同一运行")
	}
	runs, _ := e.svc.Run.ListRunsByLot(e.ctx, first.ID)
	if len(runs) != 1 {
		t.Fatalf("幂等重放不得重复创建运行: %d", len(runs))
	}
}

// waferInputs 构造晶圆输入。
func waferInputs(prefix string, n int) []service.WaferInput {
	var out []service.WaferInput
	for i := 1; i <= n; i++ {
		out = append(out, service.WaferInput{Code: fmt.Sprintf("%s-W%d", prefix, i), Slot: i})
	}
	return out
}

// TestOptimisticLockConflict 乐观锁冲突：陈旧版本号的状态更新必须返回冲突。
func TestOptimisticLockConflict(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()

	// 设备状态并发更新：先用旧版本更新成功，再用同一旧版本更新必须冲突。
	e.activateVersions()
	e.setupEquipment(false)

	eq := e.eq1
	if _, err := e.svc.Master.SetEquipmentStatus(e.ctx, eq.ID, domain.EquipDown, eq.Version); err != nil {
		t.Fatalf("首次状态更新: %v", err)
	}
	if _, err := e.svc.Master.SetEquipmentStatus(e.ctx, eq.ID, domain.EquipActive, eq.Version); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("陈旧版本必须冲突: %v", err)
	}

	// 修订启用并发冲突。
	rev, err := e.svc.Master.CreateRevision(e.ctx, e.route.ID, []domain.RouteStation{
		{Seq: 1, StationID: e.st1.ID, RecipeID: e.rc1.ID, MetrologyPlanID: e.plan1.ID},
	})
	if err != nil {
		t.Fatalf("修订: %v", err)
	}
	if _, err := e.svc.Master.ActivateRevision(e.ctx, rev.ID, rev.Version+9); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("错误版本号必须冲突: %v", err)
	}
}

// TestSetEquipmentStatusConcurrentInterleave 设备停机并发交错：
// 终端 A、B 同时读到 v1/ACTIVE。A 先停机成功推进到 v2/DOWN；
// B 仍用打开记录时的旧版本 v1 再次提交停机（目标状态相同）。
// 修复前：相同目标状态被当作“无变化”绕过乐观锁，仓储 UPDATE 不校验版本，
//   操作成功且把版本推进到 v3（无意义的空写）。
// 修复后：版本号不匹配即冲突，与目标状态是否相同无关，B 必须拿到 ErrConflict，
//   版本不得被推进；随后 B 用更到的 v2 改回启用才会成功。
func TestSetEquipmentStatusConcurrentInterleave(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.activateVersions()
	e.setupEquipment(false)

	eq := e.eq1
	if eq.Status != domain.EquipActive || eq.Version != 1 {
		t.Fatalf("初始状态错误: status=%s version=%d", eq.Status, eq.Version)
	}

	// 两个终端各自读到同一快照 v1/ACTIVE（模拟打开记录）。
	stale := eq.Version // 1

	// 终端 A 先停机成功，版本 v1 -> v2。
	if _, err := e.svc.Master.SetEquipmentStatus(e.ctx, eq.ID, domain.EquipDown, stale); err != nil {
		t.Fatalf("终端A停机: %v", err)
	}
	current, err := e.svc.Master.GetEquipment(e.ctx, eq.ID)
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if current.Status != domain.EquipDown || current.Version != 2 {
		t.Fatalf("终端A停机后状态错误: status=%s version=%d", current.Status, current.Version)
	}

	// 终端 B 用打开时的旧版本 v1 再次提交停机（相同目标状态）。
	// 修复前：相同目标状态被绕过，操作成功并推进版本到 3。
	// 修复后：版本不匹配，必须冲突，版本不得被推进。
	if _, err := e.svc.Master.SetEquipmentStatus(e.ctx, eq.ID, domain.EquipDown, stale); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("相同目标状态的陈旧版本必须冲突，不得绕过乐观锁: %v", err)
	}
	afterB, err := e.svc.Master.GetEquipment(e.ctx, eq.ID)
	if err != nil {
		t.Fatalf("查询B后: %v", err)
	}
	if afterB.Version != 2 || afterB.Status != domain.EquipDown {
		t.Fatalf("陈旧版本提交不得推进版本或改变状态: status=%s version=%d", afterB.Status, afterB.Version)
	}

	// 终端 B 拿到正确版本 v2 后改回启用，应成功并推进到 v3。
	restored, err := e.svc.Master.SetEquipmentStatus(e.ctx, eq.ID, domain.EquipActive, afterB.Version)
	if err != nil {
		t.Fatalf("终端B改回启用: %v", err)
	}
	if restored.Status != domain.EquipActive || restored.Version != 3 {
		t.Fatalf("改回启用后状态错误: status=%s version=%d", restored.Status, restored.Version)
	}

	// 终端 A 持有的 v2 已过期，改回启用必须冲突。
	if _, err := e.svc.Master.SetEquipmentStatus(e.ctx, eq.ID, domain.EquipActive, afterB.Version); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("陈旧版本改回启用必须冲突: %v", err)
	}
}
