import { useEffect, useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input, Spin, Modal, App } from 'antd'
import {
  ArrowLeft,
  Plus,
  Send,
  Trash2,
  Wrench,
  Bot,
  User,
} from 'lucide-react'
import {
  listConversations,
  createConversation,
  deleteConversation,
  listMessages,
  streamChat,
  type Conversation,
  type AgentMessage,
  type AgentSSEEvent,
} from '../api/agent'
import { marked } from 'marked'

interface DisplayMessage {
  id: string
  role: string
  content?: string
  toolName?: string
  toolArgs?: string
  toolResult?: string
  streaming?: boolean
}

export default function Chat() {
  const navigate = useNavigate()
  const { message: antdMessage } = App.useApp()
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [activeConv, setActiveConv] = useState<string | null>(null)
  const [messages, setMessages] = useState<DisplayMessage[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadingConv, setLoadingConv] = useState(false)
  const [showNew, setShowNew] = useState(false)
  const [newTitle, setNewTitle] = useState('')
  const abortRef = useRef<AbortController | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const skipNextLoadRef = useRef(false)

  const loadConversations = useCallback(async () => {
    try {
      const convs = await listConversations()
      setConversations(convs)
    } catch {
      antdMessage.error('加载对话列表失败')
    }
  }, [antdMessage])

  useEffect(() => {
    loadConversations()
  }, [loadConversations])

  const loadMessages = useCallback(async (convId: string) => {
    setLoadingConv(true)
    try {
      const msgs = await listMessages(convId)
      const display: DisplayMessage[] = []
      for (const m of msgs) {
        if (m.role === 'user') {
          display.push({ id: m.id, role: 'user', content: m.content })
        } else if (m.role === 'assistant') {
          let toolCalls: any[] | undefined
          if (m.toolCalls) {
            try { toolCalls = JSON.parse(m.toolCalls) } catch {}
          }
          display.push({ id: m.id, role: 'assistant', content: m.content })
          if (toolCalls && toolCalls.length > 0) {
            for (const tc of toolCalls) {
              display.push({
                id: m.id + '-tc-' + tc.id,
                role: 'tool',
                toolName: tc.name,
                toolArgs: JSON.stringify(tc.args, null, 2),
              })
            }
          }
        } else if (m.role === 'tool') {
          // attach result to the last tool message
          const last = display[display.length - 1]
          if (last && last.role === 'tool') {
            last.toolResult = m.content
          }
        }
      }
      setMessages(display)
    } catch {
      antdMessage.error('加载消息失败')
    }
    setLoadingConv(false)
  }, [antdMessage])

  useEffect(() => {
    if (activeConv && skipNextLoadRef.current) {
      skipNextLoadRef.current = false
      return
    }
    if (activeConv) loadMessages(activeConv)
    else setMessages([])
  }, [activeConv, loadMessages])

  useEffect(() => () => {
    abortRef.current?.abort()
  }, [])

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages])

  const handleCreate = async () => {
    try {
      const conv = await createConversation(newTitle || '新对话')
      setNewTitle('')
      setShowNew(false)
      setConversations((prev) => [conv, ...prev])
      setActiveConv(conv.id)
    } catch {
      antdMessage.error('创建对话失败')
    }
  }

  const handleDelete = (conv: Conversation) => {
    Modal.confirm({
      title: '删除此对话？',
      okText: '删除',
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteConversation(conv.id)
          setConversations((prev) => prev.filter((c) => c.id !== conv.id))
          if (activeConv === conv.id) {
            setActiveConv(null)
            setMessages([])
          }
        } catch {
          antdMessage.error('删除失败')
        }
      },
    })
  }

  const handleSend = async () => {
    if (!input.trim() || loading) return
    let convId = activeConv
    if (!convId) {
      const conv = await createConversation('新对话')
      setConversations((prev) => [conv, ...prev])
      skipNextLoadRef.current = true
      setActiveConv(conv.id)
      convId = conv.id
    }
    const userMsg = input.trim()
    setInput('')

    // 乐观添加用户消息
    const userDisplayId = Date.now() + '-user'
    const assistantId = Date.now() + '-assistant'
    setMessages((prev) => [
      ...prev,
      { id: userDisplayId, role: 'user', content: userMsg },
      { id: assistantId, role: 'assistant', content: '', streaming: true },
    ])
    setLoading(true)

    let assistantContent = ''

    const controller = streamChat(
      convId,
      userMsg,
      (ev: AgentSSEEvent) => {
        if (ev.event === 'token') {
          assistantContent += ev.data.content || ''
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, content: assistantContent } : m,
            ),
          )
        } else if (ev.event === 'tool_call') {
          setMessages((prev) => [
            ...prev.slice(0, -1),
            {
              id: Date.now() + '-tc',
              role: 'tool',
              toolName: ev.data.name,
              toolArgs: JSON.stringify(ev.data.args, null, 2),
            },
            { id: assistantId, role: 'assistant', content: assistantContent, streaming: true },
          ])
        } else if (ev.event === 'tool_result') {
          setMessages((prev) => {
            const idx = prev.findIndex((m) => m.role === 'tool' && m.toolName === ev.data.name && !m.toolResult)
            if (idx >= 0) {
              const next = [...prev]
              next[idx] = { ...next[idx], toolResult: ev.data.result }
              return next
            }
            return prev
          })
        } else if (ev.event === 'done') {
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, content: ev.data.content || assistantContent, streaming: false } : m,
            ),
          )
        } else if (ev.event === 'error') {
          antdMessage.error(ev.data.message || 'Agent 错误')
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, streaming: false, content: m.content || '[出错了]' } : m,
            ),
          )
        }
      },
      (err) => {
        setLoading(false)
        antdMessage.error(err.message)
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantId ? { ...m, streaming: false, content: m.content || '[网络错误]' } : m,
          ),
        )
      },
      () => {
        setLoading(false)
        loadConversations()
      },
    )
    abortRef.current = controller
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', background: '#f8fafc' }}>
      <div className="app-topbar" style={{ background: '#0f172a' }}>
        <Button
          type="text"
          icon={<ArrowLeft size={16} />}
          onClick={() => navigate('/')}
          style={{ color: '#f1f5f9' }}
        >
          返回
        </Button>
        <div style={{ color: '#f1f5f9', fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
          <Bot size={18} />
          AI 助手
        </div>
      </div>
      <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
        {/* 对话列表 */}
        <aside style={{ width: 260, borderRight: '1px solid #e2e8f0', display: 'flex', flexDirection: 'column', background: '#fff' }}>
          <div style={{ padding: 12, borderBottom: '1px solid #e2e8f0' }}>
            <Button type="primary" icon={<Plus size={14} />} block onClick={() => setShowNew(true)}>
              新建对话
            </Button>
          </div>
          <div style={{ flex: 1, overflow: 'auto' }}>
            {conversations.map((conv) => (
              <div
                key={conv.id}
                onClick={() => setActiveConv(conv.id)}
                style={{
                  padding: '10px 12px',
                  cursor: 'pointer',
                  background: activeConv === conv.id ? '#f1f5f9' : 'transparent',
                  borderBottom: '1px solid #f1f5f9',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  fontSize: 13,
                  color: '#0f172a',
                  fontWeight: activeConv === conv.id ? 600 : 400,
                }}
              >
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>
                  {conv.title}
                </span>
                <Trash2
                  size={14}
                  style={{ color: '#94a3b8', flexShrink: 0 }}
                  onClick={(e) => { e.stopPropagation(); handleDelete(conv) }}
                />
              </div>
            ))}
            {conversations.length === 0 && (
              <div style={{ padding: 16, fontSize: 13, color: '#94a3b8', textAlign: 'center' }}>
                暂无对话
              </div>
            )}
          </div>
        </aside>

        {/* 消息区 */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
          <div ref={scrollRef} style={{ flex: 1, overflow: 'auto', padding: '20px 24px' }}>
            {!activeConv ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: 12, color: '#94a3b8' }}>
                <Bot size={48} />
                <span style={{ fontSize: 16 }}>选择一个对话或新建对话开始</span>
              </div>
            ) : loadingConv ? (
              <div style={{ display: 'grid', placeItems: 'center', height: '100%' }}>
                <Spin />
              </div>
            ) : (
              messages.map((m) => {
                if (m.role === 'tool') {
                  return (
                    <div key={m.id} style={{ display: 'flex', justifyContent: 'center', margin: '8px 0' }}>
                      <div style={{
                        background: '#e2e8f0',
                        borderRadius: 8,
                        padding: '6px 12px',
                        fontSize: 12,
                        color: '#475569',
                        display: 'flex',
                        alignItems: 'center',
                        gap: 6,
                        maxWidth: 600,
                      }}>
                        <Wrench size={12} />
                        <span>{m.toolName}</span>
                        {m.toolResult && (
                          <details>
                            <summary style={{ cursor: 'pointer', fontSize: 11 }}>结果</summary>
                            <pre style={{ fontSize: 11, maxWidth: 400, overflow: 'auto', marginTop: 4 }}>{m.toolResult}</pre>
                          </details>
                        )}
                      </div>
                    </div>
                  )
                }
                const isUser = m.role === 'user'
                return (
                  <div key={m.id} style={{
                    display: 'flex',
                    justifyContent: isUser ? 'flex-end' : 'flex-start',
                    marginBottom: 16,
                  }}>
                    <div style={{
                      display: 'flex',
                      gap: 8,
                      maxWidth: '75%',
                      flexDirection: isUser ? 'row-reverse' : 'row',
                    }}>
                      <div style={{
                        width: 32, height: 32, borderRadius: '50%',
                        display: 'grid', placeItems: 'center',
                        background: isUser ? '#0f172a' : '#22c55e',
                        color: '#fff', flexShrink: 0,
                      }}>
                        {isUser ? <User size={16} /> : <Bot size={16} />}
                      </div>
                      <div style={{
                        background: isUser ? '#0f172a' : '#fff',
                        color: isUser ? '#f1f5f9' : '#0f172a',
                        padding: '10px 16px',
                        borderRadius: 12,
                        border: isUser ? 'none' : '1px solid #e2e8f0',
                        fontSize: 14,
                        lineHeight: 1.7,
                        wordBreak: 'break-word',
                      }} dangerouslySetInnerHTML={{
                        __html: marked.parse(m.content || (m.streaming ? '思考中…' : '')) as string,
                      }} />
                    </div>
                  </div>
                )
              })
            )}
          </div>
          {/* 输入区 */}
          <div style={{ padding: '12px 24px', borderTop: '1px solid #e2e8f0', background: '#fff' }}>
            <div style={{ display: 'flex', gap: 8 }}>
              <Input.TextArea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="输入消息，Enter 发送，Shift+Enter 换行"
                autoSize={{ minRows: 1, maxRows: 4 }}
                style={{ flex: 1, borderRadius: 8 }}
                disabled={loading}
              />
              <Button
                type="primary"
                icon={<Send size={14} />}
                onClick={handleSend}
                loading={loading}
                style={{ height: 'auto' }}
              />
            </div>
          </div>
        </div>
      </div>
      <Modal
        title="新建对话"
        open={showNew}
        onOk={handleCreate}
        onCancel={() => setShowNew(false)}
        okText="创建"
      >
        <Input
          autoFocus
          placeholder="对话标题（可选）"
          value={newTitle}
          onChange={(e) => setNewTitle(e.target.value)}
          onPressEnter={handleCreate}
        />
      </Modal>
    </div>
  )
}
