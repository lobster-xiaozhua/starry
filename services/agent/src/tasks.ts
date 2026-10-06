// 长程任务执行器：把高层目标拆成多步计划，逐步通过 supervisor 执行，
// 每步检查点回写后端（agent_tasks），并通过 SSE 推送进度。支持取消与步数上限。
import { HumanMessage } from "@langchain/core/messages";
import { supervisorExecutor } from "./agents.js";
import { createModel } from "./llm.js";
import { agentContext } from "./context.js";
import {
  createTask as createTaskBE,
  getTaskRaw,
  updateTask as updateTaskBE,
} from "./backend-client.js";

const MAX_STEPS = 12;

type Sub = (ev: any) => void;
const subscribers = new Map<string, Set<Sub>>();

export function subscribeTask(taskId: string, cb: Sub): () => void {
  let set = subscribers.get(taskId);
  if (!set) {
    set = new Set();
    subscribers.set(taskId, set);
  }
  set.add(cb);
  return () => set!.delete(cb);
}

function emit(taskId: string, ev: any) {
  const set = subscribers.get(taskId);
  if (!set) return;
  for (const cb of set) {
    try {
      cb(ev);
    } catch {
      /* ignore */
    }
  }
}

// 创建任务并异步执行，立即返回任务 ID（进度经 SSE 推送）。
export async function createAndRunLongTask(goal: string, token: string): Promise<string> {
  const task = (await createTaskBE(token, goal)) as any;
  const id: string = task.id;
  // 异步执行，不阻塞 HTTP 响应
  runLongTask(id, goal, token).catch((e) => {
    console.error("[task] run failed:", e);
    updateTaskBE(token, id, { status: "failed", error: String(e?.message || e) }).catch(() => {});
    emit(id, { type: "error", message: String(e?.message || e) });
  });
  return id;
}

async function runLongTask(id: string, goal: string, token: string) {
  await updateTaskBE(token, id, { status: "running" });
  emit(id, { type: "status", status: "running" });

  let plan: string[] = [];
  try {
    plan = await planSteps(goal);
  } catch (e) {
    console.warn("[task] planning failed, falling back to single step:", e);
    plan = [goal];
  }
  await updateTaskBE(token, id, { plan: JSON.stringify(plan) });
  emit(id, { type: "plan", plan });

  let progress = 0;
  let lastResult = "";
  for (const step of plan) {
    const status = await getStatus(token, id);
    if (status === "canceled") {
      await updateTaskBE(token, id, { status: "canceled" });
      emit(id, { type: "status", status: "canceled" });
      return;
    }
    emit(id, { type: "step_start", step, progress });
    const stepResult = await runStep(goal, plan, progress, step, token, id);
    lastResult = stepResult;
    progress++;
    await updateTaskBE(token, id, { progress, result: lastResult });
    emit(id, { type: "step_done", step, progress, result: stepResult });
    if (progress >= MAX_STEPS) break;
  }

  await updateTaskBE(token, id, { status: "done", result: lastResult });
  emit(id, { type: "done", result: lastResult });
}

// 用 LLM 把目标拆成有序步骤（JSON 数组）。
async function planSteps(goal: string): Promise<string[]> {
  const model = createModel();
  const prompt = [
    "你是一个任务规划器。请把下面的目标拆成不超过 8 个、可独立执行的有序步骤。",
    "只输出 JSON 数组（字符串数组），不要解释。例如：[\"步骤1\",\"步骤2\"]。",
    `目标：${goal}`,
  ].join("\n");
  const res = await model.invoke(prompt);
  const text = typeof res.content === "string" ? res.content : "";
  const match = text.match(/\[[\s\S]*\]/);
  if (!match) return [goal];
  try {
    const arr = JSON.parse(match[0]);
    if (Array.isArray(arr) && arr.length > 0) {
      return arr.map((s) => String(s)).slice(0, 8);
    }
  } catch {
    /* fall through */
  }
  return [goal];
}

// 单步执行：在 supervisor 上下文中运行，流式推送 token。
async function runStep(
  goal: string,
  plan: string[],
  progress: number,
  step: string,
  token: string,
  id: string,
): Promise<string> {
  const done = plan
    .slice(0, progress)
    .map((s, i) => `${i + 1}. ${s}`)
    .join("\n");
  const ctxMsg =
    `整体目标：${goal}\n` +
    `已完成步骤：\n${done || "（无）"}\n` +
    `当前步骤（第 ${progress + 1} 步 / 共 ${plan.length} 步）：${step}\n` +
    `请执行当前步骤：必要时委派给子 agent 或使用工具，并给出本步骤的结论。`;
  const eventStream = supervisorExecutor.streamEvents(
    { messages: [new HumanMessage(ctxMsg)] },
    { version: "v2" },
  );
  let answer = "";
  await agentContext.run("supervisor", async () => {
    for await (const ev of eventStream) {
      if (ev.event === "on_chat_model_stream") {
        const t = textContent(ev.data?.chunk?.content);
        if (t) emit(id, { type: "token", content: t });
      } else if (ev.event === "on_chat_model_end") {
        const c = textContent(ev.data?.output?.content);
        if (c) answer = c;
      }
    }
  });
  return answer || "(无结论)";
}

async function getStatus(token: string, id: string): Promise<string> {
  try {
    const t = (await getTaskRaw(token, id)) as any;
    return t?.status || "queued";
  } catch {
    return "queued";
  }
}

function textContent(content: unknown): string {
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content.map((p: any) => (typeof p === "string" ? p : p?.text || "")).join("");
  }
  if (content == null) return "";
  return String(content);
}
