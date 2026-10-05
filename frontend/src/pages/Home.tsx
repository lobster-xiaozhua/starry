import { Avatar, Button, Progress, Tag, App } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { ShieldCheck, LogOut, UserRound, Mail, Lock, Settings2, Timer } from 'lucide-react'
import { Bot } from 'lucide-react'
import { api, extractError, tokenStore } from '../api/client'
import type { AuthUser } from '../api/client'

export default function Home() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [expiresAt, setExpiresAt] = useState<number | null>(null)
  const [tokenTotal, setTokenTotal] = useState<number>(0)
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    api
      .get('/auth/me')
      .then((res) => setUser(res.data.data.user))
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
        setTokenTotal(payload.exp * 1000 - issued)
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

  const logout = async () => {
    try {
      await api.post('/auth/logout', { refreshToken: tokenStore.refresh })
    } catch (err) {
      message.error(extractError(err).message)
      return
    }
    tokenStore.clear()
    message.success('已登出')
    navigate('/login', { replace: true })
  }

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

  return (
    <>
      <div className="app-topbar">
        <div className="brand">
          <span className="brand-mark">
            <ShieldCheck size={18} aria-hidden />
          </span>
          账号认证系统
        </div>
        {user && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Avatar style={{ backgroundColor: user.role === 'admin' ? '#f59e0b' : '#22c55e' }}>
              {user.username.slice(0, 1).toUpperCase()}
            </Avatar>
            <span style={{ color: '#0f172a', fontSize: 14, fontWeight: 500 }}>{user.username}</span>
          </div>
        )}
      </div>
      <div className="account-page">
        <div className="account-card">
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 24 }}>
            <ShieldCheck size={22} style={{ color: '#22c55e' }} aria-hidden />
            <h1 style={{ margin: 0, fontSize: 20, fontWeight: 800, color: '#0f172a', letterSpacing: '-0.02em' }}>
              登录状态正常
            </h1>
            <Tag color="success">
              <Timer size={12} style={{ verticalAlign: '-1px', marginRight: 4 }} aria-hidden />
              会话 {remainingText}
            </Tag>
          </div>
          {user && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 16 }}>
              <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
                <UserRound size={18} style={{ color: '#64748b', marginTop: 2 }} aria-hidden />
                <div>
                  <div style={{ fontSize: 12, color: '#64748b', marginBottom: 2 }}>用户名</div>
                  <div style={{ fontWeight: 600, color: '#0f172a' }}>{user.username}</div>
                </div>
              </div>
              <div>
                <Mail size={18} style={{ color: '#64748b', marginTop: 2 }} aria-hidden />
                <div>
                  <div style={{ fontSize: 12, color: '#64748b', marginBottom: 2 }}>邮箱</div>
                  <div style={{ fontWeight: 600, color: '#0f172a' }}>{user.email}</div>
                </div>
              </div>
              <div>
                <Settings2 size={18} style={{ color: '#64748b', marginTop: 2 }} aria-hidden />
                <div>
                  <div style={{ fontSize: 12, color: '#64748b', marginBottom: 2 }}>角色</div>
                  <div style={{ fontWeight: 600, color: '#0f172a' }}>
                    {user.role === 'admin' ? '管理员' : '普通用户'}
                  </div>
                </div>
              </div>
              <div>
                <Lock size={18} style={{ color: '#64748b', marginTop: 2 }} aria-hidden />
                <div>
                  <div style={{ fontSize: 12, color: '#64748b', marginBottom: 2 }}>账号状态</div>
                  <div style={{ fontWeight: 600, color: '#0f172a' }}>
                    {user.status === 'active' ? '正常' : user.status}
                  </div>
                </div>
              </div>
            </div>
          )}
          {expiresAt !== null && (
            <div style={{ marginTop: 24 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8 }}>
                <Timer size={16} style={{ color: '#64748b' }} aria-hidden />
                <span style={{ fontSize: 12, color: '#64748b' }}>访问令牌剩余（过期后自动刷新）</span>
                <span style={{ fontSize: 12, fontWeight: 600, color: remainingMs !== null && remainingMs < 60_000 ? '#ef4444' : '#0f172a', marginLeft: 'auto' }}>
                  {remainingText}
                </span>
              </div>
              <Progress
                percent={percent}
                showInfo={false}
                size="small"
                strokeColor={percent < 20 ? '#ef4444' : '#22c55e'}
                trailColor="#e2e8f0"
              />
            </div>
          )}
          <div style={{ marginTop: 28, display: 'flex', gap: 12 }}>
            {user?.role === 'admin' && (
              <Link to="/admin" style={{ flex: 1 }}>
            <Button type="primary" size="large" block>
              <Settings2 size={16} style={{ marginRight: 8 }} aria-hidden />
              进入管理页面
            </Button>
              </Link>
            )}
            <Link to="/chat" style={{ flex: 1 }}>
              <Button size="large" block style={{ borderColor: '#0f172a', color: '#0f172a' }}>
                <Bot size={16} style={{ marginRight: 8 }} aria-hidden />
                AI 助手
              </Button>
            </Link>
            <Button size="large" style={{ flex: 1 }} danger onClick={logout}>
              <LogOut size={16} style={{ marginRight: 8 }} aria-hidden />
              退出登录
            </Button>
          </div>
        </div>
      </div>
    </>
  )
}
