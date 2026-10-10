// Package otel bootstraps the OpenTelemetry tracing pipeline for the backend.
//
// Design:
//   - tracing is entirely opt-in: when OTEL_EXPORTER_OTLP_ENDPOINT is unset,
//     Setup returns nil and callers use the no-op tracer (zero overhead path).
//   - the OTLP endpoint expects gRPC (Jaeger all-in-one exposes 4317).
//   - W3C tracecontext propagation is enabled so spans chain across
//     backend -> embed -> agent services.
package otel

import (
	"context"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Enabled reports whether tracing was configured and is live.
func Enabled() bool { return Tracer != nil }

// Tracer is the process-wide tracer provider; nil when tracing is disabled.
var Tracer *sdktrace.TracerProvider

// Setup initialises OTLP gRPC tracing when endpoint is non-empty,
// otherwise returns nil and leaves the global provider at its no-op default.
func Setup(serviceName, endpoint string, log *slog.Logger) *sdktrace.TracerProvider {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		log.Info("otel: OTEL_EXPORTER_OTLP_ENDPOINT 未配置，追踪保持关闭")
		return nil
	}

	// endpoint 允许带或不带 scheme（OTEL 规范要求带 scheme，这里兼容 "jaeger:4317" 裸写法）；
	// gRPC exporter 只要 host:port，所以统一解析出 Host。
	host := strings.TrimPrefix(endpoint, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimRight(host, "/")

	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpoint(host),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Error("otel: otlp grpc exporter init failed", "error", err)
		return nil
	}

	res, err := sdkresource.New(context.Background(),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		log.Error("otel: resource init failed", "error", err)
		return nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	)
	otel.SetTracerProvider(tp)
	// 记录到包级变量：Enabled() 依赖它决定是否挂 otelgin 中间件，Shutdown() 依赖它 flush。
	Tracer = tp
	// W3C tracecontext + baggage: enables cross-service span chaining.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info("otel: tracing enabled", "endpoint", endpoint, "service", serviceName)
	return tp
}

// Shutdown flushes pending spans; safe to call with a nil provider.
func Shutdown() error {
	if Tracer == nil {
		return nil
	}
	// Shutdown 会先尽力导出未完成的 span，无需单独 ForceFlush。
	return Tracer.Shutdown(context.Background())
}
