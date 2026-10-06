import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { HumanMessage, type BaseMessage } from "@langchain/core/messages";
import { tool } from "@langchain/core/tools";
import { z } from "zod";
import { createModel } from "./llm.js";
import { allTools } from "./tools/index.js";
import { agentContext } from "./context.js";
import { SUB_AGENTS, SUPERVISOR_SYSTEM, type SubAgentSpec } from "./config/agents.config.js";

const DAILY_SYSTEM_PROMPT = [
  "你是「Starry 日常 AI 对话」助手，负责通用闲聊、问答与建议。",
  "不调用任何外部工具，不操作笔记，直接基于对话上下文用中文简洁友好地回复。",
].join("\n");

// 构建单个子 agent（自有 system prompt + 工具白名单）。
function buildSubAgent(spec: SubAgentSpec) {
  return createReactAgent({
    llm: createModel(),
    tools: spec.tools,
    messageModifier: spec.system,
  });
}

// 为子 agent 生成「委派」工具：supervisor 调用它，即在子 agent 上下文中执行任务并取回结论。
function buildDelegates() {
  const built = SUB_AGENTS.map((spec) => ({ spec, agent: buildSubAgent(spec) }));
  const tools = built.map(({ spec, agent }) =>
    tool(
      async ({ task }: { task: string }) => {
        const res = await agentContext.run(spec.key, () =>
          agent.invoke({ messages: [new HumanMessage(task)] }),
        );
        return lastText(res.messages);
      },
      {
        name: `delegate_to_${spec.key}`,
        description: `委派任务给「${spec.name}」：${spec.description}`,
        schema: z.object({ task: z.string().describe("交给该子 agent 的具体任务描述") }),
      },
    ),
  );
  return tools;
}

// 主调度 agent：拥有全部通用工具 + 委派工具，统筹团队。
export const supervisorExecutor: {
  streamEvents: (...args: any[]) => AsyncIterable<any>;
} = createReactAgent({
  llm: createModel(),
  tools: [...allTools, ...buildDelegates()],
  messageModifier: SUPERVISOR_SYSTEM,
});

// 日常对话模式：无工具的纯 LLM。
export const dailyExecutor: {
  streamEvents: (...args: any[]) => AsyncIterable<any>;
} = createReactAgent({
  llm: createModel(),
  tools: [],
  messageModifier: DAILY_SYSTEM_PROMPT,
});

function lastText(messages: BaseMessage[]): string {
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (m._getType() === "ai") {
      const c = m.content;
      if (typeof c === "string") return c;
      if (Array.isArray(c)) return c.map((p: any) => p?.text || "").join("");
    }
  }
  return "(无输出)";
}
