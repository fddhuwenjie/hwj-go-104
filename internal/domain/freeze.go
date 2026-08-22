package domain

import "encoding/json"

// FreezeSnapshot 批次首次进站时的冻结快照：
// 路线修订、站点顺序、配方快照与量测计划全部固化，后续运行只能引用快照。
type FreezeSnapshot struct {
	RouteID         string          `json:"route_id"`
	RouteRevisionID string          `json:"route_revision_id"`
	Revision        int             `json:"revision"`
	Stations        []FreezeStation `json:"stations"`
}

// FreezeStation 冻结快照中的站点条目。
type FreezeStation struct {
	Seq             int    `json:"seq"`
	StationID       string `json:"station_id"`
	StationCode     string `json:"station_code"`
	Capability      string `json:"capability"`
	RecipeID        string `json:"recipe_id"`
	RecipeVersionID string `json:"recipe_version_id"`
	RecipeSnapshot  string `json:"recipe_snapshot"`
	MetrologyPlanID string `json:"metrology_plan_id"`
	PlanSnapshot    string `json:"plan_snapshot"` // MetrologyPlan 的 JSON 快照
}

// Encode 序列化冻结快照。
func (s *FreezeSnapshot) Encode() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeFreezeSnapshot 反序列化冻结快照。
func DecodeFreezeSnapshot(raw string) (*FreezeSnapshot, error) {
	var s FreezeSnapshot
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// StationAt 返回指定顺序号的冻结站点。
func (s *FreezeSnapshot) StationAt(seq int) *FreezeStation {
	for i := range s.Stations {
		if s.Stations[i].Seq == seq {
			return &s.Stations[i]
		}
	}
	return nil
}

// LastSeq 返回最后一个站点的顺序号。
func (s *FreezeSnapshot) LastSeq() int {
	last := 0
	for _, st := range s.Stations {
		if st.Seq > last {
			last = st.Seq
		}
	}
	return last
}
