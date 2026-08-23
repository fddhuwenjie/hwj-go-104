package service_test

import (
	"testing"
	"time"

	"gowork/wafer/internal/domain"
)

// TestStablePagination 稳定分页：游标遍历不重复、不遗漏，
// 遍历期间追加数据不影响已开始的遍历。
func TestStablePagination(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	// 造 7 个批次，创建时间错开。
	for i, code := range []string{"P1", "P2", "P3", "P4", "P5", "P6", "P7"} {
		e.clk.Set(baseTime.Add(time.Duration(i) * time.Minute))
		e.registerLot("LOT-" + code)
	}

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page := domain.Page{Limit: 3, Cursor: cursor}
		lots, next, err := e.svc.Lot.ListLots(e.ctx, page)
		if err != nil {
			t.Fatalf("分页查询: %v", err)
		}
		for _, l := range lots {
			if seen[l.ID] {
				t.Fatalf("分页重复: %s", l.Code)
			}
			seen[l.ID] = true
		}
		pages++
		if next == "" {
			break
		}
		cursor = next
		if pages == 1 {
			// 遍历期间追加新批次，不应影响已开始的游标遍历（新数据排在游标之后可见）。
			e.clk.Set(baseTime.Add(time.Hour))
			e.registerLot("LOT-LATE-ADD")
		}
		if pages > 10 {
			t.Fatal("分页未收敛")
		}
	}
	if len(seen) != 8 {
		t.Fatalf("分页遗漏或重复: %d", len(seen))
	}
}

