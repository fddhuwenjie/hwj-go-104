package httpx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gowork/wafer/internal/clock"
	"gowork/wafer/internal/httpx"
	"gowork/wafer/internal/service"
	"gowork/wafer/internal/sqlite"
)

// httpEnv HTTP 端到端测试环境。
type httpEnv struct {
	t   *testing.T
	srv *httptest.Server
	clk *clock.Manual
}

var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func newHTTPEnv(t *testing.T) *httpEnv {
	t.Helper()
	db := filepath.Join(t.TempDir(), "http.db")
	store, err := sqlite.Open(db)
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	clk := clock.NewManual(baseTime)
	svc := service.NewServices(service.Deps{Store: store, Clock: clk})
	health := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	srv := httptest.NewServer(httpx.NewRouter(svc, health))
	t.Cleanup(srv.Close)
	return &httpEnv{t: t, srv: srv, clk: clk}
}

// do 发送请求并解码统一响应。
func (e *httpEnv) do(method, path string, body any, headers map[string]string) (int, map[string]any, http.Header) {
	e.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, reader)
	if err != nil {
		e.t.Fatalf("构造请求: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("请求: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out, resp.Header
}

// data 提取 data 字段。
func data(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	d, ok := m["data"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 data: %v", m)
	}
	return d
}

// TestHTTPEndToEnd HTTP 端到端：主数据 -> 启用 -> 登记 -> 冻结进站 ->
// 开工 -> 完工 -> 量测 -> 封存 -> 放行 -> 关闭；覆盖统一错误与幂等头。
func TestHTTPEndToEnd(t *testing.T) {
	e := newHTTPEnv(t)

	// 健康检查。
	code, body, _ := e.do("GET", "/healthz", nil, nil)
	if code != 200 || body["status"] != "ok" {
		t.Fatalf("healthz: %d %v", code, body)
	}

	// 统一 JSON 错误：非法参数。
	code, body, _ = e.do("POST", "/api/v1/product-families", map[string]any{"code": ""}, nil)
	if code != 400 || body["code"] != "VALIDATION" {
		t.Fatalf("统一错误: %d %v", code, body)
	}
	// 404。
	code, body, _ = e.do("GET", "/api/v1/lots/none", nil, nil)
	if code != 404 || body["code"] != "NOT_FOUND" {
		t.Fatalf("404: %d %v", code, body)
	}

	// 主数据。
	code, body, _ = e.do("POST", "/api/v1/product-families", map[string]any{"code": "PF", "name": "逻辑"}, nil)
	pfID := data(t, body)["id"].(string)
	if code != 201 {
		t.Fatalf("产品族: %d %v", code, body)
	}
	code, body, _ = e.do("POST", "/api/v1/stations", map[string]any{"code": "ETCH", "name": "刻蚀", "capability": "etch"}, nil)
	stID := data(t, body)["id"].(string)
	code, body, _ = e.do("POST", "/api/v1/recipes", map[string]any{"code": "RCP", "name": "配方", "equipment_family": "FA"}, nil)
	rcID := data(t, body)["id"].(string)
	code, body, _ = e.do("POST", "/api/v1/metrology-plans", map[string]any{
		"code": "MP", "name": "计划", "metric": "cd", "sample_positions": []int{1, 2}, "min_samples": 2, "pass_limit": 10,
	}, nil)
	planID := data(t, body)["id"].(string)
	planVer := int(data(t, body)["row_version"].(float64))
	code, _, _ = e.do("POST", "/api/v1/metrology-plans/"+planID+"/activate", map[string]any{"version": planVer}, nil)
	if code != 200 {
		t.Fatalf("启用计划: %d", code)
	}
	code, body, _ = e.do("POST", "/api/v1/recipes/"+rcID+"/versions", map[string]any{"params": map[string]any{"temp": 100}}, nil)
	rvID := data(t, body)["id"].(string)
	rvVer := int(data(t, body)["row_version"].(float64))
	code, _, _ = e.do("POST", "/api/v1/recipe-versions/"+rvID+"/activate", map[string]any{"version": rvVer}, nil)
	if code != 200 {
		t.Fatalf("启用配方: %d", code)
	}
	code, body, _ = e.do("POST", "/api/v1/routes", map[string]any{"product_family_id": pfID, "code": "RT", "name": "路线"}, nil)
	routeID := data(t, body)["id"].(string)
	code, body, _ = e.do("POST", "/api/v1/routes/"+routeID+"/revisions", map[string]any{
		"stations": []map[string]any{{"seq": 1, "station_id": stID, "recipe_id": rcID, "metrology_plan_id": planID}},
	}, nil)
	revID := data(t, body)["id"].(string)
	revVer := int(data(t, body)["version"].(float64))
	code, _, _ = e.do("POST", "/api/v1/revisions/"+revID+"/activate", map[string]any{"version": revVer}, nil)
	if code != 200 {
		t.Fatalf("启用修订: %d", code)
	}
	code, body, _ = e.do("POST", "/api/v1/equipment", map[string]any{"code": "EQ1", "name": "设备", "family": "FA", "station_id": stID}, nil)
	eqID := data(t, body)["id"].(string)
	code, body, _ = e.do("POST", "/api/v1/equipment/"+eqID+"/chambers", map[string]any{"code": "CHA", "capability": "etch"}, nil)
	chID := data(t, body)["id"].(string)
	code, _, _ = e.do("POST", "/api/v1/qualifications", map[string]any{
		"equipment_id": eqID, "station_id": stID,
		"valid_from": baseTime.Add(-time.Hour).Format(time.RFC3339),
		"valid_to":   baseTime.Add(24 * time.Hour).Format(time.RFC3339),
	}, nil)
	if code != 201 {
		t.Fatalf("资质: %d", code)
	}

	// 批次登记（幂等键重放）。
	lotReq := map[string]any{
		"code": "LOT-HTTP", "product_family_id": pfID, "route_id": routeID,
		"wafers": []map[string]any{{"code": "W1", "slot": 1}, {"code": "W2", "slot": 2}},
	}
	code, body, hdr := e.do("POST", "/api/v1/lots", lotReq, map[string]string{"Idempotency-Key": "k1"})
	if code != 201 {
		t.Fatalf("登记: %d %v", code, body)
	}
	lotID := data(t, body)["id"].(string)
	code, body2, hdr2 := e.do("POST", "/api/v1/lots", lotReq, map[string]string{"Idempotency-Key": "k1"})
	if code != 201 || hdr2.Get("Idempotent-Replay") != "true" || data(t, body2)["id"] != lotID {
		t.Fatalf("幂等重放: %d %v %v", code, hdr2, body2)
	}
	_ = hdr

	// 进站冻结 -> 开工 -> 完工 -> 量测 -> 封存 -> 放行 -> 关闭。
	code, body, _ = e.do("POST", "/api/v1/lots/"+lotID+"/enter", nil, nil)
	if code != 200 || data(t, body)["frozen_revision_id"] == "" {
		t.Fatalf("进站冻结: %d %v", code, body)
	}
	code, body, _ = e.do("POST", "/api/v1/runs", map[string]any{"lot_id": lotID, "equipment_id": eqID, "chamber_id": chID}, nil)
	if code != 201 {
		t.Fatalf("开工: %d %v", code, body)
	}
	runID := data(t, body)["id"].(string)
	code, _, _ = e.do("POST", "/api/v1/runs/"+runID+"/complete", nil, nil)
	if code != 200 {
		t.Fatalf("完工: %d", code)
	}
	code, body, _ = e.do("GET", "/api/v1/lots/"+lotID+"/wafers", nil, nil)
	wafers := body["data"].([]any)
	var readings []map[string]any
	for _, w := range wafers {
		readings = append(readings, map[string]any{"wafer_id": w.(map[string]any)["id"], "metric": "cd", "value": 5})
	}
	code, _, _ = e.do("POST", "/api/v1/runs/"+runID+"/readings", map[string]any{"readings": readings}, nil)
	if code != 201 {
		t.Fatalf("量测: %d", code)
	}
	code, body, _ = e.do("POST", "/api/v1/runs/"+runID+"/seal", nil, nil)
	if code != 200 || data(t, body)["judgment"] != "PASS" {
		t.Fatalf("封存: %d %v", code, body)
	}
	code, _, _ = e.do("POST", "/api/v1/lots/"+lotID+"/release-next", nil, nil)
	if code != 201 {
		t.Fatalf("放行: %d", code)
	}
	code, body, _ = e.do("POST", "/api/v1/lots/"+lotID+"/close", nil, nil)
	if code != 200 || data(t, body)["status"] != "CLOSED" {
		t.Fatalf("关闭: %d %v", code, body)
	}

	// 在制查询为空（已关闭）。
	code, body, _ = e.do("GET", "/api/v1/queries/wip-lots?limit=10", nil, nil)
	if code != 200 {
		t.Fatalf("在制查询: %d", code)
	}
	items := data(t, body)["items"]
	if items != nil && len(items.([]any)) != 0 {
		t.Fatalf("关闭批次不应出现在在制查询: %v", items)
	}
}
