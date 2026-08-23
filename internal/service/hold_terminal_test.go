package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
)

// TestScrappedLotCannotBeReheld 报废终态不可逆：
// 批次经复判报废并记录关闭时间后，质检人员再次发起人工暂扣必须被拒绝，
// 终态 SCRAPPED 与 ClosedAt 不得被覆盖。
//
// 复现路径：failAndHold -> Review(Scrap) -> CreateHold（二次暂扣）。
func TestScrappedLotCannotBeReheld(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll()

	lot := e.registerLot("LOT-REHOLD")
	_, hold := failAndHold(t, e, lot.ID) // 批次进入 ON_HOLD，暂扣 OPEN

	// 复判报废：批次进入终态 SCRAPPED，记录关闭时间。
	res, _, err := e.svc.Hold.Review(e.ctx, hold.ID, domain.ReviewScrap, "无法返工", 0, "")
	if err != nil {
		t.Fatalf("复判报废: %v", err)
	}
	if res.Lot.Status != domain.LotScrapped || res.Lot.ClosedAt == nil {
		t.Fatalf("报废状态错误: %+v", res.Lot)
	}
	closedAt := *res.Lot.ClosedAt

	// 质检人员再次发起人工暂扣：终态不可逆，必须拒绝。
	_, _, err = e.svc.Hold.CreateHold(e.ctx, lot.ID, "二次抽检异常", "")
	if err == nil {
		t.Fatal("报废批次再次暂扣应被拒绝，终态被穿透")
	}
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("应返回状态非法错误，实际: %v", err)
	}

	// 批次仍为终态 SCRAPPED，关闭时间未被覆盖。
	after, _ := e.svc.Lot.GetLot(e.ctx, lot.ID)
	if after.Status != domain.LotScrapped {
		t.Fatalf("报废终态被改写为 %s", after.Status)
	}
	if after.ClosedAt == nil || !after.ClosedAt.Equal(closedAt) {
		t.Fatalf("关闭时间被覆盖: 期望 %v，实际 %+v", closedAt, after.ClosedAt)
	}
}
