package domain

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Page 稳定分页请求：游标基于 (created_at, id)，保证在数据追加时不漂移。
type Page struct {
	Limit  int
	Cursor string
}

// Normalize 收敛分页参数。
func (p Page) Normalize() Page {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	return p
}

// PageResult 分页结果。
type PageResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// CursorKey 游标键。
type CursorKey struct {
	CreatedAt time.Time
	ID        string
}

// EncodeCursor 编码游标。
func EncodeCursor(createdAt time.Time, id string) string {
	raw := fmt.Sprintf("%d|%s", createdAt.UnixMilli(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor 解码游标；非法游标返回校验错误。
func DecodeCursor(cursor string) (CursorKey, error) {
	var k CursorKey
	if cursor == "" {
		return k, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return k, NewValidationError(FieldError{Field: "cursor", Message: "非法游标"})
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return k, NewValidationError(FieldError{Field: "cursor", Message: "非法游标"})
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return k, NewValidationError(FieldError{Field: "cursor", Message: "非法游标"})
	}
	k.CreatedAt = time.UnixMilli(ms).UTC()
	k.ID = parts[1]
	return k, nil
}
