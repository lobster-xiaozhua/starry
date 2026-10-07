import { api, type ApiResponse } from '@shared/api'
import type { BoardView, BoardTaskPriority } from '../types'

export async function listBoards(): Promise<BoardView[]> {
  const res = await api.get<ApiResponse<{ boards: BoardView[] }>>('/boards')
  return res.data.data.boards
}

export async function createBoard(payload: { name: string; color?: string }): Promise<void> {
  await api.post('/boards', payload)
}

export async function updateBoard(
  id: string,
  payload: { name?: string; color?: string; position?: number },
): Promise<void> {
  await api.patch(`/boards/${id}`, payload)
}

export async function deleteBoard(id: string): Promise<void> {
  await api.delete(`/boards/${id}`)
}

export async function createColumn(boardId: string, title: string): Promise<void> {
  await api.post(`/boards/${boardId}/columns`, { title })
}

export async function updateColumn(
  id: string,
  payload: { title?: string; position?: number },
): Promise<void> {
  await api.patch(`/columns/${id}`, payload)
}

export async function deleteColumn(id: string): Promise<void> {
  await api.delete(`/columns/${id}`)
}

export async function createTask(
  boardId: string,
  payload: {
    title: string
    columnId: string
    note?: string
    priority?: BoardTaskPriority
    due?: string
  },
): Promise<void> {
  await api.post(`/boards/${boardId}/tasks`, payload)
}

export async function updateTask(
  id: string,
  payload: {
    title?: string
    note?: string
    priority?: BoardTaskPriority
    columnId?: string
    position?: number
    due?: string
  },
): Promise<void> {
  await api.patch(`/tasks/${id}`, payload)
}

export async function deleteTask(id: string): Promise<void> {
  await api.delete(`/tasks/${id}`)
}
