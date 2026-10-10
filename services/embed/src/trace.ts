// 分布式追踪（可选）：OTEL_EXPORTER_OTLP_ENDPOINT 已配置时通过 OTLP gRPC 导出 trace，
// 使 embed 请求（后端 → embed）挂在调用方的 trace 下；未配置时零开销透传。
import { trace, SpanStatusCode } from "@opentelemetry/api";
import logger from "./logger.js";

let initialized = false;

export async function initTracing(serviceName: string): Promise<void> {
  const endpoint = (process.env.OTEL_EXPORTER_OTLP_ENDPOINT || "").trim();
  if (!endpoint) return; // 未配置：保持 no-op tracer
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
    logger.info({ endpoint }, "otel tracing enabled");
  } catch (e: any) {
    initialized = false;
    logger.warn({ err: e }, "otel tracing init failed, staying no-op");
  }
}

/** 在独立 span 中执行 fn；追踪未启用时等价于直接调用 fn。 */
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
