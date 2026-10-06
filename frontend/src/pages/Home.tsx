import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Progress, Tag, Button, Empty, App } from 'antd'
import {
  MessageSquare,
  Bot,
  NotebookPen,
  Shield,
  Timer,
  ArrowRight,
  Coins,
} from 'lucide-react'
import { api, fetchMe, tokenStore, type AuthUser } from '@shared/api'
import { listConversations, type Conversation } from '../api/agent'

interface RecentNote {
  id: string
  title: string
  updatedAt?: string
}

export default function Home() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [expiresAt, setExpiresAt] = useState<number | null>(null)
  const [tokenTotal, setTokenTotal] = useState(0)
  const [now, setNow] = useState(Date.now())
  const [recent, setRecent] = useState<RecentNote[]>([])
  const [convs, setConvs] = useState<Conversation[]>([])
  const [usage, setUsage] = useState<{ promptTokens: number; completionTokens: number }>({
    promptTokens: 0,
    completionTokens: 0,
  })

  useEffect(() => {
    fetchMe()
      .then(setUser)
      .catch(() => navigate('/login', { replace: true }))
  }, [navigate])

  useEffect(() => {
    const token = tokenStore.access
    if (!token) return
    try {
      const payload = JSON.parse(atob(token.split('.')[1]))
      if (payload.exp) {
        setExpiresAt(payload.exp * 1000)
        const issued = (payload.iat ?? 0) * 1000
        setTokenTotal(Math.max(1, payload.exp * 1000 - issued))
      }
    } catch {
      setExpiresAt(null)
    }
  }, [user])

  useEffect(() => {
    if (expiresAt === null) return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [expiresAt])

  useEffect(() => {
    api
      .get('/notes?size=5&archived=false')
      .then((res: any) => setRecent(res.data?.data?.notes ?? []))
      .catch(() => {})
  }, [])

  useEffect(() => {
    listConversations()
      .then((cs) => setConvs(cs.slice(0, 4)))
      .catch(() => {})
  }, [])

  useEffect(() => {
    api
      .get('/agent/usage')
      .then((res: any) => {
        const d = res.data?.data
        if (d) setUsage({ promptTokens: d.promptTokens ?? 0, completionTokens: d.completionTokens ?? 0 })
      })
      .catch(() => {})
  }, [])

  const remainingMs = expiresAt !== null ? expiresAt - now : null
  const percent =
    remainingMs !== null && tokenTotal > 0
      ? Math.max(0, Math.min(100, (remainingMs / tokenTotal) * 100))
      : 0
  const remainingText =
    remainingMs === null || remainingMs <= 0
      ? '已过期'
      : remainingMs >= 3_600_000
        ? `${Math.floor(remainingMs / 3_600_000)} 小时 ${Math.floor((remainingMs % 3_600_000) / 60_000)} 分钟`
        : remainingMs >= 60_000
          ? `${Math.floor(remainingMs / 60_000)} 分钟 ${Math.floor((remainingMs % 60_000) / 1000)} 秒`
          : `${Math.ceil(remainingMs / 1000)} 秒`

  const actions = [
    { key: '/chat', icon: <MessageSquare size={20} />, title: 'AI 对话', desc: '日常问答，轻量流畅' },
    { key: '/agent', icon: <Bot size={20} />, title: '工作模式', desc: 'Agent 编排 + 笔记工具' },
    { key: '/notes', icon: <NotebookPen size={20} />, title: '云笔记', desc: '记录与整理你的知识' },
    ...(user?.role === 'admin'
      ? [{ key: '/admin', icon: <Shield size={20} />, title: '管理后台', desc: '用户与系统设置' }]
      : []),
  ]

  return (
    <div
      style={{
        maxWidth: 960,
        margin: '0 auto',
        padding: '32px 24px 48px',
        color: 'var(--text)',
      }}
    >
      <div style={{ marginBottom: 24 }}>
        <div style={{ fontSize: 24, fontWeight: 800, letterSpacing: '-0.02em' }}>
          你好，{user?.username || '—'} 👋
        </div>
        <div style={{ color: 'var(--text-secondary)', marginTop: 4 }}>
          Starry 工作台 · 对话、工作、笔记，一处直达
        </div>
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
          gap: 16,
          marginBottom: 24,
        }}
      >
        {actions.map((a) => (
          <Card
            key={a.key}
            hoverable
            onClick={() => navigate(a.key)}
            style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
            styles={{ body: { display: 'flex', alignItems: 'center', gap: 14 } }}
          >
            <div
              style={{
                width: 40,
                height: 40,
                borderRadius: 10,
                display: 'grid',
                placeItems: 'center',
                background: 'var(--accent-bg)',
                color: 'var(--accent)',
                flexShrink: 0,
              }}
            >
              {a.icon}
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ fontWeight: 600 }}>{a.title}</div>
              <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{a.desc}</div>
            </div>
            <ArrowRight size={16} style={{ color: 'var(--text-secondary)' }} />
          </Card>
        ))}
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
          gap: 16,
        }}
      >
        <Card
          title="会话状态"
          style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
            <Tag color="success">
              <Timer size={12} style={{ verticalAlign: '-1px', marginRight: 4 }} />
              令牌剩余 {remainingText}
            </Tag>
          </div>
          <Progress
            percent={percent}
            showInfo={false}
            size="small"
            strokeColor={percent < 20 ? '#ef4444' : '#22c55e'}
            trailColor="var(--border)"
          />
          <div style={{ marginTop: 12, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Tag>{user?.role === 'admin' ? '管理员' : '普通用户'}</Tag>
            <Tag color={user?.status === 'active' ? 'success' : 'warning'}>
              {user?.status === 'active' ? '账号正常' : user?.status}
            </Tag>
            <Tag icon={<Coins size={12} />} color="blue">
              AI 用量 {usage.promptTokens + usage.completionTokens} tokens
            </Tag>
          </div>
        </Card>

        <Card
          title="最近对话"
          extra={
            <Button type="link" size="small" onClick={() => navigate('/agent')}>
              全部
            </Button>
          }
          style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
        >
          {convs.length === 0 ? (
            <Empty description="还没有对话" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {convs.map((c) => (
                <div
                  key={c.id}
                  onClick={() => navigate(`/agent?conv=${c.id}`)}
                  style={{
                    cursor: 'pointer',
                    padding: '8px 10px',
                    borderRadius: 8,
                    border: '1px solid var(--border)',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    gap: 8,
                  }}
                >
                  <span
                    style={{
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {c.title || '新对话'}
                  </span>
                  <ArrowRight size={14} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card
          title="最近笔记"
          extra={
            <Button type="link" size="small" onClick={() => navigate('/notes')}>
              全部
            </Button>
          }
          style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
        >
          {recent.length === 0 ? (
            <Empty description="还没有笔记" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {recent.map((n) => (
                <div
                  key={n.id}
                  onClick={() => navigate(`/notes/${n.id}`)}
                  style={{
                    cursor: 'pointer',
                    padding: '8px 10px',
                    borderRadius: 8,
                    border: '1px solid var(--border)',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    gap: 8,
                  }}
                >
                  <span
                    style={{
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {n.title || '无标题'}
                  </span>
                  <ArrowRight size={14} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}
