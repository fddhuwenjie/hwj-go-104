package httpx

import (
	"net/http"

	"gowork/wafer/internal/domain"
)

// createRoute 建档工艺路线。
func (h *handlers) createRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductFamilyID string `json:"product_family_id"`
		Code            string `json:"code"`
		Name            string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.CreateRoute(r.Context(), req.ProductFamilyID, req.Code, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listRoutes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListRoutes(r.Context(), r.URL.Query().Get("product_family_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getRoute(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.GetRoute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// createRevision 创建路线修订草稿。
func (h *handlers) createRevision(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Stations []struct {
			Seq             int    `json:"seq"`
			StationID       string `json:"station_id"`
			RecipeID        string `json:"recipe_id"`
			MetrologyPlanID string `json:"metrology_plan_id"`
		} `json:"stations"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	var stations []domain.RouteStation
	for _, st := range req.Stations {
		stations = append(stations, domain.RouteStation{
			Seq:             st.Seq,
			StationID:       st.StationID,
			RecipeID:        st.RecipeID,
			MetrologyPlanID: st.MetrologyPlanID,
		})
	}
	out, err := h.svc.Master.CreateRevision(r.Context(), id, stations)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listRevisions(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.ListRevisions(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getRevision(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.GetRevision(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) listRevisionStations(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.ListRouteStations(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// activateRevision 启用路线修订（携带乐观锁版本号）。
func (h *handlers) activateRevision(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.svc.Master.ActivateRevision(r.Context(), id, req.Version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}
