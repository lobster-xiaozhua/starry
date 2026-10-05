import { AsyncLocalStorage } from "node:async_hooks";

export interface RequestContext {
  token: string;
  userId: string;
  role: string;
}

// AsyncLocalStorage 在整个异步调用链中保持 per-request 的用户身份，
// 使 LangGraph 工具无需显式传参即可拿到当前用户的 JWT token。
export const requestContext = new AsyncLocalStorage<RequestContext>();
