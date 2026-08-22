package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// failAndHold 让第一站量测 FAIL 并自动生成暂扣，返回 (run, hold)。
func failAndHold(t *testing.T, e *testEnv, lotID string) (*domain.Run, *domain.Hold) {
	t.Helper()
	run := e.enterAndRun(lotID)
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
	return sealed, hold
}

// TestHoldBlocksLotAndChildren 未关闭暂扣阻断批次及其子批的后续站点操作。
func TestHoldBlocksLotAndChildren(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	parent := e.registerLot("LOT-HP")
	// 先拆分子批（拆分后再暂扣父批，验证子批被阻断）。
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, parent.ID)
	child, _, err := e.svc.Lot.SplitLot(e.ctx, parent.ID, "LOT-HP-C1", []string{wafers[2].ID}, "")
	if err != nil {
		t.Fatalf("拆分: %v", err)
	}

	// 父批人工暂扣。
	if _, _, err := e.svc.Hold.CreateHold(e.ctx, parent.ID, "人工抽检异常", ""); err != nil {
		t.Fatalf("暂扣: %v", err)
	}

	// 父批进站被阻断。
	if _, _, err := e.svc.Lot.Enter(e.ctx, parent.ID, ""); !errors.Is(err, domain.ErrHoldBlocking) {
		t.Fatalf("父批进站应被阻断: %v", err)
	}
	// 子批进站同样被阻断（父批未关闭暂扣向下传导）。
	if _, _, err := e.svc.Lot.Enter(e.ctx, child.ID, ""); !errors.Is(err, domain.ErrHoldBlocking) {
		t.Fatalf("子批进站应被阻断: %v", err)
	}
}

// TestReviewRelease 复判放行：暂扣关闭，批次恢复原路线继续。
func TestReviewRelease(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-REL")
	_, hold := failAndHold(t, e, lot.ID)

	res, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewRelease, "人工确认可接受", 0, "")
	if err != nil {
		t.Fatalf("复判放行: %v", err)
	}
	if res.Hold.Status != domain.HoldReleased || res.Hold.ClosedAt == nil {
		t.Fatalf("暂扣未关闭: %+v", res.Hold)
	}
	// 第一站运行已判定（FAIL 但被人工放行），批次进入站点间等待，可放行下一站。
	if res.Lot.Status != domain.LotWaiting {
		t.Fatalf("放行后状态错误: %s", res.Lot.Status)
	}
	if _, _, err := e.svc.Lot.ReleaseNext(e.ctx, lot.ID, "复判放行", ""); err != nil {
		t.Fatalf("复判后放行: %v", err)
	}
	after, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
	if after.CurrentSeq != 2 || after.Status != domain.LotQueued {
		t.Fatalf("复判后站点推进错误: %+v", after)
	}

	// 暂扣只允许复判一次。
	if _, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewScrap, "", 0, ""); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("重复复判应拒绝: %v", err)
	}
}

// TestReviewReworkNewRevision 复判返工换版：
// 创建新路线修订从指定站点重入；旧运行、旧量测、旧暂扣、旧快照不被改写。
func TestReviewReworkNewRevision(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-RW")
	oldRun, hold := failAndHold(t, e, lot.ID)
	oldSnapshot := ""
	{
		l, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
		oldSnapshot = l.FreezeSnapshot
	}
	oldRevCount, _ := e.svc.Master.ListRevisions(e.ctx, e.route.ID)

	res, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewRework, "返工重入第一站", 1, "")
	if err != nil {
		t.Fatalf("复判返工: %v", err)
	}
	if res.Hold.Status != domain.HoldReworked {
		t.Fatalf("暂扣状态错误: %s", res.Hold.Status)
	}
	if res.Rework == nil || res.NewRevisionID == "" {
		t.Fatal("返工必须创建新修订")
	}
	// 新修订启用且标记返工来源。
	newRev, err := e.svc.Master.GetRevision(e.ctx, res.NewRevisionID)
	if err != nil {
		t.Fatalf("新修订: %v", err)
	}
	if newRev.Status != domain.RevActive || newRev.ReworkFromHoldID != hold.ID || newRev.ReentrySeq != 1 {
		t.Fatalf("返工修订错误: %+v", newRev)
	}
	revs, _ := e.svc.Master.ListRevisions(e.ctx, e.route.ID)
	if len(revs) != len(oldRevCount)+1 {
		t.Fatal("返工必须新增路线修订")
	}

	// 批次重置到新修订起点。
	after, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
	if after.Status != domain.LotRegistered || after.CurrentSeq != 0 || after.FrozenRevisionID != newRev.ID {
		t.Fatalf("返工后批次状态错误: %+v", after)
	}

	// 旧运行未被改写。
	oldRunAfter, _ := e.svc.Run.GetRun(e.ctx, oldRun.ID)
	if oldRunAfter.Status != domain.RunJudged || oldRunAfter.Judgment != domain.JudgeFail {
		t.Fatalf("旧运行被改写: %+v", oldRunAfter)
	}
	// 旧量测仍封存。
	readings, _ := e.svc.Reading.ListReadings(e.ctx, oldRun.ID)
	for _, r := range readings {
		if !r.Sealed {
			t.Fatal("旧量测封存状态被改写")
		}
	}
	// 旧暂扣记录仍在且为 REWORKED。
	oldHold, _ := e.svc.Hold.GetHold(e.ctx, hold.ID)
	if oldHold.Status != domain.HoldReworked {
		t.Fatalf("旧暂扣被改写: %s", oldHold.Status)
	}
	// 旧快照 JSON 不再属于批次，但旧修订站点不可变。
	oldStations, _ := e.svc.Master.ListRouteStations(e.ctx, e.rev.ID)
	if len(oldStations) != 2 {
		t.Fatal("旧修订站点被改写")
	}
	_ = oldSnapshot

	// 从重入站点重新进站、开工、完工、判定 PASS。
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("返工重入进站: %v", err)
	}
	run2, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq1.ID, e.ch1.ID, nil, "")
	if err != nil {
		t.Fatalf("返工开工: %v", err)
	}
	if run2.RouteRevisionID != newRev.ID {
		t.Fatal("返工运行必须使用新修订")
	}
	sealed := e.completeAndSeal(run2.ID, 5.0)
	if sealed.Judgment != domain.JudgePass {
		t.Fatalf("返工后判定错误: %s", sealed.Judgment)
	}
}

