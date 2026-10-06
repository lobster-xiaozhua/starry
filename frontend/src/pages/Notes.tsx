import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { message, Modal, Input, Spin, Button } from 'antd'
import { Plus, Search, FileDown, Sparkles } from 'lucide-react'
import {
  listNotes,
  createNote,
  deleteNote,
  setArchived as setNoteArchived,
  exportMarkdown,
  exportAll,
  getNote,
  seedDemoNotes,
} from '../api/notes'
import { useSSE } from '../api/useSSE'
import { NoteItem, useDebounce, SSEEvent } from '../types'
import { downloadBlob } from '../utils'
import { NoteCard } from '../components/NoteCard'
import Sidebar from '../components/Sidebar'

const PAGE_SIZE = 20

export default function NotesPage() {
  const navigate = useNavigate()
  const [notes, setNotes] = useState<NoteItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [q, setQ] = useState('')
  const debouncedQ = useDebounce(q, 300)
  const [tags, setTags] = useState<string[]>([])
  const [archived, setArchived] = useState(false)
  const [loading, setLoading] = useState(false)
  const [newTitle, setNewTitle] = useState('')
  const [showNew, setShowNew] = useState(false)
  const [tagVersion, setTagVersion] = useState(0)
  const [seeding, setSeeding] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await listNotes({
        page,
        size: PAGE_SIZE,
        q: debouncedQ,
        tag: tags,
        archived: archived || undefined,
      })
      setNotes(data.notes)
      setTotal(data.total)
    } catch {
      message.error('加载失败')
    }
    setLoading(false)
  }, [page, debouncedQ, tags, archived])

  // 保存最新 load 引用，使乐观操作的错误回滚回调保持稳定（NoteCard memo 生效前提）
  const loadRef = useRef(load)
  useEffect(() => {
    loadRef.current = load
  }, [load])

  useEffect(() => {
    load()
  }, [load])

  // 拉取单条笔记并就地替换/增删，替代全量列表请求
  const refreshNote = useCallback(
    async (id: string) => {
      try {
        const n = await getNote(id)
        setNotes((prev) => {
          const idx = prev.findIndex((x) => x.id === id)
          // 归档状态与当前视图不符 → 从列表移除
          if (n.archived !== archived) {
            if (idx >= 0) {
              const next = [...prev]
              next.splice(idx, 1)
              return next
            }
            return prev
          }
          if (idx >= 0) {
            const next = [...prev]
            next[idx] = n
            return next
          }
          // 不在列表但符合当前视图 → 前置插入（created）
          return [n, ...prev]
        })
      } catch {
        // 笔记不属于当前视图或已被删除，忽略
      }
    },
    [archived],
  )

  // 精准事件处理：按事件类型定点更新列表，不再每条事件全量 reload
  // refreshNote 依赖 archived，但 useSSE 通过 ref 读取最新 listener，不触发重连
  const onSSEEvent = useCallback(
    (ev: SSEEvent) => {
      // 标签计数可能变化的事件 → 递增 tagVersion 触发 Sidebar 刷新
      if (
        ev.type === 'note.created' ||
        ev.type === 'note.deleted' ||
        ev.type === 'note.updated'
      ) {
        setTagVersion((v) => v + 1)
      }
      switch (ev.type) {
        case 'note.deleted':
          setNotes((prev) => prev.filter((n) => n.id !== ev.noteId))
          setTotal((t) => Math.max(0, t - 1))
          break
        case 'note.created':
        case 'note.updated':
        case 'note.archived':
        case 'note.unarchived':
          void refreshNote(ev.noteId)
          break
        default:
          break
      }
    },
    [refreshNote],
  )

  useSSE(onSSEEvent)

  const handleCreate = async () => {
    if (!newTitle.trim()) return
    try {
      const note = await createNote({ title: newTitle.trim(), body: '', tags: [] })
      setNewTitle('')
      setShowNew(false)
      navigate(`/notes/${note.id}`)
    } catch {
      message.error('创建失败')
    }
  }

  // 乐观归档：立即切换状态，失败才回滚
  const handleArchive = useCallback(async (note: NoteItem) => {
    const target = !note.archived
    setNotes((prev) =>
      prev.map((n) => (n.id === note.id ? { ...n, archived: target } : n)),
    )
    try {
      await setNoteArchived(note.id, target)
    } catch {
      message.error('归档操作失败')
      loadRef.current()
    }
  }, [])

  const handleExportMd = useCallback(async (noteId: string) => {
    try {
      const blob = await exportMarkdown(noteId)
      downloadBlob(blob, 'note.md')
    } catch {
      message.error('导出失败')
    }
  }, [])

  // 乐观删除：立即移除，失败才回滚
  const handleDelete = useCallback((note: NoteItem) => {
    Modal.confirm({
      title: '删除此笔记？',
      okText: '删除',
      okType: 'danger',
      onOk: async () => {
        setNotes((prev) => prev.filter((n) => n.id !== note.id))
        setTotal((t) => Math.max(0, t - 1))
        try {
          await deleteNote(note.id)
        } catch {
          message.error('删除失败，已恢复列表')
          loadRef.current()
        }
      },
    })
  }, [])

  const handleExportAll = useCallback(async () => {
    try {
      const blob = await exportAll()
      downloadBlob(blob, 'notes-export.json')
    } catch {
      message.error('导出失败')
    }
  }, [])

  const handleOpen = useCallback((id: string) => {
    navigate(`/notes/${id}`)
  }, [navigate])

  // 把笔记交给 AI 助手：组装总结提示词并跳转到工作模式（输入已预填）。
  const handleAskAi = useCallback((note: NoteItem) => {
    const prompt = `请总结并提炼以下笔记的要点，用结构化列表呈现：\n\n# ${note.title}\n\n${note.body}`
    navigate('/agent', { state: { draft: prompt } })
  }, [navigate])

  const totalPages = Math.ceil(total / PAGE_SIZE)

  // 一键灌入示例笔记，便于新用户快速体验（后端幂等：已有笔记则不重复灌）
  const handleSeedDemo = useCallback(async () => {
    setSeeding(true)
    try {
      const res = await seedDemoNotes()
      if (res.count > 0) {
        message.success(`已加载 ${res.count} 条示例笔记`)
        setPage(1)
        loadRef.current()
      } else {
        message.info('你已有笔记，未重复灌入示例')
      }
    } catch {
      message.error('加载示例失败')
    }
    setSeeding(false)
  }, [])

  return (
    <div className="app-shell">
      <Sidebar
        selectedTags={tags}
        tagVersion={tagVersion}
        onToggleTag={(t) => {
          setTags((prev) => (prev.includes(t) ? prev.filter((x) => x !== t) : [...prev, t]))
          setPage(1)
        }}
        onShowArchived={() => {
          setArchived((a) => !a)
          setPage(1)
        }}
        showArchived={archived}
      />
      <main className="main">
        <div className="main-header">
          <Input
            allowClear
            prefix={<Search size={14} style={{ color: 'var(--text-secondary)' }} />}
            placeholder="搜索笔记标题、内容、标签..."
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              setPage(1)
            }}
            style={{ flex: 1, maxWidth: 420 }}
          />
          <div style={{ display: 'flex', gap: 8 }}>
            <Button icon={<FileDown size={14} />} onClick={handleExportAll} size="small">
              导出
            </Button>
            <Button type="primary" icon={<Plus size={14} />} onClick={() => setShowNew(true)} size="small">
              新建
            </Button>
          </div>
        </div>
        <div className="main-content">
          {loading ? (
            <div style={{ display: 'grid', placeItems: 'center', height: 200 }}>
              <Spin />
            </div>
          ) : notes.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '60px 0', color: 'var(--text-secondary)' }}>
              {q ? (
                '未找到匹配的笔记'
              ) : (
                <>
                  <div style={{ marginBottom: 16 }}>暂无笔记，点击新建或加载示例快速体验</div>
                  <Button
                    icon={<Sparkles size={14} />}
                    loading={seeding}
                    onClick={handleSeedDemo}
                  >
                    加载示例笔记
                  </Button>
                </>
              )}
            </div>
          ) : (
            <div className="note-list">
              {notes.map((n) => (
                <NoteCard
                  key={n.id}
                  note={n}
                  onOpen={handleOpen}
                  onArchive={handleArchive}
                  onExport={handleExportMd}
                  onDelete={handleDelete}
                  onAskAi={handleAskAi}
                />
              ))}
            </div>
          )}
          {total > 0 && (
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                marginTop: 20,
                fontSize: 13,
                color: 'var(--text-secondary)',
              }}
            >
              <Button size="small" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                上一页
              </Button>
              <span>
                第 {page} 页 / 共 {totalPages} 页
              </span>
              <Button size="small" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
                下一页
              </Button>
            </div>
          )}
        </div>
      </main>
      {showNew && (
        <Modal
          title="新建笔记"
          open={showNew}
          onOk={handleCreate}
          onCancel={() => setShowNew(false)}
          okText="创建"
        >
          <Input
            autoFocus
            placeholder="笔记标题"
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            onPressEnter={handleCreate}
          />
        </Modal>
      )}
    </div>
  )
}
