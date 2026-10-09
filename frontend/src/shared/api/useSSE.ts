import { useEffect, useRef, useState, useCallback } from 'react'
import { fetchEventSource } from '@microsoft/fetch-event-source'
import { SSEEvent } from '../lib/types.ts'
import { sseSession } from './sse-session.ts'

type Listener = (ev: SSEEvent) => void

/**
 * Subscribe to the note SSE event stream.
 *
 * Design notes:
 * - onEvent is read via ref so the fetchEventSource connection is only
 *   created once on mount, avoiding reconnect churn from parent re-renders.
 * - Monotonic seq is used for dedup and replay: on reconnect we send since=<lastSeq>,
 *   and the server replays events with seq > lastSeq.
 * - token is sent via the Authorization header (fetchEventSource supports custom
 *   headers), not via URL query parameter, to avoid exposing credentials in logs/URLs.
 */
export function useSSE(onEvent: Listener) {
  const ctrlRef = useRef<AbortController | null>(null)
  const lastSeqRef = useRef<number>(0)
  const onEventRef = useRef(onEvent)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    onEventRef.current = onEvent
  }, [onEvent])

  const ensureEventSource = useCallback(async () => {
    if (ctrlRef.current) {
      ctrlRef.current.abort()
      ctrlRef.current = null
    }
    const since = lastSeqRef.current > 0 ? `?since=${lastSeqRef.current}` : ''
    const token = localStorage.getItem('accessToken') ?? ''
    const ctrl = new AbortController()
    ctrlRef.current = ctrl

    const handleNote = (data: SSEEvent) => {
      if (data.seq && data.seq > lastSeqRef.current) {
        lastSeqRef.current = data.seq
      }
      onEventRef.current(data)
    }

    try {
      await fetchEventSource(`/api/notes/events${since}`, {
        signal: ctrl.signal,
        headers: {
          Authorization: `Bearer ${token}`,
        },
        onopen: async (response) => {
          if (response.ok) {
            setConnected(true)
          } else {
            throw new Error(`SSE connection failed: ${response.status}`)
          }
        },
        onmessage: (ev) => {
          if (ev.event === 'replay' || ev.event === 'note') {
            const data = JSON.parse(ev.data) as SSEEvent
            handleNote(data)
          } else if (ev.event === 'session') {
            const data = JSON.parse(ev.data) as SSEEvent
            if (data.sourceSessionId) sseSession.set(data.sourceSessionId)
          }
        },
        onerror: () => {
          setConnected(false)
          if (ctrlRef.current === ctrl) {
            ctrlRef.current = null
          }
          setTimeout(() => {
            if (!ctrlRef.current) {
              ensureEventSource()
            }
          }, 3000)
          throw new Error('SSE error, will retry')
        },
      })
    } catch {
      if (ctrl.signal.aborted) return
      setConnected(false)
      if (ctrlRef.current === ctrl) {
        ctrlRef.current = null
      }
      setTimeout(() => {
        if (!ctrlRef.current) {
          ensureEventSource()
        }
      }, 3000)
    }
  }, [])

  useEffect(() => {
    ensureEventSource()
    return () => {
      ctrlRef.current?.abort()
      ctrlRef.current = null
    }
  }, [ensureEventSource])

  return { connected, reconnect: ensureEventSource }
}
