import {
  Alert,
  Avatar,
  Button,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  Menu,
  Space,
  Spin,
  Switch,
  Tag,
  App,
} from 'antd'
import { useNavigate } from 'react-router-dom'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { GetProps } from 'antd'
import {
  ShieldCheck,
  KeyRound,
  Timer,
  Eye,
  Users,
  LogOut,
  Search,
  Unlock,
  Save,
  RotateCcw,
  Lock,
  Info,
  ChevronRight,
} from 'lucide-react'
import { api, extractError, tokenStore } from '../../shared/api/client.ts'
import { formatError } from '../../shared/lib/errors.ts'
import type { AuthSettings } from '../../shared/api/client.ts'

interface AdminUserView {
  id: string
  username: string
  email: string
  role: string
  status: string
  locked: boolean
  failCount: number
}

type FieldType<T> = Record<keyof T, unknown>

type MenuKey = 'password' | 'lockout' | 'session' | 'captcha' | 'users'

const SECTION_META: { key: MenuKey; title: string; desc: string; icon: React.ElementType }[] = [
  { key: 'password', title: '密码策略', desc: '注册与改密时的密码强度要求', icon: KeyRound },
  { key: 'lockout', title: '锁定策略', desc: '连续登录失败的自动锁定规则', icon: Lock },
  { key: 'session', title: '会话有效期', desc: 'JWT 双令牌时长，仅对新令牌生效', icon: Timer },
  { key: 'captcha', title: '验证码开关', desc: '登录图形验证码的启用状态', icon: Eye },
]

