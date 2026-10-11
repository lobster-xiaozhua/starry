package otel

import (
	"io"
	"log/slog"
	"testing"
)

// tracing 未配置时必须完全不产生副作用：Setup 返回 nil、Enabled 为 false、
// Shutdown 是安全的 no-op（调用方 main 无条件 defer Shutdown）。
func TestSetupDisabledWithoutEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if tp := Setup("starry-test", "", logger); tp != nil {
		t.Fatalf("Setup with empty endpoint = %v, want nil", tp)
	}
	if Enabled() {
		t.Fatal("Enabled() = true when OTEL_EXPORTER_OTLP_ENDPOINT is unset")
	}
	if err := Shutdown(); err != nil {
		t.Fatalf("Shutdown() while disabled = %v, want nil", err)
	}
}

// 只输入空白字符同样视为未配置，避免误挂 otelgin 中间件。
func TestSetupTreatsBlankEndpointAsDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if tp := Setup("starry-test", "   ", logger); tp != nil {
		t.Fatalf("Setup with blank endpoint = %v, want nil", tp)
	}
	if Enabled() {
		t.Fatal("Enabled() = true for blank endpoint")
	}
}
