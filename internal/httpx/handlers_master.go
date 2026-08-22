package httpx

import (
	"net/http"
)

// createProductFamily 建档产品族。
func (h *handlers) createProductFamily(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.svc.Master.CreateProductFamily(r.Context(), req.Code, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, p)
}

func (h *handlers) listProductFamilies(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListProductFamilies(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getProductFamily(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := h.svc.Master.GetProductFamily(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, p)
}

// createStation 建档站点。
func (h *handlers) createStation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code       string `json:"code"`
		Name       string `json:"name"`
		Capability string `json:"capability"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	st, err := h.svc.Master.CreateStation(r.Context(), req.Code, req.Name, req.Capability)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, st)
}

func (h *handlers) listStations(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListStations(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getStation(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	st, err := h.svc.Master.GetStation(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, st)
}
