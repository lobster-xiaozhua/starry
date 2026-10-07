import "dotenv/config";
// 运行时冒烟测试：用真实 LLM 跑通 agent 平台核心链路（不依赖后端/Redis）。
// 1) 直接模型调用  2) 日常对话（无工具）  3) supervisor + 工具调用（计算器）
// 4) supervisor 委派子 agent（分析师，仅用本地计算器，免网络）
import { createModel } from "./src/llm.js";
import { supervisorExecutor, dailyExecutor } from "./src/agents.js";

function log(...a: unknown[]) {
  process.stdout.write(a.map((x) => (typeof x === "string" ? x : JSON.stringify(x))).join(" ") + "\n");
}

async function testBasicModel() {
  log("\n=== [1] 基础模型调用 ===");
  const m = createModel();
  const res = await m.invoke("用一句话介绍你自己，并说明你支持中文。");
  const text = typeof res.content === "string" ? res.content : JSON.stringify(res.content);
  log("模型回复:", text.slice(0, 200));
  if (!text.trim()) throw new Error("模型返回空");
  log("PASS");
}

async function runExecutor(label: string, executor: any, message: string) {
  log(`\n=== ${label} ===`);
  const events: Record<string, number> = {};
  let assistant = "";
  let toolCalls: string[] = [];
  for await (const ev of executor.streamEvents({ messages: [new (await import("@langchain/core/messages")).HumanMessage(message)] }, { version: "v2" })) {
    if (ev.event === "on_chat_model_stream") {
      const t = textOf(ev.data?.chunk?.content);
      if (t) process.stdout.write(t);
      events.token = (events.token || 0) + 1;
    } else if (ev.event === "on_tool_start") {
      toolCalls.push(ev.name);
      events.tool_start = (events.tool_start || 0) + 1;
      log(`\n  [tool_start] ${ev.name} args=${JSON.stringify(ev.data?.input)}`);
    } else if (ev.event === "on_tool_end") {
      events.tool_end = (events.tool_end || 0) + 1;
      log(`  [tool_end]   ${ev.name} => ${textOf(ev.data?.output).slice(0, 120)}`);
    } else if (ev.event === "on_chat_model_end") {
      const c = textOf(ev.data?.output?.content);
      if (c) assistant = c;
    }
  }
  log(`\n  事件统计: ${JSON.stringify(events)}  工具: ${toolCalls.join(",") || "无"}`);
  return { assistant, toolCalls };
}

function textOf(c: unknown): string {
  if (typeof c === "string") return c;
  if (Array.isArray(c)) return c.map((p: any) => (typeof p === "string" ? p : p?.text || "")).join("");
  return "";
}

async function main() {
  await testBasicModel();
  const daily = await runExecutor("[2] 日常对话(无工具)", dailyExecutor, "你好，用一句话说明你是什么助手。");
  if (!daily.assistant.trim()) throw new Error("日常对话无输出");

  const calc = await runExecutor(
    "[3] supervisor + 工具(计算器)",
    supervisorExecutor,
    "请计算 (123 + 456) * 2 等于多少？请使用计算器工具并给出最终数值。",
  );
  if (calc.toolCalls.length === 0) throw new Error("supervisor 未调用任何工具");

  const delegate = await runExecutor(
    "[4] supervisor 委派子 agent(分析师)",
    supervisorExecutor,
    "请把任务委派给 analyst（分析师）子 agent：计算 99 * 88 的结果并简要说明含义。只使用本地工具，不要联网。",
  );
  if (!delegate.toolCalls.some((t) => t.startsWith("delegate_to_"))) {
    throw new Error("supervisor 未委派子 agent");
  }

  log("\n========== 全部冒烟测试通过 ==========");
}

main().then(() => process.exit(0)).catch((e) => {
  log("\n!!! 测试失败:", e?.message || e);
  process.exit(1);
});
