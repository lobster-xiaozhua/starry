import { useEffect, useState } from 'react'

export type Note = {
  id: string
  title: string
  body: string
  archived: boolean
  createdAt: string
  updatedAt: string
  tags: string[]
}

export type NoteItem = {
  id: string
  title: string
  body: string
  archived: boolean
  updatedAt: string
  tags: string[]
}

export type SSEEvent = {
  noteId: string
  type: string
  ts: string
  sourceSessionId: string
  eventId: string
  seq: number
}

export type TagRow = { name: string; count: number }

export type ImportResult = { ok: number; skipped: number; errors: string[] }

export function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}
