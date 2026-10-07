import { useCallback, useEffect, useRef, useState } from 'react'
import {
  App,
  Button,
  Card,
  Empty,
  Input,
  List,
  Popconfirm,
  Progress,
  Space,
  Tabs,
  Tag,
} from 'antd'
import { Database, ListChecks, Search, Send, Trash2, Upload, X } from 'lucide-react'
import {
  deleteKnowledgeDoc,
  ingestKnowledge,
  listKnowledgeDocs,
  searchKnowledge,
  type KnowledgeDoc,
  type KnowledgeHit,
} from './knowledge.ts'
import {
  cancelTask,
  createTask,
  listTasks,
  subscribeTaskProgress,
  type AgentTask,
} from '../chat/tasks.ts'

const STATUS_META: Record<string, { color: string; text: string }> = {
  queued: { color: 'default', text: '排队中' },
  running: { color: 'processing', text: '执行中' },
  done: { color: 'success', text: '已完成' },
  failed: { color: 'error', text: '失败' },
  canceled: { color: 'warning', text: '已取消' },
}

/**
 * 知识库（RAG）与长程任务的工作台界面：
 * - 知识库：文本入库（本地模型向量化）、相似检索、文档列表与删除
 * - 长程任务：提交高层目标，实时订阅多步执行进度（SSE），可取消
 */
