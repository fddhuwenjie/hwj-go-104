package rules

import (
	"gowork/wafer/internal/domain"
)

// CheckSamplingCoverage 抽样覆盖校验：
// 已提交读数必须覆盖计划指定的晶圆位置（槽位）且数量不少于最小样本数。
// lotSlots 为批次当前晶圆槽位集合，计划位置以批次实际存在的槽位为准。
func CheckSamplingCoverage(plan domain.MetrologyPlan, readings []domain.Reading, lotSlots map[int]bool) error {
	covered := map[int]bool{}
	count := 0
	for _, r := range readings {
		if r.Late {
			continue // 迟到量测不计入覆盖
		}
		if !covered[r.Slot] {
			covered[r.Slot] = true
			count++
		}
	}
	for _, pos := range plan.SamplePositions {
		if !lotSlots[pos] {
			continue // 批次不存在该槽位（如子批拆分后），不要求覆盖
		}
		if !covered[pos] {
			return domain.ErrSampling
		}
	}
	if count < plan.MinSamples {
		return domain.ErrSampling
	}
	return nil
}
