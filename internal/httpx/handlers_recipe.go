package httpx

import (
	"encoding/json"
	"net/http"
)

// createRecipe 建档配方。
func (h *handlers) createRecipe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code            string `json:"code"`
		Name            string `json:"name"`
		EquipmentFamily string `json:"equipment_family"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.CreateRecipe(r.Context(), req.Code, req.Name, req.EquipmentFamily)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listRecipes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Master.ListRecipes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getRecipe(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.GetRecipe(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// createRecipeVersion 创建配方版本草稿。
func (h *handlers) createRecipeVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Params json.RawMessage `json:"params"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.CreateRecipeVersion(r.Context(), id, req.Params)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusCreated, out)
}

func (h *handlers) listRecipeVersions(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.ListRecipeVersions(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

func (h *handlers) getRecipeVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathValue(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.svc.Master.GetRecipeVersion(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}

// activateRecipeVersion 启用配方版本（生成不可变快照）。
func (h *handlers) activateRecipeVersion(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.svc.Master.ActivateRecipeVersion(r.Context(), id, req.Version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, out)
}
