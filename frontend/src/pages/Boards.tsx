import { useCallback, useEffect, useState } from 'react'
import {
  App,
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Tag,
} from 'antd'
import { Plus, Trash2, KanbanSquare } from 'lucide-react'
import {
  createBoard,
  createColumn,
  createTask,
  deleteBoard,
  deleteColumn,
  deleteTask,
  listBoards,
  updateBoard,
  updateColumn,
  updateTask,
} from '../api/boards'
import type { BoardTask, BoardTaskPriority, BoardView } from '../types'

const PRIORITY_META: Record<BoardTaskPriority, { color: string; text: string }> = {
  low: { color: 'default', text: '低' },
  medium: { color: 'gold', text: '中' },
  high: { color: 'red', text: '高' },
}

type TaskDraft = {
  id?: string
  columnId: string
  title: string
  note: string
  priority: BoardTaskPriority
  due: string
}

const emptyDraft = (columnId: string): TaskDraft => ({
  columnId,
  title: '',
  note: '',
  priority: 'medium',
  due: '',
})

export default function BoardsPage() {
  const { message } = App.useApp()
  const [boards, setBoards] = useState<BoardView[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [creatingBoard, setCreatingBoard] = useState(false)
  const [newBoardName, setNewBoardName] = useState('')

  const [modalOpen, setModalOpen] = useState(false)
  const [draft, setDraft] = useState<TaskDraft>(emptyDraft(''))
  const [savingTask, setSavingTask] = useState(false)

  const [dragTaskId, setDragTaskId] = useState<string | null>(null)

  const reload = useCallback(async () => {
    try {
      const data = await listBoards()
      setBoards(data)
      setSelectedId((prev) => {
        if (prev && data.some((b) => b.id === prev)) return prev
        return data[0]?.id ?? null
      })
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '加载看板失败')
    } finally {
      setLoading(false)
    }
  }, [message])

  useEffect(() => {
    reload()
  }, [reload])

  const selected = boards.find((b) => b.id === selectedId) ?? null

  const handleCreateBoard = async () => {
    const name = newBoardName.trim()
    if (!name) {
      message.warning('请输入看板名称')
      return
    }
    try {
      await createBoard({ name })
      setNewBoardName('')
      setCreatingBoard(false)
      await reload()
      message.success('看板已创建')
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '创建失败')
    }
  }

  const handleDeleteBoard = async (id: string) => {
    try {
      await deleteBoard(id)
      await reload()
      message.success('看板已删除')
    } catch {
      message.error('删除失败')
    }
  }

  const handleAddColumn = async () => {
    if (!selected) return
    const title = window.prompt('新列名称', '新列')
    if (!title || !title.trim()) return
    try {
      await createColumn(selected.id, title.trim())
      await reload()
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '新建列失败')
    }
  }

  const handleDeleteColumn = async (id: string) => {
    try {
      await deleteColumn(id)
      await reload()
    } catch {
      message.error('删除列失败')
    }
  }

  const openCreate = (columnId: string) => {
    setDraft(emptyDraft(columnId))
    setModalOpen(true)
  }
  const openEdit = (t: BoardTask) => {
    setDraft({
      id: t.id,
      columnId: t.columnId,
      title: t.title,
      note: t.note,
      priority: t.priority,
      due: t.dueDate ? t.dueDate.slice(0, 10) : '',
    })
    setModalOpen(true)
  }

  const handleSaveTask = async () => {
    if (!selected) return
    if (!draft.title.trim()) {
      message.warning('请输入任务标题')
      return
    }
    setSavingTask(true)
    try {
      if (draft.id) {
        await updateTask(draft.id, {
          title: draft.title,
          note: draft.note,
          priority: draft.priority,
          columnId: draft.columnId,
          due: draft.due || '',
        })
      } else {
        await createTask(selected.id, {
          title: draft.title,
          columnId: draft.columnId,
          note: draft.note,
          priority: draft.priority,
          due: draft.due || '',
        })
      }
      setModalOpen(false)
      await reload()
      message.success('已保存')
    } catch (e: any) {
      message.error(e?.response?.data?.message || e?.message || '保存失败')
    } finally {
      setSavingTask(false)
    }
  }

  const handleDeleteTask = async (id: string) => {
    try {
      await deleteTask(id)
      await reload()
      message.success('已删除')
    } catch {
      message.error('删除失败')
    }
  }

  const handleMoveTask = async (taskId: string, toColumnId: string) => {
    try {
      await updateTask(taskId, { columnId: toColumnId })
      await reload()
    } catch {
      message.error('移动失败')
    }
  }

  if (loading) {
    return (
      <div style={{ padding: 48, textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', height: '100%', overflow: 'hidden' }}>
      {/* 看板列表 */}
      <div
        className="theme-aware"
        style={{
          width: 220,
          borderRight: '1px solid var(--border)',
          padding: 12,
          flexShrink: 0,
          overflow: 'auto',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
          <strong style={{ color: 'var(--text)' }}>我的看板</strong>
          <Button size="small" icon={<Plus size={14} />} onClick={() => setCreatingBoard(true)} />
        </div>
        {boards.map((b) => (
          <div
            key={b.id}
            onClick={() => setSelectedId(b.id)}
            style={{
              padding: '8px 10px',
              borderRadius: 8,
              cursor: 'pointer',
              marginBottom: 6,
              background: b.id === selectedId ? 'var(--accent-bg)' : 'transparent',
              color: b.id === selectedId ? 'var(--accent)' : 'var(--text)',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}
          >
            <span style={{ width: 8, height: 8, borderRadius: 99, background: b.color }} />
            <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {b.name}
            </span>
            <Popconfirm title="删除该看板？" onConfirm={() => handleDeleteBoard(b.id)}>
              <Button size="small" type="text" danger icon={<Trash2 size={12} />} onClick={(e) => e.stopPropagation()} />
            </Popconfirm>
          </div>
        ))}
        {boards.length === 0 && (
          <div style={{ fontSize: 13, color: 'var(--text-secondary)' }}>还没有看板，点 + 新建</div>
        )}
      </div>

      {/* 看板主体 */}
      <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
        {!selected ? (
          <Empty description="选择或新建一个看板" style={{ marginTop: 80 }} />
        ) : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 14 }}>
              <span style={{ width: 12, height: 12, borderRadius: 99, background: selected.color }} />
              <h2 style={{ margin: 0, color: 'var(--text)' }}>{selected.name}</h2>
              <Button size="small" icon={<Plus size={14} />} onClick={handleAddColumn}>
                新建列
              </Button>
            </div>

            <div style={{ display: 'flex', gap: 14, alignItems: 'flex-start', minHeight: 200 }}>
              {selected.columns.map((col) => (
                <div
                  key={col.id}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={async (e) => {
                    e.preventDefault()
                    const tid = e.dataTransfer.getData('text/taskId') || dragTaskId
                    if (tid && tid !== '' && col.id !== draft.columnId) {
                      await handleMoveTask(tid, col.id)
                    }
                    setDragTaskId(null)
                  }}
                  style={{
                    width: 280,
                    flexShrink: 0,
                    background: 'var(--bg)',
                    border: '1px solid var(--border)',
                    borderRadius: 12,
                    padding: 10,
                  }}
                >
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      marginBottom: 10,
                    }}
                  >
                    <strong style={{ color: 'var(--text)' }}>
                      {col.title} <span style={{ color: 'var(--text-secondary)' }}>({col.tasks.length})</span>
                    </strong>
                    <Popconfirm title="删除该列及其任务？" onConfirm={() => handleDeleteColumn(col.id)}>
                      <Button size="small" type="text" danger icon={<Trash2 size={12} />} />
                    </Popconfirm>
                  </div>

                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    {col.tasks.map((t) => (
                      <Card
                        key={t.id}
                        size="small"
                        draggable
                        onDragStart={(e) => {
                          e.dataTransfer.setData('text/taskId', t.id)
                          setDragTaskId(t.id)
                        }}
                        style={{ cursor: 'grab', background: 'var(--card)', borderColor: 'var(--border)' }}
                        onClick={() => openEdit(t)}
                      >
                        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 }}>
                          <span style={{ color: 'var(--text)', fontWeight: 500 }}>{t.title}</span>
                          <Tag color={PRIORITY_META[t.priority]?.color}>
                            {PRIORITY_META[t.priority]?.text}
                          </Tag>
                        </div>
                        {t.dueDate && (
                          <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 4 }}>
                            截止 {t.dueDate.slice(0, 10)}
                          </div>
                        )}
                      </Card>
                    ))}
                    <Button
                      size="small"
                      block
                      icon={<Plus size={14} />}
                      onClick={() => openCreate(col.id)}
                      style={{ borderStyle: 'dashed' }}
                    >
                      添加任务
                    </Button>
                  </Space>
                </div>
              ))}
            </div>
          </>
        )}
      </div>

      {/* 新建看板弹窗 */}
      <Modal
        open={creatingBoard}
        title="新建看板"
        onOk={handleCreateBoard}
        onCancel={() => setCreatingBoard(false)}
        okText="创建"
        cancelText="取消"
      >
        <Input
          placeholder="看板名称，如 产品迭代、个人待办"
          value={newBoardName}
          onChange={(e) => setNewBoardName(e.target.value)}
          onPressEnter={handleCreateBoard}
          prefix={<KanbanSquare size={16} />}
        />
      </Modal>

      {/* 任务编辑弹窗 */}
      <Modal
        open={modalOpen}
        title={draft.id ? '编辑任务' : '新建任务'}
        onOk={handleSaveTask}
        onCancel={() => setModalOpen(false)}
        confirmLoading={savingTask}
        okText="保存"
        cancelText="取消"
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Input
            placeholder="任务标题"
            value={draft.title}
            onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
          />
          <Input.TextArea
            rows={3}
            placeholder="备注（可选）"
            value={draft.note}
            onChange={(e) => setDraft((d) => ({ ...d, note: e.target.value }))}
          />
          <Space>
            <Select
              value={draft.priority}
              onChange={(v) => setDraft((d) => ({ ...d, priority: v }))}
              style={{ width: 120 }}
              options={[
                { value: 'low', label: '低优先级' },
                { value: 'medium', label: '中优先级' },
                { value: 'high', label: '高优先级' },
              ]}
            />
            <Input
              type="date"
              style={{ width: 180 }}
              value={draft.due}
              onChange={(e) => setDraft((d) => ({ ...d, due: e.target.value }))}
            />
          </Space>
        </Space>
      </Modal>
    </div>
  )
}
