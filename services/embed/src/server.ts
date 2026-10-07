// 独立向量化（嵌入）服务：从对话/Agent 编排服务中拆出，仅负责文本 → 向量。
// 解耦的目的：嵌入模型（@xenova/transformers，依赖 onnxruntime 原生二进制 + HuggingFace
// 模型下载）是「重且脆」的一环；一旦模型下载被网络拦截，只会影响知识库入库/检索，
// 绝不会再像拆分前那样把整个对话服务一起拖垮（502 连环爆）。
import "dotenv/config";
import express from "express";
import cors from "cors";

import { embedTexts } from "./embed.js";

const app = express();
app.use(
  cors({
    origin: process.env.AGENT_CORS_ORIGIN || "http://127.0.0.1:5173",
    credentials: true,
  }),
);
app.use(express.json({ limit: "2mb" }));

const PORT = parseInt(process.env.PORT || "3002", 10);

// 内部向量化端点：供 Go 后端在知识库入库/检索时调用。用 X-Internal-Token 保护，
// 仅允许受信任的内部服务访问（未配置 AGENT_INTERNAL_TOKEN 时视为开发环境放行）。
const INTERNAL_TOKEN = process.env.AGENT_INTERNAL_TOKEN || "";

// 嵌入模型加载状态：异步预热，不阻塞进程启动；供 /health 暴露以便运维观测。
// 属于「精细调控」里可观测性的一环——一眼能看出向量化是否就绪。
let modelState: "pending" | "ready" | "error" = "pending";
let modelError = "";

(async () => {
  try {
    await embedTexts(["warmup"]);
    modelState = "ready";
    console.log("[embed] embedding model ready");
  } catch (e: any) {
    modelState = "error";
    modelError = e?.message || String(e);
    console.error("[embed] embedding model load failed:", e);
  }
})();

app.post("/api/agent/embed", async (req, res) => {
  if (INTERNAL_TOKEN && req.header("X-Internal-Token") !== INTERNAL_TOKEN) {
    res.status(401).json({ code: 1005, message: "internal token required" });
    return;
  }
  if (modelState !== "ready") {
    res.status(503).json({
      code: 3003,
      message: "embedding model not ready: " + modelError,
    });
    return;
  }
  const texts = (req.body?.texts as string[]) || [];
  if (!Array.isArray(texts) || texts.length === 0) {
    res.status(400).json({ code: 3001, message: "texts required" });
    return;
  }
  try {
    const embeddings = await embedTexts(texts.slice(0, 64));
    res.json({ embeddings });
  } catch (e: any) {
    console.error("[embed] embed failed:", e);
    res.status(500).json({ code: 3002, message: e?.message || "embed failed" });
  }
});

// 健康检查：始终返回 200（liveness），embed 服务的 readiness（模型是否就绪）随 embedding 字段暴露。
app.get("/health", (_req, res) => {
  res.json({ status: "ok", embedding: modelState });
});

app.listen(PORT, () => {
  console.log(`[embed] listening on :${PORT}`);
});
