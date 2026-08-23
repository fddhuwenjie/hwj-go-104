package httpx

import (
	"net/http"
	"time"
)

// expiredQualificationRuns 过期资质但尚未复判的运行（稳定分页）。
func (h *handlers) expiredQualificationRuns(w http.ResponseWriter, r *http.Request) {
	page, err := queryPage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	items, next, err := h.svc.Query.ExpiredQualificationRuns(r.Context(), page)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

// wipLots 当前在制批次（稳定分页）。
func (h *handlers) wipLots(w http.ResponseWriter, r *http.Request) {
	page, err := queryPage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	items, next, err := h.svc.Query.WipLots(r.Context(), page)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

// stationQueues 等待超时且设备能力可用的站点队列。
func (h *handlers) stationQueues(w http.ResponseWriter, r *http.Request) {
	secs, err := queryInt(r, "min_wait_seconds", 0)
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := h.svc.Query.StationQueues(r.Context(), time.Duration(secs)*time.Second)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

// reworkStats 重复返工聚合。
func (h *handlers) reworkStats(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Query.ReworkStats(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

// genealogyAudit 谱系审计。
func (h *handlers) genealogyAudit(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Query.GenealogyAudit(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

// listLotReleases 批次放行记录。
func (h *handlers) listLotReleases(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Query.ListReleases(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// listLotAudit 批次审计事件。
func (h *handlers) listLotAudit(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Query.ListAudit(r.Context(), "lot", id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}
