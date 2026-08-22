package rules

import (
	"strings"

	"gowork/wafer/internal/domain"
)

// CheckCapability 校核设备与腔体能力是否匹配站点：
// 设备必须挂在该站点下，腔体能力标签须覆盖站点要求的能力标签。
func CheckCapability(eq domain.Equipment, ch domain.Chamber, st domain.Station) error {
	if eq.Status != domain.EquipActive {
		return domain.ErrCapability
	}
	if eq.StationID != st.ID {
		return domain.ErrCapability
	}
	if ch.EquipmentID == "" {
		return domain.ErrCapability
	}
	if ch.Status != "ACTIVE" {
		return domain.ErrCapability
	}
	if !capabilityCovers(ch.Capability, st.Capability) {
		return domain.ErrCapability
	}
	return nil
}

// capabilityCovers 能力标签覆盖判断：逗号分隔标签集合的超集判断。
func capabilityCovers(have, required string) bool {
	set := map[string]bool{}
	for _, t := range strings.Split(have, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			set[t] = true
		}
	}
	for _, t := range strings.Split(required, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !set[t] {
			return false
		}
	}
	return true
}

// CheckRecipeFamily 校核配方只能在适配的设备族上执行。
func CheckRecipeFamily(recipe domain.Recipe, eq domain.Equipment) error {
	if recipe.EquipmentFamily != eq.Family {
		return domain.ErrRecipeFamily
	}
	return nil
}
