package httpx

import (
	"net/http"
	"time"

	"gowork/wafer/internal/domain"
)

// createEquipment 建档设备。
func (h *handlers) createEquipment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		Family    string `json:"family"`
		StationID string `json:"station_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.CreateEquipment(r.Context(), req.Code, req.Name, req.Family, req.StationID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listEquipment(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListEquipment(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getEquipment(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.GetEquipment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// setEquipmentStatus 乐观锁更新设备状态。
func (h *handlers) setEquipmentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Status  domain.EquipmentStatus `json:"status"`
		Version int                    `json:"version"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.SetEquipmentStatus(r.Context(), id, req.Status, req.Version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// createChamber 建档腔体。
func (h *handlers) createChamber(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Code       string `json:"code"`
		Capability string `json:"capability"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.CreateChamber(r.Context(), id, req.Code, req.Capability)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listChambers(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.ListChambers(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// createQualification 建档资质窗口。
func (h *handlers) createQualification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EquipmentID string `json:"equipment_id"`
		ChamberID   string `json:"chamber_id"`
		StationID   string `json:"station_id"`
		ValidFrom   string `json:"valid_from"`
		ValidTo     string `json:"valid_to"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	from, err := time.Parse(time.RFC3339, req.ValidFrom)
	if err != nil {
		writeError(w, domain.NewValidationError(domain.FieldError{Field: "valid_from", Message: "必须是 RFC3339 时间"}))
		return
	}
	to, err := time.Parse(time.RFC3339, req.ValidTo)
	if err != nil {
		writeError(w, domain.NewValidationError(domain.FieldError{Field: "valid_to", Message: "必须是 RFC3339 时间"}))
		return
	}
	out, err := h.svc.Master.CreateQualification(r.Context(), req.EquipmentID, req.ChamberID, req.StationID, from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listQualifications(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListQualifications(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// createPlan 建档量测计划。
func (h *handlers) createPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code            string  `json:"code"`
		Name            string  `json:"name"`
		Metric          string  `json:"metric"`
		SamplePositions []int   `json:"sample_positions"`
		MinSamples      int     `json:"min_samples"`
		PassLimit       float64 `json:"pass_limit"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.CreatePlan(r.Context(), req.Code, req.Name, req.Metric, req.SamplePositions, req.MinSamples, req.PassLimit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listPlans(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListPlans(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getPlan(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.GetPlan(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// activatePlan 启用量测计划。
func (h *handlers) activatePlan(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Version int `json:"version"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.ActivatePlan(r.Context(), id, req.Version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}
