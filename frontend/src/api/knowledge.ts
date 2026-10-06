import { api, type ApiResponse } from './client'

export interface KnowledgeDoc {
  id: string
  title: string
  source?: string
  chunkCount: number
  createdAt: string
}

export interface KnowledgeHit {
  docId: string
  content: string
}

// 知识库入库：文本 → 分块 → 本地模型向量化 → pgvector 存储
export async function ingestKnowledge(payload: {
  title: string
  content: string
  source?: string
}): Promise<{ docId: string; chunks: number }> {
  const res = await api.post<ApiResponse<{ docId: string; chunks: number }>>(
    '/knowledge/ingest',
    payload,
  )
  return res.data.data
}

// 知识库相似检索（供 AI 的 knowledge_search 使用，也可供界面直接查询）
export async function searchKnowledge(q: string, k = 5): Promise<KnowledgeHit[]> {
  const res = await api.get<ApiResponse<{ results: KnowledgeHit[] }>>('/knowledge/search', {
    params: { q, k },
  })
  return res.data.data?.results ?? []
}

export async function listKnowledgeDocs(): Promise<KnowledgeDoc[]> {
  const res = await api.get<ApiResponse<{ docs: KnowledgeDoc[] }>>('/knowledge/docs')
  return res.data.data?.docs ?? []
}

export async function deleteKnowledgeDoc(id: string): Promise<void> {
  await api.delete(`/knowledge/docs/${id}`)
}
