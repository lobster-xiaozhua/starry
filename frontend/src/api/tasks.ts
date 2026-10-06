import { api, tokenStore, type ApiResponse } from './client'

export type TaskStatus = 'queued' | 'running' | 'done' | 'failed' | 'canceled'

export interface AgentTask {
  id: string
  goal: string
  status: TaskStatus
  plan?: string
  progress: number
  result?: string
  error?: string
  createdAt: string
  updatedAt: string
}

// 创建长程任务（Agent 服务会立即开始异步执行），返回任务 ID
export async function createTask(goal: string): Promise<string> {
  const res = await api.post<ApiResponse<{ id: string }>>('/agent/tasks', { goal })
  return res.data.data?.id ?? ''
}

export async function getTask(id: string): Promise<AgentTask> {
  const res = await api.get<ApiResponse<AgentTask>>(`/agent/tasks/${id}`)
  return res.data.data
}

export async function listTasks(): Promise<AgentTask[]> {
  const res = await api.get<ApiResponse<{ tasks: AgentTask[] }>>('/agent/tasks')
  return res.data.data?.tasks ?? []
}

export async function cancelTask(id: string): Promise<void> {
  await api.post(`/agent/tasks/${id}/cancel`)
}

// 任务进度 SSE 订阅。返回 AbortController 供取消订阅。
// 事件类型：status / plan / step_start / token / step_done / done / error
//
// 采用 fetch + ReadableStream 而非 EventSource：EventSource 无法携带
// Authorization 头，若把 JWT 放进 URL 查询串会泄漏到访问日志，属于安全反模式。
export function subscribeTaskProgress(
  id: string,
  onEvent: (type: string, data: any) => void,
  onError?: (err: Error) => void,
): AbortController {
  const controller = new AbortController()
  const token = tokenStore.access ?? ''

  fetch(`/api/agent/tasks/${id}/stream`, {
    headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
    signal: controller.signal,
  })
    .then(async (res) => {
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `HTTP ${res.status}`)
      }
      const reader = res.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const blocks = buffer.split('\n\n')
        buffer = blocks.pop() ?? ''
        for (const block of blocks) {
          const lines = block.split('\n')
          let eventType = 'message'
          let dataStr = ''
          for (const line of lines) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) dataStr += line.slice(6)
            else if (line.startsWith(':')) continue // 心跳注释
          }
          if (!dataStr) continue
          try {
            onEvent(eventType, JSON.parse(dataStr))
          } catch {
            /* 忽略畸形块 */
          }
        }
      }
    })
    .catch((err) => {
      if (err.name === 'AbortError') return
      onError?.(err)
    })

  return controller
}
