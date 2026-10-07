import { useEffect, useRef, useState, useCallback } from 'react'
import { SSEEvent } from '../lib/types.ts'
import { sseSession } from './sse-session.ts'

type Listener = (ev: SSEEvent) => void

/**
 * 订阅笔记 SSE 事件流。
 *
 * 设计要点：
 * - onEvent 通过 ref 读取，EventSource 只在挂载时创建一次，
 *   避免 onEvent 随父组件 state 变化而导致反复重连。
 * - 以单调 seq 做去重与断线回放：重连时发送 since=<lastSeq>，
 *   服务端回放 seq > lastSeq 的事件。
 * - 接收 `session` 事件后将会话 ID 写入 sseSession，供 REST 请求
 *   携带 X-Session-Id 头，使 broker 跳过来源会话、避免回环。
 *
 * 注：token 经 URL 传递为已知安全隐患，后续将在 Phase 0 改为一次性 ticket。
 */
export function useSSE(onEvent: Listener) {
  const esRef = useRef<EventSource | null>(null)
  const lastSeqRef = useRef<number>(0)
  const onEventRef = useRef(onEvent)
  const [connected, setConnected] = useState(false)

  // 始终使用最新的 listener，但不触发 EventSource 重建
  useEffect(() => {
    onEventRef.current = onEvent
  }, [onEvent])

  const ensureEventSource = useCallback(() => {
    if (esRef.current) esRef.current.close()
    const since = lastSeqRef.current > 0 ? `&since=${lastSeqRef.current}` : ''
    const token = localStorage.getItem('accessToken') ?? ''
    const es = new EventSource(`/api/notes/events?token=${token}${since}`)
    esRef.current = es

    const handleNote = (ev: MessageEvent<string>) => {
      const data = JSON.parse(ev.data) as SSEEvent
      if (data.seq && data.seq > lastSeqRef.current) {
        lastSeqRef.current = data.seq
      }
      onEventRef.current(data)
    }
    const handleSession = (ev: MessageEvent<string>) => {
      const data = JSON.parse(ev.data) as SSEEvent
      if (data.sourceSessionId) sseSession.set(data.sourceSessionId)
    }

    es.addEventListener('replay', handleNote)
    es.addEventListener('note', handleNote)
    es.addEventListener('session', handleSession)
    es.onopen = () => setConnected(true)
    es.onerror = () => {
      setConnected(false)
      es.close()
      esRef.current = null
      setTimeout(() => {
        if (!esRef.current || esRef.current.readyState === EventSource.CLOSED) {
          ensureEventSource()
        }
      }, 3000)
    }
  }, [])

  useEffect(() => {
    ensureEventSource()
    return () => {
      esRef.current?.close()
      esRef.current = null
    }
  }, [ensureEventSource])

  return { connected, reconnect: ensureEventSource }
}
