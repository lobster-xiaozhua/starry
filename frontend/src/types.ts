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

// ===== 任务与项目管理（看板） =====
export type BoardTaskPriority = 'low' | 'medium' | 'high'

export type BoardTask = {
  id: string
  boardId: string
  columnId: string
  title: string
  note: string
  priority: BoardTaskPriority
  dueDate?: string
  position: number
  createdAt: string
  updatedAt: string
}

export type BoardColumn = {
  id: string
  boardId: string
  title: string
  position: number
  tasks: BoardTask[]
}

export type Board = {
  id: string
  name: string
  color: string
  position: number
}

export type BoardView = Board & { columns: BoardColumn[] }

// ===== 网盘 / 文件管理 =====
export type DriveFile = {
  id: string
  parentId: string | null
  name: string
  mime: string
  size: number
  isDir: boolean
  createdAt: string
  updatedAt: string
}

// ===== 密码 / 密钥保险箱 =====
export type VaultType = 'password' | 'note' | 'card' | 'apikey'
export type VaultItem = {
  id: string
  title: string
  type: VaultType
  encrypted: string
  createdAt: string
  updatedAt: string
}
