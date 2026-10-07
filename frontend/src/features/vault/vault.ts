import { api, type ApiResponse } from '@shared/api'
import type { VaultItem, VaultType } from '../../shared/lib/types.ts'

export type VaultSalt = {
  salt: string
  verifier: string
  setup: boolean
}

export async function getVaultSalt(): Promise<VaultSalt> {
  const res = await api.get<ApiResponse<VaultSalt>>('/vault/salt')
  return res.data.data
}

export async function setupVault(verifier: string): Promise<void> {
  await api.post('/vault/setup', { verifier })
}

export async function listVaultItems(): Promise<VaultItem[]> {
  const res = await api.get<ApiResponse<{ items: VaultItem[] }>>('/vault/items')
  return res.data.data.items
}

export async function createVaultItem(payload: {
  title: string
  type: VaultType
  encrypted: string
}): Promise<VaultItem> {
  const res = await api.post<ApiResponse<VaultItem>>('/vault/items', payload)
  return res.data.data
}

export async function updateVaultItem(
  id: string,
  payload: { title?: string; encrypted?: string },
): Promise<void> {
  await api.patch(`/vault/items/${id}`, payload)
}

export async function deleteVaultItem(id: string): Promise<void> {
  await api.delete(`/vault/items/${id}`)
}
