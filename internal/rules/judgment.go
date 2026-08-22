package rules

import (
	"gowork/wafer/internal/domain"
)

// AutoJudge 自动判定：全部非迟到读数均不超过计划阈值则 PASS，否则 FAIL。
// 无读数时返回 NONE，调用方应已完成抽样覆盖校验。
func AutoJudge(plan domain.MetrologyPlan, readings []domain.Reading) domain.Judgment {
	seen := false
	for _, r := range readings {
		if r.Late {
			continue
		}
		seen = true
		if r.Value > plan.PassLimit {
			return domain.JudgeFail
		}
	}
	if !seen {
		return domain.JudgeNone
	}
	return domain.JudgePass
}
