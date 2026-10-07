// Package logx 统一后端的结构化日志入口。
//
// 约定：全仓库不再直接使用标准库 log，统一走 slog（logx.L() 获取 logger）。
// 生产环境输出 JSON（便于采集到 Loki/ES/云日志），开发环境输出可读文本。
package logx

import (
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	cur    *slog.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	curEnv string       = "development"
)

// Setup 按运行环境初始化全局 logger，应在组合根最早处调用一次。
// level 为空时按环境推导：production → info，其它 → debug。
func Setup(env string, level string) {
	env = strings.ToLower(strings.TrimSpace(env))
	if env == "" {
		env = "development"
	}
	lvl := parseLevel(level)
	if level == "" {
		if env == "production" {
			lvl = slog.LevelInfo
		} else {
			lvl = slog.LevelDebug
		}
	}

	var h slog.Handler
	opts := &slog.HandlerOptions{Level: lvl}
	if env == "production" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}

	mu.Lock()
	cur = slog.New(h).With(slog.String("service", "starry-api"), slog.String("env", env))
	curEnv = env
	mu.Unlock()

	slog.SetDefault(L())
}

// SetLogger 替换全局 logger。测试用它把输出接到 buffer 上做断言，
// 生产若需接入自建采集器（如 otel handler）也可在此注入。
func SetLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	mu.Lock()
	cur = l
	mu.Unlock()
	slog.SetDefault(l)
}

// L 返回当前 logger。
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return cur
}

// Env 返回当前运行环境标签，供需要按环境降级的代码判断。
func Env() string {
	mu.RLock()
	defer mu.RUnlock()
	return curEnv
}

// IsProduction 判断是否为生产环境（影响降级策略与明文调试输出）。
func IsProduction() bool {
	return Env() == "production"
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info", "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}
