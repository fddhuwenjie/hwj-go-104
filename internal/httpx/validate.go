package httpx

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gowork/wafer/internal/domain"
)

// decodeJSON 解析请求体 JSON；空体返回 nil（允许无参 POST）。
func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return domain.NewValidationError(domain.FieldError{Field: "body", Message: "请求体必须是合法 JSON: " + err.Error()})
	}
	return nil
}

// queryInt 解析非负整数查询参数。
func queryInt(r *http.Request, key string, def int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, domain.NewValidationError(domain.FieldError{Field: key, Message: "必须是非负整数"})
	}
	return n, nil
}

// queryPage 解析分页参数。
func queryPage(r *http.Request) (domain.Page, error) {
	limit, err := queryInt(r, "limit", 50)
	if err != nil {
		return domain.Page{}, err
	}
	return domain.Page{Limit: limit, Cursor: r.URL.Query().Get("cursor")}.Normalize(), nil
}

// pathValue 提取路径参数并校验非空。
func pathValue(r *http.Request, key string) (string, error) {
	v := pathParam(r, key)
	if v == "" {
		return "", domain.NewValidationError(domain.FieldError{Field: key, Message: "路径参数不能为空"})
	}
	return v, nil
}
