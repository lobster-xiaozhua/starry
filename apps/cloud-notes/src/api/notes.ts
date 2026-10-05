import { api, ApiResponse } from '@shared/api'
import { Note, NoteItem, TagRow, ImportResult } from '../types'

export async function listNotes(params: {
  page?: number
  size?: number
  q?: string
  tag?: string[]
  archived?: boolean
}): Promise<{ notes: NoteItem[]; total: number }> {
  const res = await api.get<ApiResponse<{ notes: NoteItem[]; total: number }>>('/notes', {
    params: {
      page: params.page ?? 1,
      size: params.size ?? 50,
      q: params.q || undefined,
      tag: params.tag?.length ? params.tag.join(',') : undefined,
      archived: params.archived ? 'true' : undefined,
    },
  })
  return res.data.data
}

export async function getNote(id: string): Promise<NoteItem> {
  const res = await api.get<ApiResponse<NoteItem>>(`/notes/${id}`)
  return res.data.data
}

export async function createNote(payload: {
  title: string
  body: string
  tags: string[]
}): Promise<Note> {
  const res = await api.post<ApiResponse<Note>>('/notes', payload)
  return res.data.data
}

export async function updateNote(
  id: string,
  payload: { title?: string; body?: string; tags?: string[] },
): Promise<Note> {
  const res = await api.put<ApiResponse<Note>>(`/notes/${id}`, payload)
  return res.data.data
}

export async function deleteNote(id: string): Promise<void> {
  await api.delete(`/notes/${id}`)
}

export async function setArchived(id: string, archived: boolean): Promise<void> {
  await api.post(`/notes/${id}/archive`, { archived })
}

export async function listTags(): Promise<TagRow[]> {
  const res = await api.get<ApiResponse<TagRow[]>>('/notes/tags')
  return res.data.data
}

export async function uploadImage(
  file: File,
  noteId?: string,
): Promise<{ url: string; thumbUrl: string }> {
  const fd = new FormData()
  fd.append('file', file)
  if (noteId) fd.append('noteId', noteId)
  const res = await api.post<ApiResponse<{ url: string; thumbUrl: string }>>('/notes/upload', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.data
}

export async function exportAll(): Promise<Blob> {
  const res = await api.get('/notes/export/all', { responseType: 'blob' })
  return res.data as unknown as Blob
}

export async function exportMarkdown(id: string): Promise<Blob> {
  const res = await api.get(`/notes/${id}/export`, { responseType: 'blob' })
  return res.data as unknown as Blob
}

export async function importJSON(file: File): Promise<ImportResult> {
  const text = await file.text()
  const res = await api.post<ApiResponse<ImportResult>>('/notes/import', text, {
    headers: { 'Content-Type': 'application/json' },
  })
  return res.data.data
}

export async function importMarkdown(file: File): Promise<ImportResult> {
  const text = await file.text()
  const res = await api.post<ApiResponse<ImportResult>>('/notes/import', text, {
    headers: { 'Content-Type': 'text/markdown' },
  })
  return res.data.data
}