// TestWipLotsQuery 在制批次查询：携带冻结修订与最近暂扣原因。
func TestWipLotsQuery(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-WIP")
	e.enterAndRun(lot.ID)
	if _, _, err := e.svc.Hold.CreateHold(e.ctx, lot.ID, "抽检异常A", ""); err != nil {
		t.Fatalf("暂扣: %v", err)
	}

	items, _, err := e.svc.Query.WipLots(e.ctx, domain.Page{Limit: 10})
	if err != nil {
		t.Fatalf("在制查询: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("在制数量错误: %d", len(items))
	}
	if items[0].FrozenRevision == nil || *items[0].FrozenRevision != 1 {
		t.Fatalf("在制视图必须携带冻结修订号: %+v", items[0])
	}
	if items[0].LatestHoldReason != "抽检异常A" {
		t.Fatalf("最近暂扣原因错误: %q", items[0].LatestHoldReason)
	}
	if items[0].FrozenAt == nil {
		t.Fatal("在制视图必须携带冻结时间")
	}
}

// TestStationQueuesQuery 站点队列：等待超阈值且设备能力可用才返回。
func TestStationQueuesQuery(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-Q")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	// 等待 10 分钟。
	e.clk.Advance(10 * time.Minute)

	// 阈值 5 分钟：应返回。
	items, err := e.svc.Query.StationQueues(e.ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("队列查询: %v", err)
	}
	if len(items) != 1 || items[0].LotID != lot.ID {
		t.Fatalf("队列查询错误: %+v", items)
	}
	if items[0].WaitSeconds < 600 || items[0].CapableEquipment < 1 {
		t.Fatalf("队列等待或能力统计错误: %+v", items[0])
	}
	// 阈值 1 小时：不返回。
	items, err = e.svc.Query.StationQueues(e.ctx, time.Hour)
	if err != nil || len(items) != 0 {
		t.Fatalf("阈值过滤错误: %v %+v", err, items)
	}

	// 设备停机后：无可用能力，不返回。
	if _, err := e.svc.Master.SetEquipmentStatus(e.ctx, e.eq1.ID, domain.EquipDown, e.eq1.Version); err != nil {
		t.Fatalf("设备停机: %v", err)
	}
	items, err = e.svc.Query.StationQueues(e.ctx, 5*time.Minute)
	if err != nil || len(items) != 0 {
		t.Fatalf("能力过滤错误: %v %+v", err, items)
	}
}

// TestStationQueuesCountsEquipmentOnce 多个合格腔体不能把同一设备重复计数。
func TestStationQueuesCountsEquipmentOnce(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()
	if _, err := e.svc.Master.CreateChamber(e.ctx, e.eq1.ID, "CH-B", "etch,backup"); err != nil {
		t.Fatalf("新增腔体: %v", err)
	}
	lot := e.registerLot("LOT-Q-UNIQUE")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	e.clk.Advance(10 * time.Minute)
	items, err := e.svc.Query.StationQueues(e.ctx, 5*time.Minute)
	if err != nil || len(items) != 1 {
		t.Fatalf("队列查询: %v %+v", err, items)
	}
	if items[0].CapableEquipment != 1 {
		t.Fatalf("同一设备被多个腔体重复计数: %+v", items[0])
	}
}

// TestReworkStatsQuery 重复返工聚合：按设备腔体与配方版本分组。
func TestReworkStatsQuery(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-RS")
	// 两次返工：第一站 FAIL -> 返工重入 -> 再 FAIL -> 再返工。
	for i := 0; i < 2; i++ {
		_, hold := failAndHold(t, e, lot.ID)
		if _, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewRework, "返工", 1, ""); err != nil {
			t.Fatalf("返工 %d: %v", i, err)
		}
	}
	stats, err := e.svc.Query.ReworkStats(e.ctx)
	if err != nil {
		t.Fatalf("返工聚合: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("聚合行数错误: %+v", stats)
	}
	if stats[0].EquipmentID != e.eq1.ID || stats[0].ChamberID != e.ch1.ID || stats[0].ReworkLots != 1 {
		t.Fatalf("聚合内容错误: %+v", stats[0])
	}
	if stats[0].RecipeVersionID == "" {
		t.Fatal("聚合必须携带配方版本")
	}
}

// enterAndRunChamber 进站并开工到指定设备腔体（用于多腔体场景）。
func (e *testEnv) enterAndRunChamber(lotID, equipmentID, chamberID string) *domain.Run {
	t := e.t
	if _, _, err := e.svc.Lot.Enter(e.ctx, lotID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	run, _, err := e.svc.Run.CreateRun(e.ctx, lotID, equipmentID, chamberID, nil, "")
	if err != nil {
		t.Fatalf("开工: %v", err)
	}
	return run
}

// failAndHoldChamber 让第一站量测 FAIL 并自动生成暂扣，运行到指定设备腔体上。
func failAndHoldChamber(t *testing.T, e *testEnv, lotID, equipmentID, chamberID string) *domain.Hold {
	t.Helper()
	run := e.enterAndRunChamber(lotID, equipmentID, chamberID)
	sealed := e.completeAndSeal(run.ID, 999.0) // 超过阈值 10
	if sealed.Judgment != domain.JudgeFail {
		t.Fatalf("判定应为 FAIL: %s", sealed.Judgment)
	}
	lot, _ := e.svc.Lot.GetLot(e.ctx, lotID)
	if lot.Status != domain.LotOnHold {
		t.Fatalf("判定失败批次应为 ON_HOLD: %s", lot.Status)
	}
	hold, err := e.store.LatestHold(e.ctx, lotID)
	if err != nil {
		t.Fatalf("暂扣不存在: %v", err)
	}
	return hold
}

// TestReworkStatsMultiCombination 同一设备的两个腔体分别产生重复返工时，
// 设备、腔体、配方版本三个维度必须分别保留：
// 同设备两腔体各经历一次返工 -> 两个 REWORKED 暂扣落在不同腔体 ->
// 重复返工聚合必须返回两行，每行 rework_lots=1，腔体不同。
func TestReworkStatsMultiCombination(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	// 同设备 eq1 下新增第二个腔体 CH-B（能力同 CH-A，覆盖站点 etch）。
	chB, err := e.svc.Master.CreateChamber(e.ctx, e.eq1.ID, "CH-B", "etch")
	if err != nil {
		t.Fatalf("新增腔体: %v", err)
	}

	lot := e.registerLot("LOT-RS-MC")
	// 两轮返工：第一轮落在腔体 A，第二轮落在腔体 B。
	// 每轮 FAIL -> 复判返工（换版重入第一站）。两轮后该批次即“重复返工”批次。
	chambers := []string{e.ch1.ID, chB.ID}
	for i, chID := range chambers {
		hold := failAndHoldChamber(t, e, lot.ID, e.eq1.ID, chID)
		if _, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewRework, "返工", 1, ""); err != nil {
			t.Fatalf("返工 %d: %v", i, err)
		}
	}

	stats, err := e.svc.Query.ReworkStats(e.ctx)
	if err != nil {
		t.Fatalf("返工聚合: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("同设备两腔体必须各自保留聚合行，得到 %d 行: %+v", len(stats), stats)
	}
	// 两行同设备，腔体不同，配方版本相同（返工沿用启用版本），各 rework_lots=1。
	seenChambers := map[string]int{}
	for _, s := range stats {
		if s.EquipmentID != e.eq1.ID {
			t.Fatalf("设备维度错误: %+v", s)
		}
		if s.RecipeVersionID == "" {
			t.Fatalf("配方版本维度缺失: %+v", s)
		}
		if s.ReworkLots != 1 {
			t.Fatalf("每个腔体的重复返工批次应为 1，得到 %d: %+v", s.ReworkLots, s)
		}
		seenChambers[s.ChamberID]++
	}
	if seenChambers[e.ch1.ID] != 1 || seenChambers[chB.ID] != 1 {
		t.Fatalf("腔体维度被折叠，期望两个腔体各一行，实际: %+v", stats)
	}
}

// TestGenealogyAuditQuery 谱系审计：父批报废但子批在制 -> STATUS_MISMATCH。
func TestGenealogyAuditQuery(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-GA")
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)
	child, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-GA-C1", []string{wafers[2].ID}, "")
	if err != nil {
		t.Fatalf("拆分: %v", err)
	}
	if _, _, err := e.svc.Lot.Scrap(e.ctx, parent.ID, "父批污染", ""); err != nil {
		t.Fatalf("报废: %v", err)
	}
	issues, err := e.svc.Query.GenealogyAudit(e.ctx)
	if err != nil {
		t.Fatalf("审计: %v", err)
	}
	found := false
	for _, is := range issues {
		if is.Issue == "STATUS_MISMATCH" && is.LotID == child.ID && is.Related == parent.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("应检出父子状态不一致: %+v", issues)
	}
}
