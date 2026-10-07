import { api, type ApiResponse } from '@shared/api'
import type { DriveFile } from '../types'

export type DriveList = {
  items: DriveFile[]
  used: number
  quota: number
  maxOne: number
}

export async function listDrive(parent?: string): Promise<DriveList> {
  const res = await api.get<ApiResponse<DriveList>>('/drive', {
    params: parent && parent !== 'root' ? { parent } : {},
  })
  return res.data.data
}

export async function createFolder(name: string, parentID?: string): Promise<void> {
  await api.post('/drive/folders', { name, parentID: parentID || undefined })
}

export async function uploadFile(file: File, parentID?: string): Promise<DriveFile> {
  const fd = new FormData()
  fd.append('file', file)
  if (parentID && parentID !== 'root') fd.append('parentID', parentID)
  const res = await api.post<ApiResponse<DriveFile>>('/drive/upload', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.data
}

export async function renameFile(id: string, name: string): Promise<void> {
  await api.patch(`/drive/${id}`, { name })
}

export async function deleteDrive(id: string): Promise<void> {
  await api.delete(`/drive/${id}`)
}

// 通过已鉴权的 api 实例拉取文件流并触发浏览器下载（避免裸导航丢失 Bearer）。
export async function downloadFile(id: string, filename: string): Promise<void> {
  const res = await api.get(`/drive/${id}/download`, { responseType: 'blob' })
  const blob = res.data as Blob
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
