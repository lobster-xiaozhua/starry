import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Avatar } from 'antd'
import { Sun, Moon, LogOut } from 'lucide-react'
import { listTags } from '../api/notes'
import { fetchMe, tokenStore, api } from '@shared/api'
import type { AuthUser } from '@shared/api'
import { useTheme } from '../theme'

type Props = {
  selectedTags: string[]
  onToggleTag: (tag: string) => void
  onShowArchived: () => void
  showArchived: boolean
  tagVersion: number
}

const ACCOUNT_PORT = 5173

export default function Sidebar({
  selectedTags,
  onToggleTag,
  onShowArchived,
  showArchived,
  tagVersion,
}: Props) {
  const [tags, setTags] = useState<{ name: string; count: number }[]>([])
  const [user, setUser] = useState<AuthUser | null>(null)
  const { dark, toggle } = useTheme()
  const navigate = useNavigate()

  // tagVersion 由父组件在笔记增删改事件时递增，驱动标签计数刷新
  useEffect(() => {
    listTags().then(setTags).catch(() => setTags([]))
  }, [tagVersion])

  // 拉取当前用户身份，与登录系统打通
  useEffect(() => {
    fetchMe().then(setUser)
  }, [])

  const handleLogout = useCallback(async () => {
    try {
      await api.post('/auth/logout', { refreshToken: tokenStore.refresh })
    } catch {
      // 即使登出请求失败也清除本地令牌
    }
    tokenStore.clear()
    navigate('/login', { replace: true })
  }, [navigate])

  const accountUrl = `${window.location.protocol}//${window.location.hostname}:${ACCOUNT_PORT}/`

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <strong style={{ color: 'var(--accent)' }}>云笔记</strong>
        <button
          onClick={toggle}
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            color: 'var(--text)',
            display: 'flex',
            alignItems: 'center',
          }}
          aria-label="切换主题"
        >
          {dark ? <Sun size={18} /> : <Moon size={18} />}
        </button>
      </div>
      <div className="sidebar-content">
        <div style={{ fontSize: 12, color: 'var(--text-secondary)', padding: '8px 10px' }}>
          标签
        </div>
        <ul className="tag-list">
          {tags.map((t) => (
            <li
              key={t.name}
              className={`tag-item ${selectedTags.includes(t.name) ? 'active' : ''}`}
              role="button"
              tabIndex={0}
              onClick={() => onToggleTag(t.name)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  onToggleTag(t.name)
                }
              }}
            >
              <span>#{t.name}</span>
              <span className="tag-count">{t.count}</span>
            </li>
          ))}
          {tags.length === 0 && (
            <div style={{ fontSize: 13, color: 'var(--text-secondary)', padding: '8px 10px' }}>
              暂无标签
            </div>
          )}
        </ul>
        <div style={{ fontSize: 12, color: 'var(--text-secondary)', padding: '16px 10px 8px' }}>
          归档
        </div>
        <button
          onClick={onShowArchived}
          style={{
            width: '100%',
            padding: '8px 10px',
            border: 'none',
            borderRadius: 8,
            background: showArchived ? 'var(--accent-bg)' : 'var(--bg)',
            color: showArchived ? 'var(--accent)' : 'var(--text)',
            cursor: 'pointer',
            fontSize: 14,
          }}
        >
          {showArchived ? '退出归档' : '查看归档'}
        </button>
      </div>
      <div
        style={{
          borderTop: '1px solid var(--border)',
          padding: '12px',
          display: 'flex',
          alignItems: 'center',
          gap: 10,
        }}
      >
        {user && (
          <>
            <Avatar
              size={28}
              style={{
                backgroundColor: user.role === 'admin' ? '#f59e0b' : '#22c55e',
                flexShrink: 0,
              }}
            >
              {user.username.slice(0, 1).toUpperCase()}
            </Avatar>
            <span
              style={{
                fontSize: 13,
                fontWeight: 500,
                color: 'var(--text)',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                flex: 1,
              }}
            >
              {user.username}
            </span>
          </>
        )}
        <a
          href={accountUrl}
          target="_blank"
          rel="noreferrer"
          style={{
            fontSize: 12,
            color: 'var(--text-secondary)',
            textDecoration: 'none',
            whiteSpace: 'nowrap',
          }}
          title="账号管理"
        >
          账号
        </a>
        <button
          onClick={handleLogout}
          aria-label="退出登录"
          title="退出登录"
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            color: 'var(--text-secondary)',
            display: 'flex',
            alignItems: 'center',
            padding: 4,
            flexShrink: 0,
          }}
        >
          <LogOut size={16} />
        </button>
      </div>
    </aside>
  )
}
