// LLM 模型工厂。agent 编排与摘要流程共用同一模型配置。
// 集中在此，避免在 agent.ts / history.ts 各处重复配置。
import { ChatOpenAI } from "@langchain/openai";

export function createModel(): ChatOpenAI {
  return new ChatOpenAI({
    modelName: process.env.LLM_MODEL || "agnes-3.0-flash",
    openAIApiKey: process.env.LLM_API_KEY || "",
    configuration: {
      baseURL: process.env.LLM_BASE_URL || "https://apihub.agnes-ai.com/v1",
    },
    streaming: true,
    temperature: 0.7,
  });
}