export default function KnowledgePage() {
  const { message } = App.useApp()

  // ===== 知识库状态 =====
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [ingesting, setIngesting] = useState(false)
  const [docs, setDocs] = useState<KnowledgeDoc[]>([])
  const [query, setQuery] = useState('')
  const [hits, setHits] = useState<KnowledgeHit[]>([])
  const [searching, setSearching] = useState(false)

  // ===== 长程任务状态 =====
  const [goal, setGoal] = useState('')
  const [tasks, setTasks] = useState<AgentTask[]>([])
  const [activeTask, setActiveTask] = useState<AgentTask | null>(null)
  const [log, setLog] = useState<string[]>([])
  const controllerRef = useRef<AbortController | null>(null)

  const reloadDocs = useCallback(async () => {
    try {
      setDocs(await listKnowledgeDocs())
    } catch {
      /* 忽略：空知识库或接口不可用 */
    }
  }, [])

  const reloadTasks = useCallback(async () => {
    try {
      setTasks(await listTasks())
    } catch {
      /* 忽略 */
    }
  }, [])

  useEffect(() => {
    reloadDocs()
    reloadTasks()
    return () => controllerRef.current?.abort()
  }, [reloadDocs, reloadTasks])

  // ===== 知识库操作 =====
  const handleIngest = async () => {
    const body = content.trim()
    if (!body) {
      message.warning('请输入要入库的正文内容')
      return
    }
    setIngesting(true)
    try {
      const res = await ingestKnowledge({ title: title.trim(), content: body, source: 'manual' })
      message.success(`已入库，生成 ${res.chunks} 个片段`)
      setTitle('')
      setContent('')
      reloadDocs()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '入库失败')
    } finally {
      setIngesting(false)
    }
  }

  const handleSearch = async () => {
    const q = query.trim()
    if (!q) {
      message.warning('请输入检索关键词')
      return
    }
    setSearching(true)
    try {
      setHits(await searchKnowledge(q, 5))
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '检索失败')
    } finally {
      setSearching(false)
    }
  }

  const handleDeleteDoc = async (id: string) => {
    try {
      await deleteKnowledgeDoc(id)
      message.success('已删除')
      reloadDocs()
    } catch {
      message.error('删除失败')
    }
  }

  // ===== 长程任务操作 =====
  const handleCreateTask = async () => {
    const g = goal.trim()
    if (!g) {
      message.warning('请输入任务目标')
      return
    }
    try {
      const id = await createTask(g)
      if (!id) throw new Error('未返回任务 ID')
      setGoal('')
      setLog([`任务已创建：${g}`])
      const controller = subscribeTaskProgress(
        id,
        (type, data) => {
          if (type === 'status') {
            const st = data?.status ?? 'queued'
            setActiveTask((prev) => ({ ...(prev ?? { id, goal: g, progress: 0 }), status: st } as AgentTask))
            setLog((prev) => [...prev, `状态：${STATUS_META[st]?.text ?? st}`])
          } else if (type === 'plan') {
            const plan: string[] = Array.isArray(data?.plan) ? data.plan : []
            setLog((prev) => [...prev, ...plan.map((s, i) => `计划 ${i + 1}. ${s}`)])
          } else if (type === 'step_start') {
            setLog((prev) => [...prev, `▶ ${data?.step ?? ''}`])
          } else if (type === 'step_done') {
            setActiveTask((prev) => (prev ? { ...prev, progress: data?.progress ?? 0 } : prev))
            setLog((prev) => [...prev, `✓ 完成：${data?.step ?? ''}`])
          } else if (type === 'done') {
            setActiveTask((prev) => (prev ? { ...prev, status: 'done', result: data?.result } : prev))
            setLog((prev) => [...prev, '全部完成', data?.result ?? ''])
          } else if (type === 'error') {
            setLog((prev) => [...prev, `错误：${data?.message ?? ''}`])
          }
        },
        (err) => message.error(`进度订阅失败：${err.message}`),
      )
      controllerRef.current = controller
      reloadTasks()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '创建任务失败')
    }
  }

  const handleCancelTask = async () => {
    if (!activeTask?.id) return
    try {
      await cancelTask(activeTask.id)
      message.success('已请求取消')
      controllerRef.current?.abort()
      reloadTasks()
    } catch {
      message.error('取消失败')
    }
  }

  const knowledgeTab = (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title={<Space><Upload size={16} />文档入库</Space>}>
        <Space direction="vertical" size={10} style={{ width: '100%' }}>
          <Input
            placeholder="文档标题（可留空，自动取正文前 40 字）"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <Input.TextArea
            rows={6}
            placeholder="粘贴要纳入知识库的文本内容（Markdown 亦可）"
            value={content}
            onChange={(e) => setContent(e.target.value)}
          />
          <Button type="primary" loading={ingesting} onClick={handleIngest}>
            入库（本地免费向量化）
          </Button>
        </Space>
      </Card>

      <Card title={<Space><Search size={16} />相似检索</Space>}>
        <Space.Compact style={{ width: '100%' }}>
          <Input
            placeholder="输入问题或关键词，从知识库中检索相关片段"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onPressEnter={handleSearch}
          />
          <Button type="primary" loading={searching} onClick={handleSearch}>
            检索
          </Button>
        </Space.Compact>
        {hits.length > 0 && (
          <List
            style={{ marginTop: 12 }}
            dataSource={hits}
            renderItem={(h, i) => (
              <List.Item>
                <List.Item.Meta
                  title={`片段 ${i + 1} · 文档 ${h.docId.slice(0, 8)}`}
                  description={<span style={{ whiteSpace: 'pre-wrap' }}>{h.content}</span>}
                />
              </List.Item>
            )}
          />
        )}
      </Card>

      <Card title={<Space><Database size={16} />已入库文档（{docs.length}）</Space>}>
        {docs.length === 0 ? (
          <Empty description="暂无文档，先在上方入库" />
        ) : (
          <List
            dataSource={docs}
            renderItem={(d) => (
              <List.Item
                actions={[
                  <Popconfirm key="del" title="确认删除该文档及其向量？" onConfirm={() => handleDeleteDoc(d.id)}>
                    <Button type="text" danger icon={<Trash2 size={14} />} />
                  </Popconfirm>,
                ]}
              >
                <List.Item.Meta
                  title={d.title}
                  description={`${d.chunkCount} 个片段 · ${new Date(d.createdAt).toLocaleString()}`}
                />
              </List.Item>
            )}
          />
        )}
      </Card>
    </Space>
  )

  const taskTab = (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card title={<Space><Send size={16} />提交长程任务</Space>}>
        <Space direction="vertical" size={10} style={{ width: '100%' }}>
          <Input.TextArea
            rows={3}
            placeholder="描述一个需要多步完成的目标，例如：调研 2026 年前端框架趋势并写入笔记"
            value={goal}
            onChange={(e) => setGoal(e.target.value)}
          />
          <Space>
            <Button type="primary" onClick={handleCreateTask}>
              提交并执行
            </Button>
            <Button
              danger
              icon={<X size={14} />}
              disabled={!activeTask || activeTask.status !== 'running'}
              onClick={handleCancelTask}
            >
              取消
            </Button>
          </Space>
        </Space>
      </Card>

      {activeTask && (
        <Card title="当前任务">
          <Space direction="vertical" size={8} style={{ width: '100%' }}>
            <Space>
              <Tag color={STATUS_META[activeTask.status]?.color}>
                {STATUS_META[activeTask.status]?.text ?? activeTask.status}
              </Tag>
              <span>{activeTask.goal}</span>
            </Space>
            <Progress percent={Math.min(100, (activeTask.progress ?? 0) * 10)} size="small" />
          </Space>
        </Card>
      )}

      <Card title="执行日志">
        {log.length === 0 ? (
          <Empty description="尚无日志" />
        ) : (
          <div style={{ maxHeight: 320, overflow: 'auto', whiteSpace: 'pre-wrap', fontSize: 13 }}>
            {log.map((l, i) => (
              <div key={i}>{l}</div>
            ))}
          </div>
        )}
      </Card>

      <Card title={<Space><ListChecks size={16} />历史任务</Space>}>
        {tasks.length === 0 ? (
          <Empty description="暂无任务" />
        ) : (
          <List
            dataSource={tasks}
            renderItem={(t) => (
              <List.Item>
                <List.Item.Meta
                  title={
                    <Space>
                      <Tag color={STATUS_META[t.status]?.color}>
                        {STATUS_META[t.status]?.text ?? t.status}
                      </Tag>
                      {t.goal}
                    </Space>
                  }
                  description={`进度 ${t.progress} · ${new Date(t.createdAt).toLocaleString()}`}
                />
              </List.Item>
            )}
          />
        )}
      </Card>
    </Space>
  )

  return (
    <div style={{ padding: 24, maxWidth: 900, margin: '0 auto' }}>
      <Tabs
        defaultActiveKey="kb"
        items={[
          { key: 'kb', label: '知识库 RAG', children: knowledgeTab },
          { key: 'task', label: '长程任务', children: taskTab },
        ]}
      />
    </div>
  )
}
