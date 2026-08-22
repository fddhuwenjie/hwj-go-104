package httpx

import (
	"context"
	"net/http"
	"strings"

	"gowork/wafer/internal/service"
)

// routeEntry 路由表项：pattern 形如 "POST /api/v1/lots/{id}/split"。
type routeEntry struct {
	method   string
	segments []string
	handler  http.HandlerFunc
}

// router 轻量路由器：支持 {param} 路径参数，兼容 go1.21 语言版本。
type router struct {
	routes []routeEntry
}

// pathParamsKey 路径参数上下文键。
type pathParamsKey struct{}

// withParams 把路径参数写入上下文。
func withParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, pathParamsKey{}, params)
}

// pathParam 读取路径参数。
func pathParam(r *http.Request, key string) string {
	if m, ok := r.Context().Value(pathParamsKey{}).(map[string]string); ok {
		return m[key]
	}
	return ""
}

// handle 注册路由。
func (rt *router) handle(pattern string, h http.HandlerFunc) {
	parts := strings.SplitN(pattern, " ", 2)
	segs := strings.Split(strings.Trim(parts[1], "/"), "/")
	rt.routes = append(rt.routes, routeEntry{method: parts[0], segments: segs, handler: h})
}

// ServeHTTP 匹配路由并分发。
func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	var segs []string
	if path != "" {
		segs = strings.Split(path, "/")
	}
	methodNotAllowed := false
	for _, rtEntry := range rt.routes {
		params, ok := matchSegments(rtEntry.segments, segs)
		if !ok {
			continue
		}
		if rtEntry.method != r.Method {
			methodNotAllowed = true
			continue
		}
		rtEntry.handler(w, r.WithContext(withParams(r.Context(), params)))
		return
	}
	if methodNotAllowed {
		writeJSON(w, http.StatusMethodNotAllowed, errorBody{Error: "方法不允许", Code: "METHOD_NOT_ALLOWED"})
		return
	}
	writeJSON(w, http.StatusNotFound, errorBody{Error: "资源不存在", Code: "NOT_FOUND"})
}

// matchSegments 段匹配：{x} 匹配任意单段并提取参数。
func matchSegments(pattern, actual []string) (map[string]string, bool) {
	if len(pattern) != len(actual) {
		return nil, false
	}
	params := map[string]string{}
	for i, p := range pattern {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			params[p[1:len(p)-1]] = actual[i]
			continue
		}
		if p != actual[i] {
			return nil, false
		}
	}
	return params, true
}

