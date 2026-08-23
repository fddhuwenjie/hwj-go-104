package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"gowork/wafer/internal/repository"
)

// Store SQLite 仓储实现，实现 repository.Store 全部接口。
type Store struct {
	db *sql.DB
}

// 编译期接口断言。
var _ repository.Store = (*Store)(nil)

// Open 打开（必要时创建）SQLite 数据库文件并执行迁移。
// path 必须是文件路径，禁止 :memory:。
func Open(path string) (*Store, error) {
	if path == "" || path == ":memory:" {
		return nil, fmt.Errorf("sqlite: 必须提供可复用的数据库文件路径")
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	// 单写者：限制连接数避免 SQLITE_BUSY。
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// Ping 健康检查。
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
