package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"gowork/wafer/internal/domain"
)

// txKey 事务上下文键。
type txKey struct{}

// querier 抽象 *sql.DB 与 *sql.Tx。
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// q 返回当前上下文可用的查询器：事务内为 tx，否则为 db。
func (s *Store) q(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return s.db
}

func ms(t time.Time) int64 { return t.UTC().UnixMilli() }

func tm(v int64) time.Time { return time.UnixMilli(v).UTC() }

func tmPtr(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.UnixMilli(v.Int64).UTC()
	return &t
}

func nullMs(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UTC().UnixMilli(), Valid: true}
}

// notFound 把 sql.ErrNoRows 映射为领域 ErrNotFound。
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// marshalJSON 序列化为 JSON 字符串。
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// unmarshalJSON 反序列化 JSON 字符串。
func unmarshalJSON(raw string, v any) error {
	return json.Unmarshal([]byte(raw), v)
}

// conflictIfNoRows 乐观锁更新辅助：影响行数为 0 时返回 ErrConflict。
func conflictIfNoRows(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrConflict
	}
	return nil
}

// placeholders 生成 SQL IN 占位符。
func placeholders(n int) string {
	if n <= 0 {
		return "(NULL)"
	}
	out := "("
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += "?"
	}
	return out + ")"
}
