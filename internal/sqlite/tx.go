package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// InTx 在单一数据库事务内执行 fn：
// fn 返回 error 则回滚，panic 同样回滚并重新抛出。
// fn 内通过 s.q(ctx) 自动复用该事务。
func (s *Store) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		// 已在事务中：直接执行，避免嵌套事务。
		return fn(ctx)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	ctx = context.WithValue(ctx, txKey{}, tx)
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		if e := tx.Commit(); e != nil {
			err = fmt.Errorf("commit tx: %w", e)
		}
	}()
	err = fn(ctx)
	return err
}
