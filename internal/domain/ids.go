package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewID 生成带前缀的全局唯一标识，格式：<prefix>_<毫秒时间戳><随机>。
// 前缀便于在审计日志与谱系查询中快速识别实体类型。
func NewID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("domain.NewID: %v", err))
	}
	return fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

// 常用实体 ID 前缀。
const (
	IDPrefixProductFamily = "pf"
	IDPrefixStation       = "st"
	IDPrefixRoute         = "rt"
	IDPrefixRouteRev      = "rr"
	IDPrefixRecipe        = "rc"
	IDPrefixRecipeVer     = "rv"
	IDPrefixEquipment     = "eq"
	IDPrefixChamber       = "ch"
	IDPrefixQualification = "qu"
	IDPrefixPlan          = "mp"
	IDPrefixLot           = "lot"
	IDPrefixWafer         = "wf"
	IDPrefixRun           = "run"
	IDPrefixReading       = "rd"
	IDPrefixHold          = "hd"
	IDPrefixRelease       = "rl"
	IDPrefixAudit         = "au"
	IDPrefixJob           = "jb"
)
