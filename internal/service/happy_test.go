package service_test

import (
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// TestFullHappyChain 完整正常业务链：
// 主数据建档 -> 版本启用 -> 批次登记 -> 进站冻结 -> 开工 -> 完工 ->
// 量测采集 -> 封存判定 -> 下一站放行 -> 第二站重复 -> 末站放行 -> 批次关闭。
func TestFullHappyChain(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-001")

	// 第一站：进站（触发冻结）。
	entered, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, "")
	if err != nil {
		t.Fatalf("进站: %v", err)
	}
	if entered.Status != domain.LotQueued || entered.CurrentSeq != 1 {
		t.Fatalf("进站状态错误: %+v", entered)
	}
	if !entered.IsFrozen() {
		t.Fatal("首次进站必须冻结路线")
	}
	if entered.FrozenRevisionID != e.rev.ID {
		t.Fatalf("冻结修订错误: %s", entered.FrozenRevisionID)
	}

	// 冻结快照校验：站点顺序与配方快照固化。
	snap, err := domain.DecodeFreezeSnapshot(entered.FreezeSnapshot)
	if err != nil {
		t.Fatalf("快照解码: %v", err)
	}
	if len(snap.Stations) != 2 || snap.Stations[0].Seq != 1 || snap.Stations[1].Seq != 2 {
		t.Fatalf("快照站点顺序错误: %+v", snap.Stations)
	}
	if snap.Stations[0].RecipeSnapshot == "" || snap.Stations[0].PlanSnapshot == "" {
		t.Fatal("快照必须包含配方快照与量测计划")
	}

	// 开工、完工、量测、封存（第一站）。
	run1, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq1.ID, e.ch1.ID, nil, "")
	if err != nil {
		t.Fatalf("开工: %v", err)
	}
	if run1.Status != domain.RunRunning {
		t.Fatalf("运行状态错误: %s", run1.Status)
	}
	sealed1 := e.completeAndSeal(run1.ID, 5.0)
	if sealed1.Status != domain.RunJudged || sealed1.Judgment != domain.JudgePass {
		t.Fatalf("判定错误: %+v", sealed1)
	}

	// 下一站放行。
	rel, _, err := e.svc.Lot.ReleaseNext(e.ctx, lot.ID, "正常放行", "")
	if err != nil {
		t.Fatalf("放行: %v", err)
	}
	if rel.FromSeq != 1 || rel.ToSeq != 2 || rel.Kind != domain.ReleaseNextStation {
		t.Fatalf("放行记录错误: %+v", rel)
	}

	// 第二站：开工（站点2 设备/腔体）。
	run2, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq2.ID, e.ch2.ID, nil, "")
	if err != nil {
		t.Fatalf("第二站开工: %v", err)
	}
	if run2.StationSeq != 2 || run2.StationID != e.st2.ID {
		t.Fatalf("第二站运行错误: %+v", run2)
	}
	if _, _, err := e.svc.Run.CompleteRun(e.ctx, run2.ID, ""); err != nil {
		t.Fatalf("第二站完工: %v", err)
	}
	lotWafers, _ := e.svc.Lot.ListWafers(e.ctx, lot.ID)
	if _, _, err := e.svc.Reading.SubmitReadings(e.ctx, run2.ID, nil, ""); err == nil {
		t.Fatal("空读数必须报错")
	}
	inputs2 := []service.ReadingInput{{WaferID: lotWafers[0].ID, Metric: "particle", Value: 3.0}}
	if _, _, err := e.svc.Reading.SubmitReadings(e.ctx, run2.ID, inputs2, ""); err != nil {
		t.Fatalf("第二站量测: %v", err)
	}
	sealed2, _, err := e.svc.Reading.Seal(e.ctx, run2.ID, "")
	if err != nil {
		t.Fatalf("第二站封存: %v", err)
	}
	if sealed2.Judgment != domain.JudgePass {
		t.Fatalf("第二站判定错误: %s", sealed2.Judgment)
	}

	// 末站放行 -> COMPLETED，然后关闭。
	if _, _, err := e.svc.Lot.ReleaseNext(e.ctx, lot.ID, "", ""); err != nil {
		t.Fatalf("末站放行: %v", err)
	}
	after, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
	if after.Status != domain.LotCompleted {
		t.Fatalf("末站放行后状态错误: %s", after.Status)
	}
	closed, _, err := e.svc.Lot.Close(e.ctx, lot.ID, "")
	if err != nil {
		t.Fatalf("关闭: %v", err)
	}
	if closed.Status != domain.LotClosed || closed.ClosedAt == nil {
		t.Fatalf("关闭状态错误: %+v", closed)
	}

	// 审计事件存在。
	audits, err := e.svc.Query.ListAudit(e.ctx, "lot", lot.ID)
	if err != nil || len(audits) == 0 {
		t.Fatalf("审计事件缺失: %v", err)
	}
	// 放行记录：两站放行 + 关闭。
	releases, _ := e.svc.Query.ListReleases(e.ctx, lot.ID)
	if len(releases) != 3 {
		t.Fatalf("放行记录数量错误: %d", len(releases))
	}
}
