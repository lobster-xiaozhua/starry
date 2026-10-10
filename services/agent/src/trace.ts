// 分布式追踪（可选）：OTEL_EXPORTER_OTLP_ENDPOINT 已配置时导出 trace 到 OTLP gRPC（如 Jaeger 4317），
// 未配置时所有导出为空、withSpan 退化为无实现 trace（零开销）。
// 与 Go 后端（otelgin + otelhttp）通过 W3C traceparent 串联成跨服务 trace。
import { trace, SpanStatusCode } from "@opentelemetry/api";
import logger from "./logger.js";

let initialized = false;

export async function initTracing(serviceName: string): Promise<void> {
  const endpoint = (process.env.OTEL_EXPORTER_OTLP_ENDPOINT || "").trim();
  if (!endpoint) return; // 未配置：保持 no-op tracer，不引入任何开销
  if (initialized) return;
  initialized = true;
  try {
    const { NodeSDK } = await import("@opentelemetry/sdk-node");
    const { getNodeAutoInstrumentations } = await import(
      "@opentelemetry/auto-instrumentations-node"
    );
    new NodeSDK({
      serviceName,
      instrumentations: [getNodeAutoInstrumentations()],
    }).start();
    // W3C traceparent 传播默认开启，保持跨服务串联；SDK 按 OTEL_EXPORTER_OTLP_ENDPOINT
    // 选择 OTLP 导出器。失败时仅告警：追踪是观测增强，不能反过来影响主链路。
    logger.info({ endpoint }, "otel tracing enabled");
  } catch (e: any) {
    initialized = false;
    logger.warn({ err: e }, "otel tracing init failed, staying no-op");
  }
}

/** 在独立 span 中执行 fn，供 LLM 调用、工具执行等关键点打点。 */
export async function withSpan<T>(name: string, fn: () => Promise<T>): Promise<T> {
  const tracer = trace.getTracer("starry");
  return tracer.startActiveSpan(name, async (span) => {
    try {
      const out = await fn();
      span.setStatus({ code: SpanStatusCode.OK });
      return out;
    } catch (e: any) {
      span.setStatus({ code: SpanStatusCode.ERROR, message: e?.message });
      span.recordException(e);
      throw e;
    } finally {
      span.end();
    }
  });
}
