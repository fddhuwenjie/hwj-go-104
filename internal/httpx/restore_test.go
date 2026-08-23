package httpx_test

import (
	"testing"
	"time"
)

// TestHTTPRestoreBlockedByOpenHold HTTP 端到端：根批次存在开放暂扣（未复判）时
// POST /api/v1/lots/{id}/restore 必须返回 HOLD_BLOCKING(422)。
// 复现修复前的穿透：Restore 状态回退 QUEUED 且暂扣仍 OPEN。
func TestHTTPRestoreBlockedByOpenHold(t *testing.T) {
	e := newHTTPEnv(t)

	// 主数据：单站点单配方单计划单设备。
	pfID := httpCreateProductFamily(t, e)
	stID := httpCreateStation(t, e, "ETCH", "etch")
	rcID := httpCreateRecipe(t, e)
	planID, planVer := httpCreatePlan(t, e, 10.0)
	mustPost(t, e, "/api/v1/metrology-plans/"+planID+"/activate", map[string]any{"version": planVer})
	rvID, rvVer := httpCreateRecipeVersion(t, e, rcID)
	mustPost(t, e, "/api/v1/recipe-versions/"+rvID+"/activate", map[string]any{"version": rvVer})
	routeID := httpCreateRoute(t, e, pfID)
	revID, revVer := httpCreateRevision(t, e, routeID, stID, rcID, planID)
	mustPost(t, e, "/api/v1/revisions/"+revID+"/activate", map[string]any{"version": revVer})
	eqID, chID := httpCreateEquipment(t, e, stID, "ETCH-1", "CHA", "etch")
	httpCreateQualification(t, e, eqID, stID)

	// 批次登记。
	lotID := httpRegisterLot(t, e, "LOT-HTTP-R", pfID, routeID)

	// 人工暂扣（保持 OPEN，未经量测）。
	httpHold(t, e, lotID, "抽检异常")

	// 根批次开放暂扣未复判即 Restore：必须 HOLD_BLOCKING。
	code, body, _ := e.do("POST", "/api/v1/lots/"+lotID+"/restore", nil, nil)
	if code != 422 || body["code"] != "HOLD_BLOCKING" {
		t.Fatalf("开放暂扣 Restore 应返回 HOLD_BLOCKING(422)，得到 code=%d body=%v", code, body)
	}

	// 确认状态未回退：批次仍为 ON_HOLD（GET 批次）。
	code, body, _ = e.do("GET", "/api/v1/lots/"+lotID, nil, nil)
	if data(t, body)["status"] != "ON_HOLD" {
		t.Fatalf("状态不应回退，仍应 ON_HOLD: %v", data(t, body)["status"])
	}
	// 确认未产生 RESTORE 放行记录：放行记录列表为空。
	code, body, _ = e.do("GET", "/api/v1/lots/"+lotID+"/releases", nil, nil)
	// ListReleases 返回 []domain.Release（无记录时为 null/空），data 为数组或 nil。
	if rels, ok := body["data"].([]any); ok && len(rels) != 0 {
		t.Fatalf("阻断后不应产生放行记录: %v", rels)
	}
	_ = chID
}

// ---- HTTP 测试辅助（端到端建挡样板）----

func mustPost(t *testing.T, e *httpEnv, path string, body any) {
	t.Helper()
	code, b, _ := e.do("POST", path, body, nil)
	if code >= 300 {
		t.Fatalf("%s: %d %v", path, code, b)
	}
}

func httpCreateProductFamily(t *testing.T, e *httpEnv) string {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/product-families", map[string]any{"code": "PF", "name": "逻辑"}, nil)
	if code != 201 {
		t.Fatalf("产品族: %d %v", code, body)
	}
	return data(t, body)["id"].(string)
}

func httpCreateStation(t *testing.T, e *httpEnv, c, cap string) string {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/stations", map[string]any{"code": c, "name": c, "capability": cap}, nil)
	if code != 201 {
		t.Fatalf("站点: %d %v", code, body)
	}
	return data(t, body)["id"].(string)
}

