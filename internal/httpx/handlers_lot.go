package httpx

import (
	"net/http"

	"gowork/wafer/internal/service"
)

// registerLot 批次登记（幂等键：Idempotency-Key 请求头或 idempotency_key 字段）。
func (h *handlers) registerLot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code            string                `json:"code"`
		ProductFamilyID string                `json:"product_family_id"`
		RouteID         string                `json:"route_id"`
		Wafers          []service.WaferInput  `json:"wafers"`
		IdempotencyKey  string                `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	key := idemKey(r, req.IdempotencyKey)
	out, replay, err := h.svc.Lot.RegisterLot(r.Context(), req.Code, req.ProductFamilyID, req.RouteID, req.Wafers, key)
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listLots(w http.ResponseWriter, r *http.Request) {
	page, err := queryPage(r)
	if err != nil {
		writeError(w, err)
		return
	}
	lots, next, err := h.svc.Lot.ListLots(r.Context(), page)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": lots, "next_cursor": next})
}

func (h *handlers) getLot(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Lot.GetLot(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) listLotWafers(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Lot.ListWafers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) listLotRuns(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Run.ListRunsByLot(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// splitLot 子批拆分（幂等）。
func (h *handlers) splitLot(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		ChildCode      string   `json:"child_code"`
		WaferIDs       []string `json:"wafer_ids"`
		IdempotencyKey string   `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Lot.SplitLot(r.Context(), id, req.ChildCode, req.WaferIDs, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusCreated, out)
}

// enterLot 进站排队（首次进站冻结路线，幂等）。
func (h *handlers) enterLot(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Lot.Enter(r.Context(), id, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}

// releaseNext 下一站放行（幂等）。
func (h *handlers) releaseNext(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Note           string `json:"note"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Lot.ReleaseNext(r.Context(), id, req.Note, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusCreated, out)
}

// closeLot 批次关闭（幂等）。
func (h *handlers) closeLot(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Lot.Close(r.Context(), id, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}

// scrapLot 批次报废（幂等）。
func (h *handlers) scrapLot(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Reason         string `json:"reason"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Lot.Scrap(r.Context(), id, req.Reason, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}

// restoreLot 恢复原路线（幂等）。
func (h *handlers) restoreLot(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Lot.Restore(r.Context(), id, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}

// waferGenealogy 晶圆谱系查询。
func (h *handlers) waferGenealogy(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	wafer, moves, err := h.svc.Lot.WaferGenealogy(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"wafer": wafer, "moves": moves})
}

// idemKey 提取幂等键：优先请求头。
func idemKey(r *http.Request, bodyKey string) string {
	if h := r.Header.Get("Idempotency-Key"); h != "" {
		return h
	}
	return bodyKey
}

// writeReplayHeader 标记幂等重放响应。
func writeReplayHeader(w http.ResponseWriter, replay bool) {
	if replay {
		w.Header().Set("Idempotent-Replay", "true")
	}
}
