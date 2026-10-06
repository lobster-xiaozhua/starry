import { api } from '@shared/api'

/**
 * 当前 SSE 会话 ID 的进程内持有器。
 *
 * useSSE 在收到服务端下发的 `session` 事件后调用 set()；
 * 下方的 axios 拦截器在所有非 GET 请求上附加 X-Session-Id 头，
 * 使后端 broker 能在扇出时跳过发起变更的那个会话，避免回环。
 */
let sessionId = ''

export const sseSession = {
  get(): string {
    return sessionId
  },
  set(id: string) {
    sessionId = id
  },
}

api.interceptors.request.use((config) => {
  if (sessionId && config.method && config.method.toUpperCase() !== 'GET') {
    config.headers['X-Session-Id'] = sessionId
  }
  return config
})