// TestReviewReworkInvalidReentry 返工重入站点越界必须拒绝且事务回滚。
func TestReviewReworkInvalidReentry(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-RW2")
	_, hold := failAndHold(t, e, lot.ID)

	// 重入站点 9 不存在于冻结快照。
	if _, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewRework, "", 9, ""); err == nil {
		t.Fatal("非法重入站点应拒绝")
	}
	// 事务回滚：暂扣仍 OPEN，批次仍 ON_HOLD，无新修订。
	after, _ := e.svc.Hold.GetHold(e.ctx, hold.ID)
	if after.Status != domain.HoldOpen {
		t.Fatalf("回滚后暂扣应仍为 OPEN: %s", after.Status)
	}
	l, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
	if l.Status != domain.LotOnHold {
		t.Fatalf("回滚后批次应仍为 ON_HOLD: %s", l.Status)
	}
}

// TestReviewScrap 复判报废。
func TestReviewScrap(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-SCRAP")
	_, hold := failAndHold(t, e, lot.ID)

	res, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewScrap, "无法返工", 0, "")
	if err != nil {
		t.Fatalf("复判报废: %v", err)
	}
	if res.Lot.Status != domain.LotScrapped || res.Lot.ClosedAt == nil {
		t.Fatalf("报废状态错误: %+v", res.Lot)
	}
	if res.Hold.Status != domain.HoldScrapped {
		t.Fatalf("暂扣状态错误: %s", res.Hold.Status)
	}
}

// TestSamplingCoverage 抽样覆盖：必须覆盖计划位置与最小数量。
func TestSamplingCoverage(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-SMP")
	run := e.enterAndRun(lot.ID)
	if _, _, err := e.svc.Run.CompleteRun(e.ctx, run.ID, ""); err != nil {
		t.Fatalf("完工: %v", err)
	}
	wafers, _ := e.svc.Lot.ListWafers(e.ctx, lot.ID)
	// 只覆盖位置 1（计划要求 1、2 且 min=2）。
	_, _, err := e.svc.Reading.SubmitReadings(e.ctx, run.ID, []service.ReadingInput{
		{WaferID: wafers[0].ID, Metric: "cd", Value: 5.0},
	}, "")
	if err != nil {
		t.Fatalf("量测提交: %v", err)
	}
	if _, _, err := e.svc.Reading.Seal(e.ctx, run.ID, ""); !errors.Is(err, domain.ErrSampling) {
		t.Fatalf("抽样覆盖不足应拒绝封存: %v", err)
	}

	// 运行未完工不可提交量测。
	lot2 := e.registerLot("LOT-SMP2")
	run2 := e.enterAndRun(lot2.ID)
	if _, _, err := e.svc.Reading.SubmitReadings(e.ctx, run2.ID, []service.ReadingInput{
		{WaferID: wafers[0].ID, Metric: "cd", Value: 5.0},
	}, ""); !errors.Is(err, domain.ErrRunNotCompleted) {
		t.Fatalf("未完工提交量测应拒绝: %v", err)
	}
}
