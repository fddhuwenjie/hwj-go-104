package rules

import "encoding/json"

// jsonMarshal 统一 JSON 序列化入口，便于规则层测试替换。
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
