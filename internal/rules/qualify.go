package rules

import (
	"time"

	"gowork/wafer/internal/domain"
)

// CheckStartQualification 开工校核：设备在目标站点上须存在活跃资质，
// 且资质窗口已生效（valid_from <= start，valid_to > start）。
// 完整运行区间在完工时由 CoverageAtCompletion 复核。
func CheckStartQualification(quals []domain.Qualification, eq domain.Equipment, chamberID, stationID string, start time.Time) error {
	if eq.StationID != stationID {
		return domain.ErrCapability
	}
	for _, q := range quals {
		if q.EquipmentID != eq.ID || q.StationID != stationID {
			continue
		}
		if q.ChamberID != "" && q.ChamberID != chamberID {
			continue
		}
		if q.Status == domain.QualActive && !q.ValidFrom.After(start) && q.ValidTo.After(start) {
			return nil
		}
	}
	return domain.ErrQualification
}

// CoverageAtCompletion 完工复核：判断是否存在资质窗口完整覆盖 [start, end]。
// 返回 false 不阻断完工，但会在运行上标记 qual_covered=false，
// 供“过期资质但尚未复判的运行”查询与暂扣升级使用。
func CoverageAtCompletion(quals []domain.Qualification, eq domain.Equipment, chamberID, stationID string, start, end time.Time) bool {
	for _, q := range quals {
		if q.EquipmentID != eq.ID || q.StationID != stationID {
			continue
		}
		if q.ChamberID != "" && q.ChamberID != chamberID {
			continue
		}
		if q.Covers(start, end) {
			return true
		}
	}
	return false
}

// FindExpired 资质到期扫描：返回在 now 之前已失效但仍标记 ACTIVE 的资质。
func FindExpired(quals []domain.Qualification, now time.Time) []domain.Qualification {
	var out []domain.Qualification
	for _, q := range quals {
		if q.Status == domain.QualActive && !q.ValidTo.After(now) {
			out = append(out, q)
		}
	}
	return out
}
