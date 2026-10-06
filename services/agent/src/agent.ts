import { createReactAgent } from "@langchain/langgraph/prebuilt";
import { allTools } from "./tools/index.js";
import { createModel } from "./llm.js";

const SYSTEM_PROMPT = [
  "你是「Starry AI 助手」，一个集成在笔记系统中的智能助手。你可以：",
  "",
  "1. 搜索和查看用户的笔记（search_notes, get_note）",
  "2. 创建新笔记（create_note）",
  "3. 更新已有笔记（update_note）",
  "4. 查看笔记标签（list_tags）",
  "5. 进行通用问答和对话",
  "",
  "当用户的问题涉及笔记内容时，主动使用工具检索。",
  "当用户要求创建或编辑笔记时，使用相应工具。",
  "对于通用知识问题，直接回答。始终使用中文回复，简洁友好。",
].join("\n");

const DAILY_SYSTEM_PROMPT = [
  "你是「Starry 日常 AI 对话」助手，负责通用闲聊、问答与建议。",
  "不调用任何外部工具，不操作笔记，直接基于对话上下文用中文简洁友好地回复。",
].join("\n");

// LangGraph 的返回类型包含内部 web 入口路径，显式收窄为运行时所需接口，
// 避免 declaration emit 时生成不可移植的绝对模块引用。
export const agentExecutor: {
  streamEvents: (...args: any[]) => AsyncIterable<any>;
} = createReactAgent({
  llm: createModel(),
  tools: allTools,
  messageModifier: SYSTEM_PROMPT,
});

// 日常对话模式：无工具的纯 LLM，沿用同一 streamEvents 接口以便 server 复用处理逻辑。
export const dailyExecutor: {
  streamEvents: (...args: any[]) => AsyncIterable<any>;
} = createReactAgent({
  llm: createModel(),
  tools: [],
  messageModifier: DAILY_SYSTEM_PROMPT,
});
