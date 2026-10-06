import "dotenv/config";
import express from "express";
import cors from "cors";
import { authMiddleware } from "./jwt.js";
import { requestContext } from "./context.js";
import { supervisorExecutor, dailyExecutor } from "./agents.js";
import { createModel } from "./llm.js";
import { embedTexts } from "./embed.js";
import { maybeSummarize, extractUsage } from "./history.js";
import {
  listMessages,
  saveMessage,
  recordUsage,
} from "./backend-client.js";
import {
  createAndRunLongTask,
  subscribeTask,
} from "./tasks.js";
import {
  getTaskRaw,
  listTasks,
  cancelTask,
} from "./backend-client.js";
import {
  AIMessage,
  HumanMessage,
  ToolMessage,
  type BaseMessage,
} from "@langchain/core/messages";

const app = express();
app.use(cors({ origin: process.env.AGENT_CORS_ORIGIN || "http://127.0.0.1:5173", credentials: true }));
app.use(express.json({ limit: "1mb" }));

const JWT_SECRET = process.env.JWT_SECRET || "notes-test-secret-fixed";
const PORT = parseInt(process.env.PORT || "3001", 10);

// 内部向量化端点：供 Go 后端在知识库入库/检索时调用。用 X-Internal-Token 保护，
// 仅允许受信任的内部服务访问（未配置 AGENT_INTERNAL_TOKEN 时视为开发环境放行）。
const INTERNAL_TOKEN = process.env.AGENT_INTERNAL_TOKEN || "";
app.post("/api/agent/embed", async (req, res) => {
  if (INTERNAL_TOKEN && req.header("X-Internal-Token") !== INTERNAL_TOKEN) {
    res.status(401).json({ code: 1005, message: "internal token required" });
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
    console.error("[agent] embed failed:", e);
    res.status(500).json({ code: 3002, message: e?.message || "embed failed" });
  }
});

app.use("/api/agent", authMiddleware(JWT_SECRET));

// POST /api/agent/chat — SSE 流式返回 agent 回复
app.post("/api/agent/chat", async (req, res) => {
  const userId = req.userId!;
  const token = req.userToken!;
  const { conversationId, message, mode } = req.body as {
    conversationId: string;
    message: string;
    mode?: string;
  };
  const isDaily = mode === "daily";

  if (!conversationId || !message) {
    res.status(400).json({ code: 3001, message: "缺少 conversationId 或 message" });
    return;
  }

  // SSE headers
  res.setHeader("Content-Type", "text/event-stream");
  res.setHeader("Cache-Control", "no-cache");
  res.setHeader("Connection", "keep-alive");
  res.setHeader("X-Accel-Buffering", "no");
  res.flushHeaders?.();

  const send = (event: string, data: unknown) => {
    res.write(`event: ${event}\n`);
    res.write(`data: ${JSON.stringify(data)}\n\n`);
  };

  // 客户端断开（关闭页面/中断生成）时提前结束流式循环，避免继续消耗 LLM token。
  let closed = false;
  req.on("close", () => {
    closed = true;
  });

  try {
    // 1. 从 Go 后端加载对话历史（窗口裁剪 + 原始压缩在 Go 侧完成）
    const historyData = await listMessages(token, conversationId, {
      limit: 50,
      compress: true,
    });
    const rows = ((historyData as any)?.messages ?? []) as Array<{
      id?: string;
      role: string;
      content: string;
      toolCalls?: string;
      toolCallId?: string;
      createdAt?: string;
    }>;

    // 2. token 预算 + 摘要（摘要逻辑、tokenizer、LLM 调用均在 Node）
    const summarizeModel = createModel();
    const { messages: historyBase, usage: summaryUsage } = await maybeSummarize(
      token,
      conversationId,
      rows,
      summarizeModel,
    );

    // 3. 在 requestContext 中运行 agent，工具通过 AsyncLocalStorage 获取 token
    const newMessages: BaseMessage[] = [new HumanMessage(message)];

    await requestContext.run({ token, userId, role: req.userRole! }, async () => {
      const executor = isDaily ? dailyExecutor : supervisorExecutor;
      const eventStream = executor.streamEvents(
        { messages: [...historyBase, ...newMessages] },
        { version: "v2" },
      );

      let assistantContent = "";
      const agentUsage = { promptTokens: 0, completionTokens: 0 };
      const persistedMessages: Array<{
        role: "assistant" | "tool";
        content: string;
        toolCalls?: string;
        toolCallId?: string;
      }> = [];
      const pendingToolCallIds: string[] = [];

      for await (const event of eventStream) {
        if (closed) break;
        // LLM token 流
        if (event.event === "on_chat_model_stream") {
          const chunk = event.data?.chunk;
          const token = textContent(chunk?.content);
          if (token.length > 0) {
            send("token", { content: token });
          }
        }
        // LLM 输出完成 — 收集 tool_calls
        else if (event.event === "on_chat_model_end") {
          const output = event.data?.output;
          const content = textContent(output?.content);
          if (content.length > 0) assistantContent = content;
          // 累加本轮 LLM 用量（多轮工具调用会多次触发 on_chat_model_end）
          const u = extractUsage(output);
          agentUsage.promptTokens += u.promptTokens;
          agentUsage.completionTokens += u.completionTokens;
          if (output?.tool_calls?.length > 0) {
            const toolCalls = output.tool_calls as any[];
            for (const call of toolCalls) {
              if (call.id) pendingToolCallIds.push(call.id);
            }
            persistedMessages.push({
              role: "assistant",
              content,
              toolCalls: JSON.stringify(toolCalls),
            });
          }
        }
        // 工具调用开始
        else if (event.event === "on_tool_start") {
          send("tool_call", {
            id: event.id,
            name: event.name,
            args: event.data?.input,
          });
        }
        // 工具返回
        else if (event.event === "on_tool_end") {
          const result = textContent(event.data?.output);
          send("tool_result", {
            id: event.id,
            name: event.name,
            result: result.length > 500 ? result.slice(0, 500) + "…" : result,
          });
          persistedMessages.push({
            role: "tool",
            content: result,
            toolCallId: pendingToolCallIds.shift() || "unknown",
          });
        }
      }

      // 4. 持久化新消息到 Go 后端
      // 保存用户消息
      await saveMessage(token, conversationId, {
        role: "user",
        content: message,
      });

      // 保存 agent 产生的 assistant tool-call、tool result 和最终 assistant 消息。
      if (assistantContent || persistedMessages.length === 0) {
        persistedMessages.push({ role: "assistant", content: assistantContent });
      }
      for (const persisted of persistedMessages) {
        await saveMessage(token, conversationId, persisted);
      }

      // 上报 token 用量（agent 本轮 + 摘要调用）到 Go/Redis
      await recordUsage(token, {
        promptTokens: agentUsage.promptTokens + summaryUsage.promptTokens,
        completionTokens: agentUsage.completionTokens + summaryUsage.completionTokens,
      });

      // 客户端已断开则不再下发 done，节省一次网络写入
      if (!closed) {
        send("done", { content: assistantContent });
      }
    });
  } catch (err: any) {
    console.error("[agent] error:", err);
    send("error", { message: err?.message || "内部错误" });
  } finally {
    res.end();
  }
});

