package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSetupSwitchesFormatByEnv(t *testing.T) {
	prev := L()
	defer SetLogger(prev)

	// 生产环境输出 JSON，便于日志采集；开发环境输出可读文本。
	Setup("production", "")
	if !IsProduction() {
		t.Fatalf("production 环境应识别为生产")
	}
	var buf bytes.Buffer
	// 直接校验 handler 类型：JSON handler 会输出 "level":"info" 这样的键。
	SetLogger(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	L().Info("hello", "k", "v")
	if !strings.Contains(buf.String(), `"level":"INFO"`) {
		t.Fatalf("生产日志应为 JSON: %s", buf.String())
	}

	Setup("development", "")
	if IsProduction() {
		t.Fatalf("development 不应识别为生产")
	}
	var buf2 bytes.Buffer
	SetLogger(slog.New(slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelDebug})))
	L().Debug("dbg")
	if !strings.Contains(buf2.String(), "level=DEBUG") {
		t.Fatalf("开发日志应为文本且含 debug: %s", buf2.String())
	}
}

func TestParseLevelFallsBackToInfo(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"WARN":  slog.LevelWarn,
		"error": slog.LevelError,
		"info":  slog.LevelInfo,
		"":      slog.LevelInfo,
		"乱码":    slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Fatalf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSetLoggerIgnoresNil(t *testing.T) {
	prev := L()
	defer SetLogger(prev)
	SetLogger(nil)
	if L() == nil {
		t.Fatalf("注入 nil 后不应把全局 logger 置空")
	}
}
