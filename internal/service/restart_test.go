package service_test

import (
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// TestRestartResumeWipState 关闭后重启并使用同一数据库文件：
// 在制批次状态、冻结快照、运行状态完整恢复，可继续后续站点操作。
func TestRestartResumeWipState(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-RESTART")
	run := e.enterAndRun(lot.ID)
	db := e.db
	e.store.Close()

	// 同一数据库文件重新打开（模拟进程重启）。
	e2 := openEnv(t, db)
	restored, err := e2.svc.Lot.GetLot(e2.ctx, lot.ID)
	if err != nil {
		t.Fatalf("重启后读取批次: %v", err)
	}
	if restored.Status != domain.LotRunning || restored.CurrentSeq != 1 {
		t.Fatalf("重启后批次状态错误: %+v", restored)
	}
	if !restored.IsFrozen() || restored.FrozenRevisionID != e.rev.ID {
		t.Fatal("重启后冻结快照丢失")
	}

	// 运行仍可完工、量测、封存、放行。
	completed, _, err := e2.svc.Run.CompleteRun(e2.ctx, run.ID, "")
	if err != nil {
		t.Fatalf("重启后完工: %v", err)
	}
	if completed.Status != domain.RunCompleted {
		t.Fatalf("重启后完工状态错误: %s", completed.Status)
	}
	wafers, _ := e2.svc.Lot.ListWafers(e2.ctx, lot.ID)
	var sub []service.ReadingInput
	for _, w := range wafers {
		if w.Slot <= 2 {
			sub = append(sub, service.ReadingInput{WaferID: w.ID, Metric: "cd", Value: 5.0})
		}
	}
	if _, _, err := e2.svc.Reading.SubmitReadings(e2.ctx, run.ID, sub, ""); err != nil {
		t.Fatalf("重启后量测: %v", err)
	}
	sealed, _, err := e2.svc.Reading.Seal(e2.ctx, run.ID, "")
	if err != nil {
		t.Fatalf("重启后封存: %v", err)
	}
	if sealed.Judgment != domain.JudgePass {
		t.Fatalf("重启后判定错误: %s", sealed.Judgment)
	}
}
