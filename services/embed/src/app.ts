// 把路由装配从进程启动中抽离：server.ts 只负责「预热模型 + 监听端口」，
// createApp 只负责「给定依赖，产出可测的 express 应用」。
// 这样单元测试无需下载 25MB 嵌入模型，就能验证鉴权与降级契约。
import express from "express";
import cors from "cors";

export type ModelState = "pending" | "ready" | "error";

export interface EmbedService {
  embed: (texts: string[]) => Promise<number[][]>;
}

export interface AppDeps {
  internalToken: string;
  embedService: EmbedService;
  state: () => { state: ModelState; error: string };
  corsOrigin?: string;
}

// 单次请求最大文本数：防止后端一次灌入整库文档把服务打满。
export const MAX_TEXTS = 64;

export function createApp(deps: AppDeps): express.Express {
  const app = express();
  app.use(
    cors({
      origin: deps.corsOrigin || process.env.AGENT_CORS_ORIGIN || "http://127.0.0.1:5173",
      credentials: true,
    }),
  );
  app.use(express.json({ limit: "2mb" }));

  // 内部向量化端点：仅供 Go 后端调用，用 X-Internal-Token 保护。
  // 未配置 AGENT_INTERNAL_TOKEN 时视为开发环境放行。
  app.post("/api/agent/embed", async (req, res) => {
    if (deps.internalToken && req.header("X-Internal-Token") !== deps.internalToken) {
      res.status(401).json({ code: 1005, message: "internal token required" });
      return;
    }
    const model = deps.state();
    if (model.state !== "ready") {
      // 模型未就绪是「降级」而非「故障」：返回 503 让调用方明确知道可重试。
      res.status(503).json({
        code: 3003,
        message: "embedding model not ready: " + model.error,
      });
      return;
    }
    const texts = req.body?.texts as string[] | undefined;
    if (!Array.isArray(texts) || texts.length === 0) {
      res.status(400).json({ code: 3001, message: "texts required" });
      return;
    }
    try {
      const embeddings = await deps.embedService.embed(texts.slice(0, MAX_TEXTS));
      res.json({ embeddings });
    } catch (e: any) {
      console.error("[embed] embed failed:", e);
      res.status(500).json({ code: 3002, message: e?.message || "embed failed" });
    }
  });

  // 健康检查始终 200（liveness）；模型是否就绪作为 readiness 随 embedding 字段暴露。
  app.get("/health", (_req, res) => {
    res.json({ status: "ok", embedding: deps.state().state });
  });

  return app;
}
