package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"gowork/wafer/internal/domain"
)

// errorBody 统一 JSON 错误结构。
type errorBody struct {
	Error  string              `json:"error"`
	Code   string              `json:"code"`
	Fields []domain.FieldError `json:"fields,omitempty"`
}

// envelope 统一成功响应结构。
type envelope struct {
	Data any `json:"data"`
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeData 写成功响应。
func writeData(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, envelope{Data: v})
}

// writeError 把领域错误映射为统一 JSON 错误。
func writeError(w http.ResponseWriter, err error) {
	status, code := mapError(err)
	body := errorBody{Error: err.Error(), Code: code}
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		body.Fields = ve.Fields
		body.Error = "参数校验失败"
	}
	writeJSON(w, status, body)
}

// mapError 领域错误到 HTTP 状态码与错误码的映射。
func mapError(err error) (int, string) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve), errors.Is(err, domain.ErrValidation):
		return http.StatusBadRequest, "VALIDATION"
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "CONFLICT"
	case errors.Is(err, domain.ErrDuplicate):
		return http.StatusConflict, "DUPLICATE"
	case errors.Is(err, domain.ErrInvalidState):
		return http.StatusUnprocessableEntity, "INVALID_STATE"
	case errors.Is(err, domain.ErrFrozen), errors.Is(err, domain.ErrNotFrozen):
		return http.StatusUnprocessableEntity, "FREEZE"
	case errors.Is(err, domain.ErrHoldBlocking):
		return http.StatusUnprocessableEntity, "HOLD_BLOCKING"
	case errors.Is(err, domain.ErrStationSkip):
		return http.StatusUnprocessableEntity, "STATION_SKIP"
	case errors.Is(err, domain.ErrQualification):
		return http.StatusUnprocessableEntity, "QUALIFICATION"
	case errors.Is(err, domain.ErrCapability):
		return http.StatusUnprocessableEntity, "CAPABILITY"
	case errors.Is(err, domain.ErrRecipeFamily):
		return http.StatusUnprocessableEntity, "RECIPE_FAMILY"
	case errors.Is(err, domain.ErrWaferBusy):
		return http.StatusUnprocessableEntity, "WAFER_BUSY"
	case errors.Is(err, domain.ErrSampling):
		return http.StatusUnprocessableEntity, "SAMPLING"
	case errors.Is(err, domain.ErrRunNotCompleted):
		return http.StatusUnprocessableEntity, "RUN_NOT_COMPLETED"
	case errors.Is(err, domain.ErrImmutable):
		return http.StatusUnprocessableEntity, "IMMUTABLE"
	default:
		return http.StatusInternalServerError, "INTERNAL"
	}
}
