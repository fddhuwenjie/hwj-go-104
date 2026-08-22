package httpx

import (
	"net/http"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/service"
)

// createRun 开工（设备与配方资格校核，幂等）。
func (h *handlers) createRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LotID          string   `json:"lot_id"`
		EquipmentID    string   `json:"equipment_id"`
		ChamberID      string   `json:"chamber_id"`
		WaferIDs       []string `json:"wafer_ids"`
		IdempotencyKey string   `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Run.CreateRun(r.Context(), req.LotID, req.EquipmentID, req.ChamberID, req.WaferIDs, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) getRun(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Run.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// completeRun 完工（资质区间复核与站点推进，幂等）。
func (h *handlers) completeRun(w http.ResponseWriter, r *http.Request) {
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
	out, replay, err := h.svc.Run.CompleteRun(r.Context(), id, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}

// submitReadings 提交量测读数（运行完工后，幂等）。
func (h *handlers) submitReadings(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Readings       []service.ReadingInput `json:"readings"`
		IdempotencyKey string                 `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Reading.SubmitReadings(r.Context(), id, req.Readings, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listReadings(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Reading.ListReadings(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// sealRun 量测封存与自动判定（失败自动生成暂扣，幂等）。
func (h *handlers) sealRun(w http.ResponseWriter, r *http.Request) {
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
	out, replay, err := h.svc.Reading.Seal(r.Context(), id, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}

// createHold 人工暂扣（幂等）。
func (h *handlers) createHold(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LotID          string `json:"lot_id"`
		Reason         string `json:"reason"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Hold.CreateHold(r.Context(), req.LotID, req.Reason, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) getHold(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Hold.GetHold(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// reviewHold 人工复判：RELEASE / REWORK / SCRAP（幂等）。
func (h *handlers) reviewHold(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Action         domain.ReviewAction `json:"action"`
		Note           string              `json:"note"`
		ReentrySeq     int                 `json:"reentry_seq"`
		IdempotencyKey string              `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, replay, err := h.svc.Hold.Review(r.Context(), id, req.Action, req.Note, req.ReentrySeq, idemKey(r, req.IdempotencyKey))
	if err != nil {
		writeError(w, err)
		return
	}
	writeReplayHeader(w, replay)
	writeData(w, http.StatusOK, out)
}
