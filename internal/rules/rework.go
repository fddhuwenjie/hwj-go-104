package rules

import (
	"fmt"

	"gowork/wafer/internal/domain"
)

// CheckReworkReentry 返工重入校验：重入站点必须存在于冻结快照中，
// 且不得超过批次当前站点（只允许回退或原地重入）。
func CheckReworkReentry(snap *domain.FreezeSnapshot, reentrySeq, currentSeq int) error {
	if snap == nil {
		return domain.ErrNotFrozen
	}
	if snap.StationAt(reentrySeq) == nil {
		return fmt.Errorf("%w: 重入站点 %d 不存在于冻结快照", domain.ErrValidation, reentrySeq)
	}
	if reentrySeq > currentSeq {
		return fmt.Errorf("%w: 返工重入站点不能晚于当前站点", domain.ErrInvalidState)
	}
	return nil
}

// BuildReworkRevision 基于冻结快照构建返工新修订的站点序列：
// 从重入站点起保留原冻结站点顺序，生成新修订（旧修订与旧运行不受影响）。
func BuildReworkRevision(snap *domain.FreezeSnapshot, reentrySeq int) []domain.RouteStation {
	var out []domain.RouteStation
	seq := 1
	for _, st := range snap.Stations {
		if st.Seq < reentrySeq {
			continue
		}
		out = append(out, domain.RouteStation{
			Seq:             seq,
			StationID:       st.StationID,
			RecipeID:        st.RecipeID,
			MetrologyPlanID: st.MetrologyPlanID,
		})
		seq++
	}
	return out
}
