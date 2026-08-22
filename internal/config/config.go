package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 服务配置：全部来自环境变量。
type Config struct {
	Port   int    `json:"port"`
	DBPath string `json:"db_path"`
	// ShutdownTimeout 优雅关闭超时。
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	// JobInterval 后台作业调度间隔。
	JobInterval time.Duration `json:"job_interval"`
	// RunTimeout 运行超时阈值（超时运行检查使用）。
	RunTimeout time.Duration `json:"run_timeout"`
	// HoldEscalateAfter 暂扣升级阈值。
	HoldEscalateAfter time.Duration `json:"hold_escalate_after"`
	// JobMaxAttempts 作业最大重试次数。
	JobMaxAttempts int `json:"job_max_attempts"`
}

// Load 读取环境变量并校验。PORT 默认 8080，DB_PATH 必填（禁止 :memory:）。
func Load() (Config, error) {
	var c Config
	port := os.Getenv("PORT")
	if port == "" {
		c.Port = 8080
	} else {
		p, err := strconv.Atoi(port)
		if err != nil || p <= 0 || p > 65535 {
			return c, fmt.Errorf("非法 PORT: %q", port)
		}
		c.Port = p
	}
	c.DBPath = os.Getenv("DB_PATH")
	if c.DBPath == "" {
		return c, fmt.Errorf("必须设置 DB_PATH 指向可复用的 SQLite 数据库文件")
	}
	if c.DBPath == ":memory:" {
		return c, fmt.Errorf("DB_PATH 不允许使用 :memory:")
	}
	c.ShutdownTimeout = envDuration("SHUTDOWN_TIMEOUT", 10*time.Second)
	c.JobInterval = envDuration("JOB_INTERVAL", 5*time.Second)
	c.RunTimeout = envDuration("RUN_TIMEOUT", 30*time.Minute)
	c.HoldEscalateAfter = envDuration("HOLD_ESCALATE_AFTER", time.Hour)
	c.JobMaxAttempts = envInt("JOB_MAX_ATTEMPTS", 3)
	return c, nil
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