export default function AdminSettings() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [settings, setSettings] = useState<AuthSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const [users, setUsers] = useState<AdminUserView[]>([])
  const [search, setSearch] = useState('')
  const [activeKey, setActiveKey] = useState<MenuKey>('password')
  const [highlight, setHighlight] = useState<MenuKey | null>(null)
  const [form] = Form.useForm<FieldType<AuthSettings>>()
  const sectionRefs = useRef<Partial<Record<MenuKey, HTMLElement | null>>>({})
  const highlightTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const loadUsers = useCallback(
    async (keyword: string) => {
      try {
        const res = await api.get('/admin/users', { params: keyword ? { search: keyword } : {} })
        setUsers(res.data.data.users ?? [])
      } catch (err) {
        message.error(formatError(err))
      }
    },
    [message],
  )

  useEffect(() => {
    api
      .get<{ data: AuthSettings }>('/admin/settings')
      .then((res) => {
        setSettings(res.data.data)
        form.setFieldsValue(res.data.data)
      })
      .catch(() => navigate('/login', { replace: true }))
    void loadUsers('')
  }, [form, navigate, loadUsers])

  useEffect(() => {
    return () => {
      if (highlightTimer.current !== null) clearTimeout(highlightTimer.current)
      highlightTimer.current = null
    }
  }, [])

  const selectMenu = (key: MenuKey) => {
    setActiveKey(key)
    if (key === 'users') return
    const el = sectionRefs.current[key]
    if (el instanceof HTMLElement) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' })
      setHighlight(key)
      highlightTimer.current = setTimeout(() => setHighlight(null), 1200)
    }
  }

  useEffect(() => {
    if (activeKey === 'users') return
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveKey((entry.target as HTMLElement).dataset.section as MenuKey)
            break
          }
        }
      },
      { rootMargin: '-40% 0px -55% 0px', threshold: 0 },
    )
    Object.values(sectionRefs.current).forEach((el) => {
      if (el instanceof HTMLElement) observer.observe(el)
    })
    return () => observer.disconnect()
  }, [settings])

  const logout = async () => {
    try {
      await api.post('/auth/logout', { refreshToken: tokenStore.refresh })
    } catch {
      /* ignore */
    }
    tokenStore.clear()
    navigate('/login', { replace: true })
  }

  const onSave = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      const res = await api.put('/admin/settings', values)
      if (res.data.code === 0) {
        setSettings(res.data.data)
        message.success('保存成功，参数已即时生效')
      } else {
        message.error(res.data.message)
      }
    } catch (err) {
      const { message: msg, fields } = extractError(err)
      if (fields) {
        message.warning(Object.entries(fields).map(([k, v]) => `${k}: ${v}`).join('；'))
      } else {
        message.error(msg)
      }
    } finally {
      setSaving(false)
    }
  }

  const unlock = async (id: string) => {
    try {
      const res = await api.post(`/admin/users/${id}/unlock`)
      if (res.data.code === 0) {
        message.success('已解除锁定并清零失败计数')
        void loadUsers(search)
      }
    } catch (err) {
      message.error(formatError(err))
    }
  }

  const strategyForm = (
    <Form form={form} layout="vertical" style={{ maxWidth: 720 }} scrollToFirstError>
      {SECTION_META.map(({ key, title, desc, icon: Icon }) => (
        <section
          key={key}
          ref={(el) => {
            sectionRefs.current[key] = el
          }}
          data-section={key}
          className={`setting-section${highlight === key ? ' setting-section--hl' : ''}${activeKey === key ? ' setting-section--active' : ''}`}
          style={{ scrollMarginTop: 72 }}
        >
          <div className="setting-section-header">
            <span className="setting-section-icon">
              <Icon size={18} aria-hidden />
            </span>
            <div>
              <div className="setting-section-title">{title}</div>
              <div className="setting-section-desc">{desc}</div>
            </div>
          </div>
          {key === 'password' && (
            <div className="setting-fields">
              <Form.Item label="密码最小长度" name="passwordMinLength" extra="允许范围 8-128">
                <InputNumber min={8} max={128} style={{ width: 160 }} />
              </Form.Item>
              <div className="setting-switch-grid">
                <Form.Item label="要求大写字母" name="passwordRequireUpper" valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item label="要求小写字母" name="passwordRequireLower" valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item label="要求数字" name="passwordRequireDigit" valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item label="要求特殊字符" name="passwordRequireSpecial" valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item
                  label="最少字符类别数"
                  name="passwordMinCategories"
                  extra="1-4，且不超过已启用类别数"
                >
                  <InputNumber min={1} max={4} style={{ width: 160 }} />
                </Form.Item>
              </div>
            </div>
          )}
          {key === 'lockout' && (
            <div className="setting-fields">
              <Form.Item label="连续失败锁定阈值（次）" name="lockoutThreshold" extra="允许范围 1-20">
                <InputNumber min={1} max={20} style={{ width: 160 }} />
              </Form.Item>
              <Form.Item
                label="锁定时长（分钟）"
                name="lockoutDurationMinutes"
                extra="允许范围 1-1440"
              >
                <InputNumber min={1} max={1440} style={{ width: 160 }} />
              </Form.Item>
              <Alert
                type="info"
                icon={<Info size={16} aria-hidden />}
                showIcon
                message="锁定与解锁"
                description="达到阈值后账号锁定对应时长；也可在“用户管理”中手动解锁。"
              />
            </div>
          )}
          {key === 'session' && (
            <div className="setting-fields">
              <Form.Item
                label="Access Token 有效期（分钟）"
                name="accessTokenMinutes"
                extra="允许范围 5-120，仅对新颁发令牌生效"
              >
                <InputNumber min={5} max={120} style={{ width: 160 }} />
              </Form.Item>
              <Form.Item
                label="Refresh Token 有效期（天）"
                name="refreshTokenDays"
                extra="允许范围 1-30，仅对新颁发令牌生效"
              >
                <InputNumber min={1} max={30} style={{ width: 160 }} />
              </Form.Item>
            </div>
          )}
          {key === 'captcha' && (
            <div className="setting-fields">
              <Form.Item
                label="登录图形验证码"
                name="captchaEnabled"
                valuePropName="checked"
                extra="启用后登录页要求输入图形验证码"
              >
                <Switch checkedChildren="启用" unCheckedChildren="禁用" />
              </Form.Item>
            </div>
          )}
        </section>
      ))}
      <div className="form-footer">
        <Space>
          <Button type="primary" size="large" loading={saving} onClick={onSave} icon={<Save size={15} aria-hidden />}>
            保存参数
          </Button>
          <Button
            size="large"
            icon={<RotateCcw size={15} aria-hidden />}
            onClick={() => {
              if (settings) form.setFieldsValue(settings)
              message.info('已还原为当前生效配置')
            }}
          >
            还原
          </Button>
        </Space>
        <span className="form-footer-hint">保存后立即写入配置缓存并对新请求生效</span>
      </div>
    </Form>
  )

  const usersCard = (
    <div className="admin-card">
      <div className="page-header">
        <h1>
          <Users size={20} style={{ verticalAlign: '-3px', marginRight: 8, color: '#0f172a' }} aria-hidden />
          用户管理
        </h1>
        <Tag color="success">共 {users.length} 个用户</Tag>
      </div>
      <Divider style={{ margin: '20px 0' }} />
      <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
        <Input
          prefix={<Search size={15} style={{ color: '#94a3b8' }} aria-hidden />}
          placeholder="搜索用户名或邮箱"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onPressEnter={() => void loadUsers(search)}
        />
        <Button onClick={() => void loadUsers(search)}>搜索</Button>
      </Space.Compact>
      {users.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="没有匹配的用户"
          style={{ padding: '32px 0' }}
        />
      ) : (
        <List
          dataSource={users}
          renderItem={(u) => (
            <List.Item
              style={{ cursor: 'pointer' }}
              onClick={() => navigate(`/admin/users/${u.id}`)}
              actions={
                u.locked
                  ? [
                      <Button
                        key="unlock"
                        size="small"
                        type="primary"
                        danger
                        icon={<Unlock size={14} aria-hidden />}
                        onClick={(e) => {
                          e.stopPropagation()
                          void unlock(u.id)
                        }}
                      >
                        解锁
                      </Button>,
                    ]
                  : undefined
              }
            >
              <List.Item.Meta
                avatar={
                  <Avatar style={{ backgroundColor: u.role === 'admin' ? '#f59e0b' : '#22c55e' }}>
                    {u.username.slice(0, 1).toUpperCase()}
                  </Avatar>
                }
                title={
                  <Space>
                    {u.username}
                    {u.role === 'admin' && <Tag color="gold">管理员</Tag>}
                    {u.locked && (
                      <Tag color="red" icon={<Lock size={12} aria-hidden />}>
                        已锁定 失败{u.failCount}次
                      </Tag>
                    )}
                  </Space>
                }
                description={
                  <Space size={6} wrap>
                    <span>{u.email}</span>
                    <ChevronRight size={13} style={{ color: 'var(--color-muted)' }} aria-hidden />
                  </Space>
                }
              />
            </List.Item>
          )}
        />
      )}
    </div>
  )

  const menuItems: GetProps<typeof Menu>['items'] = [
    { key: 'password', icon: <KeyRound size={16} aria-hidden />, label: '密码策略' },
    { key: 'lockout', icon: <Lock size={16} aria-hidden />, label: '锁定策略' },
    { key: 'session', icon: <Timer size={16} aria-hidden />, label: '会话有效期' },
    { key: 'captcha', icon: <Eye size={16} aria-hidden />, label: '验证码开关' },
    { type: 'divider' },
    { key: 'users', icon: <Users size={16} aria-hidden />, label: '用户管理' },
  ]

  return (
    <>
      <div className="app-topbar">
        <div className="brand">
          <span className="brand-mark">
            <ShieldCheck size={18} aria-hidden />
          </span>
          登录安全参数管理
        </div>
        <Button type="text" icon={<LogOut size={16} aria-hidden />} onClick={logout} style={{ color: '#334155' }}>
          退出登录
        </Button>
      </div>
      <div className="admin-shell">
        <aside className="admin-sidebar">
          <div className="sidebar-title">安全参数</div>
          <Menu
            mode="inline"
            selectedKeys={[activeKey]}
            items={menuItems}
            onClick={({ key }) => selectMenu(key as MenuKey)}
            style={{ border: 'none', background: 'transparent' }}
          />
        </aside>
        <div className="admin-content">
          {activeKey === 'users' ? (
            usersCard
          ) : settings ? (
            <div className="admin-card">{strategyForm}</div>
          ) : (
            <div className="admin-card" style={{ textAlign: 'center', padding: '64px 0' }}>
              <Spin size="large" />
            </div>
          )}
        </div>
      </div>
    </>
  )
}
