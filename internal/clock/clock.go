package clock

import (
	"sync"
	"time"
)

// Clock 可注入时钟：生产使用 Real，测试使用 Manual。
type Clock interface {
	Now() time.Time
}

// Real 真实时钟。
type Real struct{}

// Now 返回当前 UTC 时间。
func (Real) Now() time.Time { return time.Now().UTC() }

// Manual 手动时钟，用于测试时间边界。
type Manual struct {
	mu sync.Mutex
	t  time.Time
}

// NewManual 创建手动时钟。
func NewManual(t time.Time) *Manual { return &Manual{t: t.UTC()} }

// Now 返回手动时钟当前时间。
func (m *Manual) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.t
}

// Advance 推进手动时钟。
func (m *Manual) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.t = m.t.Add(d)
}

// Set 设定手动时钟。
func (m *Manual) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.t = t.UTC()
}
