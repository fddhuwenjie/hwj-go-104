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
