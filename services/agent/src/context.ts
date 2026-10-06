import { AsyncLocalStorage } from "node:async_hooks";

export interface RequestContext {
  token: string;
  userId: string;
  role: string;
}

// AsyncLocalStorage 在整个异步调用链中保持 per-request 的用户身份，
// 使 LangGraph 工具无需显式传参即可拿到当前用户的 JWT token。
export const requestContext = new AsyncLocalStorage<RequestContext>();

// agentContext 记录「当前正在执行的是哪个 agent」，供 message_agent / read_messages
// 工具确定信箱归属，实现 agent 之间的消息互联。
export const agentContext = new AsyncLocalStorage<string>();

export function currentAgent(): string {
  return agentContext.getStore() ?? "supervisor";
}
