import { api, tokenStore, type ApiResponse } from './client'

export interface Conversation {
  id: string
  title: string
  createdAt: string
  updatedAt: string
}

export interface AgentMessage {
  id: string
  conversationId: string
  role: string
  content: string
  toolCalls?: string
  toolCallId?: string
  createdAt: string
}

export async function listConversations(): Promise<Conversation[]> {
  const res = await api.get<ApiResponse<{ conversations: Conversation[] }>>('/agent/conversations')
  return res.data.data?.conversations ?? []
}

export async function createConversation(title?: string): Promise<Conversation> {
  const res = await api.post<ApiResponse<Conversation>>('/agent/conversations', { title })
  const conv = res.data.data
  if (!conv?.id) {
    throw new Error(res.data.message || '创建对话失败')
  }
  return conv
}

export async function deleteConversation(id: string): Promise<void> {
  await api.delete(`/agent/conversations/${id}`)
}

export async function renameConversation(id: string, title: string): Promise<void> {
  await api.patch(`/agent/conversations/${id}`, { title })
}

export async function listMessages(conversationId: string): Promise<AgentMessage[]> {
  const res = await api.get<ApiResponse<{ messages: AgentMessage[] }>>(
    `/agent/conversations/${conversationId}/messages`,
  )
  return res.data.data.messages
}

// SSE 事件类型
export interface AgentSSEEvent {
  event: string
  data: {
    id?: string
    content?: string
    name?: string
    args?: unknown
    result?: string
    message?: string
  }
}

/**
 * 流式调用 agent，通过 fetch + ReadableStream 解析 SSE。
 * 返回 AbortController 供取消请求。
 */
export function streamChat(
  conversationId: string,
  message: string,
  onEvent: (ev: AgentSSEEvent) => void,
  onError?: (err: Error) => void,
  onClose?: () => void,
  mode: 'daily' | 'work' = 'work',
): AbortController {
  const controller = new AbortController()
  const token = tokenStore.access ?? ''

 fetch('/api/agent/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ conversationId, message, mode }),
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

        // 按 SSE 双换行分割事件块
        const blocks = buffer.split('\n\n')
        buffer = blocks.pop() ?? ''

        for (const block of blocks) {
          const lines = block.split('\n')
          let eventType = 'message'
          let dataStr = ''
          for (const line of lines) {
            if (line.startsWith('event: ')) eventType = line.slice(7)
            else if (line.startsWith('data: ')) dataStr += line.slice(6)
          }
          if (!dataStr) continue
          try {
            const data = JSON.parse(dataStr)
            onEvent({ event: eventType, data })
          } catch {
            // skip malformed
          }
        }
      }
      onClose?.()
    })
    .catch((err) => {
      if (err.name === 'AbortError') return
      onError?.(err)
    })

  return controller
}
