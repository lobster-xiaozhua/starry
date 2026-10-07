// 独立向量化（嵌入）服务：从对话/Agent 编排服务中拆出，仅负责文本 → 向量。
// 解耦的目的：嵌入模型（@xenova/transformers，依赖 onnxruntime 原生二进制 + HuggingFace
// 模型下载）是「重且脆」的一环；一旦模型下载被网络拦截，只会影响知识库入库/检索，
// 绝不会再像拆分前那样把整个对话服务一起拖垮（502 连环爆）。
import "dotenv/config";

import { createApp, type ModelState } from "./app.js";
import { embedTexts } from "./embed.js";

const PORT = parseInt(process.env.PORT || "3002", 10);

// 内部端点鉴权令牌；未配置时视为开发环境放行。
const INTERNAL_TOKEN = process.env.AGENT_INTERNAL_TOKEN || "";

// 嵌入模型加载状态：异步预热，不阻塞进程启动；供 /health 暴露以便运维观测。
// 属于「精细调控」里可观测性的一环——一眼能看出向量化是否就绪。
let modelState: ModelState = "pending";
let modelError = "";

const app = createApp({
  internalToken: INTERNAL_TOKEN,
  embedService: { embed: embedTexts },
  state: () => ({ state: modelState, error: modelError }),
});

// 预热：先加载模型再对外提供向量化，避免首个请求等几十秒。
// 预热失败不退出进程——保持存活并持续以 503 上报，便于运维定位而非反复重启。
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

app.listen(PORT, () => {
  console.log(`[embed] listening on :${PORT}`);
});
