import { useEffect, useState } from 'react'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { Layout, Menu, Avatar, Button } from 'antd'
import { MessageSquare, Bot, NotebookPen, Monitor, Sun, Moon, LogOut, Database, CheckSquare, FolderOpen, Lock } from 'lucide-react'
import { fetchMe, tokenStore, api, type AuthUser } from '@shared/api'
import { useTheme } from '../../theme.tsx'

const { Sider, Content } = Layout

/**
 * 统一外壳：左侧导航承载 对话 / 工作 / 笔记 / 管理，底部为用户区（头像/主题/登出）。
 * 登录态与主题由 @shared/api 与 theme.tsx 统一管理；暗色通过 .theme-dark 类驱动笔记区 CSS 变量。
 */
export default function AppShell() {
  const navigate = useNavigate()
  const location = useLocation()
  const { dark, toggle } = useTheme()
  const [user, setUser] = useState<AuthUser | null>(null)
  // 窄屏（<992px）折叠为图标栏：220px 侧栏会在手机上占掉半屏，正文被挤成竖排。
  const [collapsed, setCollapsed] = useState(false)

  useEffect(() => {
    fetchMe().then(setUser).catch(() => {})
  }, [])

  const handleLogout = async () => {
    try {
      await api.post('/auth/logout', { refreshToken: tokenStore.refresh })
    } catch {
      // 即使登出请求失败也清除本地令牌
    }
    tokenStore.clear()
    navigate('/login', { replace: true })
  }

  const selectedKey = (() => {
    const p = location.pathname
    if (p.startsWith('/notes')) return '/notes'
    if (p.startsWith('/knowledge')) return '/knowledge'
    if (p.startsWith('/agent')) return '/agent'
    if (p.startsWith('/boards')) return '/boards'
    if (p.startsWith('/drive')) return '/drive'
    if (p.startsWith('/vault')) return '/vault'
    if (p.startsWith('/console')) return '/console'
    if (p.startsWith('/admin')) return '/console'
    return '/chat'
  })()

  const items = [
    { key: '/chat', icon: <MessageSquare size={18} />, label: '对话' },
    { key: '/agent', icon: <Bot size={18} />, label: '工作' },
    { key: '/notes', icon: <NotebookPen size={18} />, label: '笔记' },
    { key: '/knowledge', icon: <Database size={18} />, label: '知识库' },
    { key: '/boards', icon: <CheckSquare size={18} />, label: '任务' },
    { key: '/drive', icon: <FolderOpen size={18} />, label: '网盘' },
    { key: '/vault', icon: <Lock size={18} />, label: '保险箱' },
    ...(user?.role === 'admin'
      ? [{ key: '/console', icon: <Monitor size={18} />, label: '控制台' }]
      : []),
  ]

  return (
    <div className={dark ? 'theme-dark' : ''} style={{ height: '100vh' }}>
      <Layout style={{ height: '100vh' }}>
        <Sider
          width={220}
          collapsedWidth={72}
          breakpoint="lg"
          collapsed={collapsed}
          onCollapse={setCollapsed}
          theme={dark ? 'dark' : 'light'}
          style={{ borderRight: `1px solid ${dark ? '#1e293b' : '#e2e8f0'}`, position: 'relative' }}
        >
          <div
            style={{
              padding: collapsed ? '18px 0 8px' : '18px 16px 8px',
              textAlign: collapsed ? 'center' : 'left',
              fontWeight: 700,
              fontSize: 16,
              color: dark ? '#f1f5f9' : '#0f172a',
            }}
          >
            {collapsed ? 'S' : 'Starry'}
          </div>
          <Menu
            mode="inline"
            theme={dark ? 'dark' : 'light'}
            selectedKeys={[selectedKey]}
            onClick={({ key }) => navigate(key)}
            items={items}
            style={{ flex: 1, borderRight: 0, background: 'transparent' }}
          />
          <div
            style={{
              borderTop: `1px solid ${dark ? '#1e293b' : '#e2e8f0'}`,
              padding: collapsed ? 8 : 12,
              display: 'flex',
              alignItems: 'center',
              justifyContent: collapsed ? 'center' : 'flex-start',
              flexWrap: 'wrap',
              gap: collapsed ? 0 : 10,
            }}
          >
            {user && !collapsed && (
              <Avatar
                size={30}
                style={{
                  backgroundColor: user.role === 'admin' ? '#f59e0b' : '#22c55e',
                  flexShrink: 0,
                }}
              >
                {user.username.slice(0, 1).toUpperCase()}
              </Avatar>
            )}
            {!collapsed && (
              <span
                style={{
                  flex: 1,
                  fontSize: 13,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  color: dark ? '#f1f5f9' : '#0f172a',
                }}
              >
                {user?.username}
              </span>
            )}
            <Button type="text" onClick={toggle} icon={dark ? <Sun size={16} /> : <Moon size={16} />} />
            <Button type="text" onClick={handleLogout} icon={<LogOut size={16} />} />
          </div>
        </Sider>
        <Layout style={{ background: dark ? '#020617' : '#f8fafc' }}>
          <Content style={{ overflow: 'auto', height: '100vh' }}>
            <Outlet />
          </Content>
        </Layout>
      </Layout>
    </div>
  )
}