// ===== 长程任务 =====

// POST /api/agent/tasks — 创建并异步执行，立即返回任务 ID
app.post("/api/agent/tasks", async (req, res) => {
  const token = req.userToken!;
  const goal = (req.body?.goal as string) || "";
  if (!goal.trim()) {
    res.status(400).json({ code: 3001, message: "goal required" });
    return;
  }
  try {
    const id = await createAndRunLongTask(goal, token);
    res.json({ id });
  } catch (e: any) {
    res.status(500).json({ code: 3002, message: e?.message || "task failed" });
  }
});

// GET /api/agent/tasks — 任务列表
app.get("/api/agent/tasks", async (req, res) => {
  const token = req.userToken!;
  try {
    const data = await listTasks(token);
    res.json(data);
  } catch (e: any) {
    res.status(500).json({ code: 3002, message: e?.message || "list failed" });
  }
});

// GET /api/agent/tasks/:id — 任务详情
app.get("/api/agent/tasks/:id", async (req, res) => {
  const token = req.userToken!;
  try {
    const data = await getTaskRaw(token, req.params.id);
    res.json(data);
  } catch (e: any) {
    res.status(500).json({ code: 3002, message: e?.message || "get failed" });
  }
});

// POST /api/agent/tasks/:id/cancel — 取消任务（转发后端，使任务接口统一收敛到 agent 服务）
app.post("/api/agent/tasks/:id/cancel", async (req, res) => {
  const token = req.userToken!;
  try {
    const data = await cancelTask(token, req.params.id);
    res.json(data);
  } catch (e: any) {
    res.status(500).json({ code: 3002, message: e?.message || "cancel failed" });
  }
});

// GET /api/agent/tasks/:id/stream — SSE 实时进度流
app.get("/api/agent/tasks/:id/stream", async (req, res) => {
  const token = req.userToken!;
  const id = req.params.id;
  res.setHeader("Content-Type", "text/event-stream");
  res.setHeader("Cache-Control", "no-cache");
  res.setHeader("Connection", "keep-alive");
  res.setHeader("X-Accel-Buffering", "no");
  res.flushHeaders?.();
  const send = (event: string, data: unknown) => {
    res.write(`event: ${event}\n`);
    res.write(`data: ${JSON.stringify(data)}\n\n`);
  };
  // 先推送当前任务状态快照
  try {
    const t = await getTaskRaw(token, id);
    send("status", t);
  } catch {
    /* ignore */
  }
  const unsub = subscribeTask(id, (ev) => send(ev.type, ev));
  const ping = setInterval(() => res.write(": ping\n\n"), 15000);
  req.on("close", () => {
    clearInterval(ping);
    unsub();
  });
});

function textContent(content: unknown): string {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((part: any) => (typeof part === "string" ? part : part?.text || ""))
      .join("");
  }
  if (typeof content === "object" && content !== null && "content" in content) {
    return textContent((content as { content?: unknown }).content);
  }
  if (content == null) return "";
  return String(content);
}

// 健康检查
app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

app.listen(PORT, () => {
  console.log(`[agent] listening on :${PORT}`);
});
