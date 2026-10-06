import { tool } from "@langchain/core/tools";
import { z } from "zod";
import { currentAgent } from "../context.js";
import { sendToAgent, readMailbox } from "../bus.js";

// 向另一个 agent 投递消息（agent 互联）。接收方可在其回合开始时通过 read_messages 收取。
export const messageAgentTool = tool(
  async ({ to, message }) => {
    const from = currentAgent();
    if (to === from) return "不能向自己投递消息。";
    const m = await sendToAgent(to, from, message);
    return `已向 ${to} 投递消息（来自 ${m.from}）。对方将在下一回合读取。`;
  },
  {
    name: "message_agent",
    description:
      "向团队中的另一个 agent（supervisor/researcher/writer/coder/analyst）异步投递一条消息，实现 agent 之间的协作与通讯。",
    schema: z.object({
      to: z.string().describe("目标 agent 名称，如 researcher / writer / coder / analyst / supervisor"),
      message: z.string().describe("要传递的消息内容"),
    }),
  },
);

// 读取本 agent 信箱中他人投递的消息。
export const readMessagesTool = tool(
  async () => {
    const me = currentAgent();
    const msgs = await readMailbox(me);
    if (msgs.length === 0) return "信箱为空，没有待处理的消息。";
    return msgs
      .map((m) => `【来自 ${m.from}】${m.message}`)
      .join("\n\n");
  },
  {
    name: "read_messages",
    description: "读取并清空本 agent 的信箱，查看其他 agent 投递的协作消息。",
    schema: z.object({}),
  },
);
