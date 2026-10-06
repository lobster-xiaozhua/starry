// BackendClient 封装对 Go 后端 REST API 的调用，
// 携带用户 JWT token 进行鉴权。供 LangGraph 工具和对话持久化使用。

const BACKEND_URL = process.env.BACKEND_URL || "http://127.0.0.1:8080";

export async function backendFetch(
  path: string,
  token: string,
  init?: RequestInit,
): Promise<any> {
  const res = await fetch(`${BACKEND_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...(init?.headers ?? {}),
    },
  });
  const body: any = await res.json();
  if (!res.ok || body.code !== 0) {
    throw new Error(body.message || `backend error ${res.status}`);
  }
  return body.data;
}

// ===== 对话持久化 =====

export async function listConversations(token: string) {
  return backendFetch("/api/agent/conversations", token);
}

export async function createConversation(token: string, title?: string) {
  return backendFetch("/api/agent/conversations", token, {
    method: "POST",
    body: JSON.stringify({ title: title ?? "" }),
  });
}

export async function deleteConversation(token: string, id: string) {
  return backendFetch(`/api/agent/conversations/${id}`, token, {
    method: "DELETE",
  });
}

export async function listMessages(
  token: string,
  conversationId: string,
  opts?: { limit?: number; compress?: boolean },
) {
  const params = new URLSearchParams();
  if (opts?.limit) params.set("limit", String(opts.limit));
  if (opts?.compress !== undefined) params.set("compress", opts.compress ? "1" : "0");
  const qs = params.toString();
  const path = `/api/agent/conversations/${conversationId}/messages${qs ? "?" + qs : ""}`;
  return backendFetch(path, token);
}

export async function saveMessage(
  token: string,
  conversationId: string,
  msg: {
    role: string;
    content: string;
    toolCalls?: string;
    toolCallId?: string;
  },
) {
  return backendFetch(`/api/agent/conversations/${conversationId}/messages`, token, {
    method: "POST",
    body: JSON.stringify(msg),
  });
}

// 批量删除消息，供摘要流程清理已被替代的原始消息。
export async function deleteMessages(token: string, conversationId: string, ids: string[]) {
  return backendFetch(`/api/agent/conversations/${conversationId}/messages/delete`, token, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

// ===== 笔记工具 =====

export async function searchNotesRequest(token: string, query: string) {
  return backendFetch(
    `/api/notes?q=${encodeURIComponent(query)}&size=10`,
    token,
  );
}

export async function getNoteRequest(token: string, noteId: string) {
  return backendFetch(`/api/notes/${noteId}`, token);
}

export async function createNoteRequest(
  token: string,
  payload: { title: string; body: string; tags?: string[] },
) {
  return backendFetch("/api/notes", token, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function updateNoteRequest(
  token: string,
  noteId: string,
  payload: { title?: string; body?: string; tags?: string[] },
) {
  return backendFetch(`/api/notes/${noteId}`, token, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

export async function listTagsRequest(token: string) {
  return backendFetch("/api/notes/tags", token);
}

// 上报本轮 LLM token 用量到 Go/Redis。失败仅告警，不阻断主流程。
export async function recordUsage(
  token: string,
  payload: { promptTokens: number; completionTokens: number; model?: string },
) {
  return backendFetch(`/api/agent/usage`, token, {
    method: "POST",
    body: JSON.stringify(payload),
  }).catch((e) => {
    console.warn("[agent] recordUsage failed:", e);
  });
}

// ===== 长程任务持久化（后端 agent_tasks 表）=====

export async function createTask(token: string, goal: string) {
  return backendFetch(`/api/agent/tasks`, token, {
    method: "POST",
    body: JSON.stringify({ goal }),
  });
}

export async function getTaskRaw(token: string, id: string) {
  return backendFetch(`/api/agent/tasks/${id}`, token);
}

export async function updateTask(
  token: string,
  id: string,
  fields: { status?: string; plan?: string; progress?: number; result?: string; error?: string },
) {
  return backendFetch(`/api/agent/tasks/${id}`, token, {
    method: "PATCH",
    body: JSON.stringify(fields),
  });
}

export async function listTasks(token: string) {
  return backendFetch(`/api/agent/tasks`, token);
}

export async function cancelTask(token: string, id: string) {
  return backendFetch(`/api/agent/tasks/${id}/cancel`, token, { method: "POST" });
}
