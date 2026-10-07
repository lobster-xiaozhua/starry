// Agent 侧首批单元测试：只覆盖不依赖 LLM / 网络的纯函数。
// 运行：npm test --workspace starry-agent
import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { HumanMessage, SystemMessage, ToolMessage, AIMessage } from "@langchain/core/messages";

import { extractUsage, toHistoryMessages, type RawMessage } from "./history.js";

describe("toHistoryMessages", () => {
  it("把 user/assistant 行映射为对应角色", () => {
    const rows: RawMessage[] = [
      { role: "user", content: "你好" },
      { role: "assistant", content: "hi" },
    ];
    const msgs = toHistoryMessages(rows);
    assert.equal(msgs.length, 2);
    assert.ok(msgs[0] instanceof HumanMessage);
    assert.ok(msgs[1] instanceof AIMessage);
  });

  it("取最后一条 summary 作为 SystemMessage 前置，且不重复输出 summary", () => {
    const rows: RawMessage[] = [
      { role: "summary", content: "旧摘要" },
      { role: "user", content: "a" },
      { role: "summary", content: "新摘要" },
      { role: "user", content: "b" },
    ];
    const msgs = toHistoryMessages(rows);
    assert.equal(msgs.length, 3);
    assert.ok(msgs[0] instanceof SystemMessage);
    assert.match((msgs[0] as SystemMessage).content as string, /新摘要/);
    assert.ok(!msgs.some((m) => (m as SystemMessage) === msgs[0] && /旧摘要/.test(m.content as string)));
  });

  it("tool 行保留 tool_call_id，缺失时回落为 unknown", () => {
    const rows: RawMessage[] = [
      { role: "tool", content: "结果", toolCallId: "call-1" },
      { role: "tool", content: "结果2" },
    ];
    const msgs = toHistoryMessages(rows) as ToolMessage[];
    assert.equal(msgs[0].tool_call_id, "call-1");
    assert.equal(msgs[1].tool_call_id, "unknown");
  });

  // 损坏的 toolCalls 会让 JSON.parse 抛错；这里必须吞掉并降级为「无工具调用」，
  // 否则一条脏数据就会让整次对话回放失败。
  it("assistant 的 toolCalls 解析失败时降级为空而不是抛错", () => {
    const rows: RawMessage[] = [{ role: "assistant", content: "x", toolCalls: "{bad json" }];
    const msgs = toHistoryMessages(rows) as AIMessage[];
    assert.deepEqual(msgs[0].tool_calls ?? [], []);
  });

  it("assistant 携带合法 toolCalls 时原样保留", () => {
    const calls = [{ name: "search", args: { q: "1" }, id: "call-9" }];
    const rows: RawMessage[] = [
      { role: "assistant", content: "x", toolCalls: JSON.stringify(calls) },
    ];
    const msgs = toHistoryMessages(rows) as AIMessage[];
    assert.deepEqual(msgs[0].tool_calls, calls);
  });
});

describe("extractUsage", () => {
  it("优先读取 LangChain usage_metadata", () => {
    const out = {
      usage_metadata: { input_tokens: 11, output_tokens: 22 },
      response_metadata: { token_usage: { prompt_tokens: 99, completion_tokens: 99 } },
    };
    assert.deepEqual(extractUsage(out), { promptTokens: 11, completionTokens: 22 });
  });

  it("兼容 OpenAI token_usage", () => {
    const out = { response_metadata: { token_usage: { prompt_tokens: 7, completion_tokens: 8 } } };
    assert.deepEqual(extractUsage(out), { promptTokens: 7, completionTokens: 8 });
  });

  it("缺失用量时返回 0 而不是 NaN/undefined", () => {
    assert.deepEqual(extractUsage(undefined), { promptTokens: 0, completionTokens: 0 });
    assert.deepEqual(extractUsage({}), { promptTokens: 0, completionTokens: 0 });
  });
});