// NewRouter 注册全部 HTTP 路由。
func NewRouter(svc *service.Services, health http.HandlerFunc) http.Handler {
	rt := &router{}
	h := &handlers{svc: svc}

	rt.handle("GET /healthz", health)

	// 主数据
	rt.handle("POST /api/v1/product-families", h.createProductFamily)
	rt.handle("GET /api/v1/product-families", h.listProductFamilies)
	rt.handle("GET /api/v1/product-families/{id}", h.getProductFamily)
	rt.handle("POST /api/v1/stations", h.createStation)
	rt.handle("GET /api/v1/stations", h.listStations)
	rt.handle("GET /api/v1/stations/{id}", h.getStation)

	// 工艺路线与修订
	rt.handle("POST /api/v1/routes", h.createRoute)
	rt.handle("GET /api/v1/routes", h.listRoutes)
	rt.handle("GET /api/v1/routes/{id}", h.getRoute)
	rt.handle("POST /api/v1/routes/{id}/revisions", h.createRevision)
	rt.handle("GET /api/v1/routes/{id}/revisions", h.listRevisions)
	rt.handle("GET /api/v1/revisions/{id}", h.getRevision)
	rt.handle("GET /api/v1/revisions/{id}/stations", h.listRevisionStations)
	rt.handle("POST /api/v1/revisions/{id}/activate", h.activateRevision)

	// 配方与版本
	rt.handle("POST /api/v1/recipes", h.createRecipe)
	rt.handle("GET /api/v1/recipes", h.listRecipes)
	rt.handle("GET /api/v1/recipes/{id}", h.getRecipe)
	rt.handle("POST /api/v1/recipes/{id}/versions", h.createRecipeVersion)
	rt.handle("GET /api/v1/recipes/{id}/versions", h.listRecipeVersions)
	rt.handle("GET /api/v1/recipe-versions/{id}", h.getRecipeVersion)
	rt.handle("POST /api/v1/recipe-versions/{id}/activate", h.activateRecipeVersion)

	// 设备、腔体与资质
	rt.handle("POST /api/v1/equipment", h.createEquipment)
	rt.handle("GET /api/v1/equipment", h.listEquipment)
	rt.handle("GET /api/v1/equipment/{id}", h.getEquipment)
	rt.handle("POST /api/v1/equipment/{id}/status", h.setEquipmentStatus)
	rt.handle("POST /api/v1/equipment/{id}/chambers", h.createChamber)
	rt.handle("GET /api/v1/equipment/{id}/chambers", h.listChambers)
	rt.handle("POST /api/v1/qualifications", h.createQualification)
	rt.handle("GET /api/v1/qualifications", h.listQualifications)

	// 量测计划
	rt.handle("POST /api/v1/metrology-plans", h.createPlan)
	rt.handle("GET /api/v1/metrology-plans", h.listPlans)
	rt.handle("GET /api/v1/metrology-plans/{id}", h.getPlan)
	rt.handle("POST /api/v1/metrology-plans/{id}/activate", h.activatePlan)

	// 批次与晶圆谱系
	rt.handle("POST /api/v1/lots", h.registerLot)
	rt.handle("GET /api/v1/lots", h.listLots)
	rt.handle("GET /api/v1/lots/{id}", h.getLot)
	rt.handle("GET /api/v1/lots/{id}/wafers", h.listLotWafers)
	rt.handle("GET /api/v1/lots/{id}/runs", h.listLotRuns)
	rt.handle("GET /api/v1/lots/{id}/releases", h.listLotReleases)
	rt.handle("GET /api/v1/lots/{id}/audit", h.listLotAudit)
	rt.handle("POST /api/v1/lots/{id}/split", h.splitLot)
	rt.handle("POST /api/v1/lots/{id}/enter", h.enterLot)
	rt.handle("POST /api/v1/lots/{id}/release-next", h.releaseNext)
	rt.handle("POST /api/v1/lots/{id}/close", h.closeLot)
	rt.handle("POST /api/v1/lots/{id}/scrap", h.scrapLot)
	rt.handle("POST /api/v1/lots/{id}/restore", h.restoreLot)
	rt.handle("GET /api/v1/wafers/{id}/genealogy", h.waferGenealogy)

	// 制程运行与量测
	rt.handle("POST /api/v1/runs", h.createRun)
	rt.handle("GET /api/v1/runs/{id}", h.getRun)
	rt.handle("POST /api/v1/runs/{id}/complete", h.completeRun)
	rt.handle("POST /api/v1/runs/{id}/readings", h.submitReadings)
	rt.handle("GET /api/v1/runs/{id}/readings", h.listReadings)
	rt.handle("POST /api/v1/runs/{id}/seal", h.sealRun)

	// 暂扣与复判
	rt.handle("POST /api/v1/holds", h.createHold)
	rt.handle("GET /api/v1/holds/{id}", h.getHold)
	rt.handle("POST /api/v1/holds/{id}/review", h.reviewHold)

	// 分析查询
	rt.handle("GET /api/v1/queries/expired-qualification-runs", h.expiredQualificationRuns)
	rt.handle("GET /api/v1/queries/wip-lots", h.wipLots)
	rt.handle("GET /api/v1/queries/station-queues", h.stationQueues)
	rt.handle("GET /api/v1/queries/rework-stats", h.reworkStats)
	rt.handle("GET /api/v1/queries/genealogy-audit", h.genealogyAudit)

	return recoverMiddleware(loggingMiddleware(rt))
}