func httpCreateRecipe(t *testing.T, e *httpEnv) string {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/recipes", map[string]any{"code": "RCP", "name": "配方", "equipment_family": "FA"}, nil)
	if code != 201 {
		t.Fatalf("配方: %d %v", code, body)
	}
	return data(t, body)["id"].(string)
}

func httpCreatePlan(t *testing.T, e *httpEnv, limit float64) (string, int) {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/metrology-plans", map[string]any{
		"code": "MP", "name": "计划", "metric": "cd",
		"sample_positions": []int{1, 2}, "min_samples": 2, "pass_limit": limit,
	}, nil)
	if code != 201 {
		t.Fatalf("计划: %d %v", code, body)
	}
	d := data(t, body)
	return d["id"].(string), int(d["row_version"].(float64))
}

func httpCreateRecipeVersion(t *testing.T, e *httpEnv, rcID string) (string, int) {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/recipes/"+rcID+"/versions", map[string]any{"params": map[string]any{"temp": 100}}, nil)
	if code != 201 {
		t.Fatalf("配方版本: %d %v", code, body)
	}
	d := data(t, body)
	return d["id"].(string), int(d["row_version"].(float64))
}

func httpCreateRoute(t *testing.T, e *httpEnv, pfID string) string {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/routes", map[string]any{"product_family_id": pfID, "code": "RT", "name": "路线"}, nil)
	if code != 201 {
		t.Fatalf("路线: %d %v", code, body)
	}
	return data(t, body)["id"].(string)
}

func httpCreateRevision(t *testing.T, e *httpEnv, routeID, stID, rcID, planID string) (string, int) {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/routes/"+routeID+"/revisions", map[string]any{
		"stations": []map[string]any{{"seq": 1, "station_id": stID, "recipe_id": rcID, "metrology_plan_id": planID}},
	}, nil)
	if code != 201 {
		t.Fatalf("修订: %d %v", code, body)
	}
	d := data(t, body)
	return d["id"].(string), int(d["version"].(float64))
}

func httpCreateEquipment(t *testing.T, e *httpEnv, stID, eqCode, chCode, cap string) (string, string) {
	t.Helper()
	code, body, _ := e.do("POST", "/api/v1/equipment", map[string]any{"code": eqCode, "name": "设备", "family": "FA", "station_id": stID}, nil)
	if code != 201 {
		t.Fatalf("设备: %d %v", code, body)
	}
	eqID := data(t, body)["id"].(string)
	code, body, _ = e.do("POST", "/api/v1/equipment/"+eqID+"/chambers", map[string]any{"code": chCode, "capability": cap}, nil)
	if code != 201 {
		t.Fatalf("腔体: %d %v", code, body)
	}
	return eqID, data(t, body)["id"].(string)
}

func httpCreateQualification(t *testing.T, e *httpEnv, eqID, stID string) {
	t.Helper()
	c, _, _ := e.do("POST", "/api/v1/qualifications", map[string]any{
		"equipment_id": eqID, "station_id": stID,
		"valid_from":   baseTime.Add(-time.Hour).Format(time.RFC3339),
		"valid_to":     baseTime.Add(24 * time.Hour).Format(time.RFC3339),
	}, nil)
	if c != 201 {
		t.Fatalf("资质: %d", c)
	}
}

func httpRegisterLot(t *testing.T, e *httpEnv, code, pfID, routeID string) string {
	t.Helper()
	c, body, _ := e.do("POST", "/api/v1/lots", map[string]any{
		"code": code, "product_family_id": pfID, "route_id": routeID,
		"wafers": []map[string]any{{"code": code + "-W1", "slot": 1}, {"code": code + "-W2", "slot": 2}},
	}, nil)
	if c != 201 {
		t.Fatalf("登记批次: %d %v", c, body)
	}
	return data(t, body)["id"].(string)
}

func httpHold(t *testing.T, e *httpEnv, lotID, reason string) {
	t.Helper()
	c, body, _ := e.do("POST", "/api/v1/holds", map[string]any{"lot_id": lotID, "reason": reason}, nil)
	if c != 201 {
		t.Fatalf("暂扣: %d %v", c, body)
	}
}
