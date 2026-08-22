package rules

import (
	"gowork/wafer/internal/domain"
)

// CheckHoldBlocking 暂扣阻断校验：给定批次及其所有后代批次上的暂扣，
// 任何未关闭暂扣都阻断后续站点操作（进站、开工、完工推进）。
func CheckHoldBlocking(holds []domain.Hold) error {
	for _, h := range holds {
		if h.IsOpen() {
			return domain.ErrHoldBlocking
		}
	}
	return nil
}

// CheckStationOrder 站点顺序校验：防止非法越站。
// next 必须等于批次当前站点顺序号 + 1。
func CheckStationOrder(currentSeq, next int, snap *domain.FreezeSnapshot) error {
	if next != currentSeq+1 {
		return domain.ErrStationSkip
	}
	if snap.StationAt(next) == nil {
		return domain.ErrStationSkip
	}
	return nil
}

// CheckWaferAvailable 校验同一晶圆不得同时处于两个运行。
func CheckWaferAvailable(busy map[string]bool, waferIDs []string) error {
	for _, id := range waferIDs {
		if busy[id] {
			return domain.ErrWaferBusy
		}
	}
	return nil
}
