import { extractError } from '../api/client.ts'

/**
 * 把异常转成可直接展示给用户的一句话。
 *
 * 规则：只有「服务端故障/网络失败」才附带追踪码——参数校验类错误由用户自己修正，
 * 挂一串追踪码只会干扰阅读；而 5xx 需要用户能把它报给运维去查日志。
 * 追踪码来自响应头 X-Request-ID，与后端访问日志一一对应。
 */
export function formatError(err: unknown, fallback = '操作失败'): string {
  const { message, requestId, status } = extractError(err)
  const text = message || fallback
  const severe = status === undefined || status === 0 || status >= 500
  if (severe && requestId) {
    return `${text}（追踪码 ${requestId}）`
  }
  return text
}
