// 历史回放与摘要：归 Node 侧负责的「摘要逻辑、Tokenizer、LLM 调用」。
// Go 仅提供窗口裁剪与原始压缩，不感知摘要语义。
import {
  AIMessage,
  HumanMessage,
  SystemMessage,
  ToolMessage,
  type BaseMessage,
} from "@langchain/core/messages";
import type { ChatOpenAI } from "@langchain/openai";
import { deleteMessages, saveMessage } from "./backend-client.js";

// 历史回放预算（粗略 token 估算）。超过则触发对旧消息的摘要。
// 用 chars/4 估算避免引入 tokenizer 依赖；精确 token 由 LLM usage 上报。
const HISTORY_TOKEN_BUDGET = 4000;
const CHARS_PER_TOKEN = 4;
const MIN_ROWS_TO_SUMMARIZE = 8;

export interface RawMessage {
  id?: string;
  role: string;
  content: string;
  toolCalls?: string;
  toolCallId?: string;
  createdAt?: string;
}

export interface UsageDelta {
  promptTokens: number;
  completionTokens: number;
}

const SUMMARIZE_PROMPT =
  "你是对话压缩器。请将下列对话浓缩成一段简洁的中文摘要，保留：" +
  "关键事实、已操作的笔记（标题/ID）、用户偏好与意图、尚未完成的事项。" +
  "不要保留寒暄与冗余工具输出。";

// 将 Go 返回的原始消息行转换为 LangChain BaseMessage[]。
// 若存在 role=summary 的消息，取最后一条作为 SystemMessage 前置；
// 其余非 summary 消息按序保留（被摘要替代的原始消息已由 Go 侧 DeleteMessages 清理）。
export function toHistoryMessages(rows: RawMessage[]): BaseMessage[] {
  const out: BaseMessage[] = [];
  let summaryIdx = -1;
  for (let i = rows.length - 1; i >= 0; i--) {
    if (rows[i].role === "summary") {
      summaryIdx = i;
      break;
    }
  }
  if (summaryIdx >= 0) {
    out.push(
      new SystemMessage(`以下是此前对话的摘要，作为上下文：\n${rows[summaryIdx].content}`),
    );
  }
  for (let i = 0; i < rows.length; i++) {
    const m = rows[i];
    if (m.role === "summary") continue;
    if (m.role === "user") {
      out.push(new HumanMessage(m.content));
    } else if (m.role === "tool") {
      out.push(
        new ToolMessage({ content: m.content, tool_call_id: m.toolCallId || "unknown" }),
      );
    } else {
      // assistant
      let toolCalls: any;
      try {
        toolCalls = m.toolCalls ? JSON.parse(m.toolCalls) : undefined;
      } catch {
        toolCalls = undefined;
      }
      out.push(new AIMessage({ content: m.content, tool_calls: toolCalls }));
    }
  }
  return out;
}

function estimateTokens(msgs: BaseMessage[]): number {
  let chars = 0;
  for (const m of msgs) {
    const c = m.content;
    if (typeof c === "string") {
      chars += c.length;
    } else if (c != null) {
      // 非字符串内容（图像/工具结果数组等）按序列化长度粗估
      chars += JSON.stringify(c).length;
    }
  }
  return Math.ceil(chars / CHARS_PER_TOKEN);
}

function textOf(content: unknown): string {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((p: any) => (typeof p === "string" ? p : p?.text || ""))
      .join("");
  }
  if (content == null) return "";
  return String(content);
}

// 从 LLM 输出中提取 token 用量，兼容 LangChain usage_metadata 与 OpenAI token_usage。
export function extractUsage(output: any): UsageDelta {
  const um = output?.usage_metadata;
  if (um && (um.input_tokens || um.output_tokens)) {
    return { promptTokens: um.input_tokens || 0, completionTokens: um.output_tokens || 0 };
  }
  const tu = output?.response_metadata?.token_usage;
  if (tu && (tu.prompt_tokens || tu.completion_tokens)) {
    return { promptTokens: tu.prompt_tokens || 0, completionTokens: tu.completion_tokens || 0 };
  }
  return { promptTokens: 0, completionTokens: 0 };
}

// 若历史超过预算，对较早部分做摘要并落库，删除已被替代的原始消息，
// 返回精简后的历史与本次摘要调用产生的 token 用量。
// 摘要逻辑、tokenizer 估算、LLM 调用全部在 Node；Go 不感知。
export async function maybeSummarize(
  token: string,
  conversationId: string,
  rows: RawMessage[],
  model: ChatOpenAI,
): Promise<{ messages: BaseMessage[]; usage: UsageDelta }> {
  const history = toHistoryMessages(rows);
  const noUsage: UsageDelta = { promptTokens: 0, completionTokens: 0 };
  if (rows.length < MIN_ROWS_TO_SUMMARIZE || estimateTokens(history) <= HISTORY_TOKEN_BUDGET) {
    return { messages: history, usage: noUsage };
  }

  // effective = 最后一条 summary 之后的消息（摘要只压缩未被摘要过的部分）
  let summaryIdx = -1;
  for (let i = rows.length - 1; i >= 0; i--) {
    if (rows[i].role === "summary") {
      summaryIdx = i;
      break;
    }
  }
  const effective = summaryIdx >= 0 ? rows.slice(summaryIdx + 1) : rows.slice();
  if (effective.length < 6) {
    return { messages: history, usage: noUsage };
  }

  // 保留最近 ~40%，其余送入摘要
  const keepCount = Math.max(4, Math.floor(effective.length * 0.4));
  const splitAt = effective.length - keepCount;
  if (splitAt <= 0) {
    return { messages: history, usage: noUsage };
  }
  const toSummarize = effective.slice(0, splitAt);
  const keepRows = effective.slice(splitAt);

  let summaryText = "";
  let usage: UsageDelta = noUsage;
  try {
    const result = await model.invoke([
      new SystemMessage(SUMMARIZE_PROMPT),
      ...toHistoryMessages(toSummarize),
      new HumanMessage("请用中文输出对以上对话的简洁摘要。"),
    ]);
    summaryText = textOf(result.content);
    usage = extractUsage(result);
  } catch (e) {
    // 摘要失败则回退为不摘要，避免阻断主对话
    console.warn("[agent] summarize failed, skipping:", e);
    return { messages: history, usage: noUsage };
  }

  // 落库摘要 + 删除已被替代的原始消息（含旧 summary，若有）
  const toDelete: string[] = [];
  if (summaryIdx >= 0 && rows[summaryIdx].id) toDelete.push(rows[summaryIdx].id!);
  for (const r of toSummarize) if (r.id) toDelete.push(r.id!);

  await saveMessage(token, conversationId, { role: "summary", content: summaryText });
  if (toDelete.length > 0) {
    await deleteMessages(token, conversationId, toDelete);
  }

  const kept = toHistoryMessages(keepRows);
  return {
    messages: [
      new SystemMessage(`以下是此前对话的摘要，作为上下文：\n${summaryText}`),
      ...kept,
    ],
    usage,
  };
}
