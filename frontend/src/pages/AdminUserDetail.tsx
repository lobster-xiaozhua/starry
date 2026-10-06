import {
  Alert,
  Avatar,
  Button,
  Card,
  Descriptions,
  Empty,
  Modal,
  Spin,
  Tag,
  App,
} from 'antd'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useEffect, useState } from 'react'
import {
  ArrowLeft,
  ShieldCheck,
  Lock,
  UserRound,
  Mail,
  Settings2,
  Copy,
  LogOut,
  KeyRound,
  Unlock,
  CalendarClock,
  UserX,
  UserCheck,
} from 'lucide-react'
import { api, extractError } from '../api/client'

interface AdminUserDetail {
  id: string
  username: string
  email: string
  role: string
  status: string
  locked: boolean
  failCount: number
  lockTtl: number
  createdAt: string
  updatedAt: string
}

const ROLE_LABEL: Record<string, string> = { admin: '管理员', user: '普通用户' }
const STATUS_LABEL: Record<string, string> = { active: '状态正常', frozen: '已冻结' }

export default function AdminUserDetail() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [detail, setDetail] = useState<AdminUserDetail | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [acting, setActing] = useState<'unlock' | 'revoke' | 'reset' | 'freeze' | 'unfreeze' | null>(null)
  const [resetToken, setResetToken] = useState('')

  const load = () => {
    api
      .get<{ data: AdminUserDetail }>(`/admin/users/${id}`)
      .then((res) => setDetail(res.data.data))
      .catch((err) => {
        if (err.response?.status === 404) setNotFound(true)
        message.error(extractError(err).message)
      })
  }

  useEffect(load, [id])

  const run = async (action: 'unlock' | 'revoke' | 'reset' | 'freeze' | 'unfreeze') => {
    setActing(action)
    try {
      const endpoint =
        action === 'reset'
          ? 'reset-password'
          : action === 'revoke'
            ? 'revoke-sessions'
            : action
      const res = await api.post(`/admin/users/${id}/${endpoint}`)
      if (res.data.code === 0) {
        if (action === 'reset') {
          setResetToken(res.data.data.token)
        } else {
          message.success(res.data.data.message)
        }
        load()
      }
    } catch (err) {
      message.error(extractError(err).message)
    } finally {
      setActing(null)
    }
  }

  const copyToken = async () => {
    try {
      await navigator.clipboard.writeText(resetToken)
      message.success('重置令牌已复制')
    } catch {
      message.warning('复制失败，请手动选择复制')
    }
  }

  if (notFound) {
    return (
      <div className="admin-card" style={{ textAlign: 'center', padding: '64px 0' }}>
        <Empty description="用户不存在" />
        <Link to="/admin?tab=users">
          <Button style={{ marginTop: 16 }}>返回用户管理</Button>
        </Link>
      </div>
    )
  }

  if (!detail) {
    return (
      <div className="admin-card" style={{ textAlign: 'center', padding: '64px 0' }}>
        <Spin size="large" />
      </div>
    )
  }

  const lockTtlText = detail.lockTtl > 0
    ? `${Math.floor(detail.lockTtl / 60)} 分钟 ${detail.lockTtl % 60} 秒`
    : ''

  return (
    <div className="admin-card">
      <div className="page-header" style={{ marginBottom: 4 }}>
        <Button
          type="text"
          icon={<ArrowLeft size={16} aria-hidden />}
          onClick={() => navigate(-1)}
          style={{ paddingLeft: 0 }}
        >
          返回
        </Button>
      </div>

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 16,
          padding: '12px 0 20px',
          borderBottom: '1px solid var(--color-border)',
          marginBottom: 24,
        }}
      >
        <Avatar size={52} style={{ backgroundColor: detail.role === 'admin' ? '#f59e0b' : '#22c55e', fontSize: 22 }}>
          {detail.username.slice(0, 1).toUpperCase()}
        </Avatar>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 20, fontWeight: 800, color: 'var(--color-primary)', letterSpacing: '-0.02em' }}>
              {detail.username}
            </span>
            <Tag color={detail.role === 'admin' ? 'gold' : 'green'}>
              {ROLE_LABEL[detail.role] ?? detail.role}
            </Tag>
            {detail.status === 'frozen' ? (
              <Tag color="red" icon={<UserX size={12} aria-hidden />}>
                已冻结
              </Tag>
            ) : detail.locked ? (
              <Tag color="red" icon={<Lock size={12} aria-hidden />}>
                已锁定 {lockTtlText}
              </Tag>
            ) : (
              <Tag color="success" icon={<ShieldCheck size={12} aria-hidden />}>
                状态正常
              </Tag>
            )}
            {detail.status !== 'active' && detail.status !== 'frozen' && (
              <Tag color="default">{STATUS_LABEL[detail.status] ?? detail.status}</Tag>
            )}
          </div>
          <div style={{ color: 'var(--color-muted)', fontSize: 13, marginTop: 6 }}>{detail.email}</div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 24 }}>
        <Card title="账号信息" bordered={false} style={{ background: '#f8fafc' }}>
          <Descriptions column={1} size="small" labelStyle={{ width: 90, color: 'var(--color-muted)' }}>
            <Descriptions.Item label="用户 ID">
              <code style={{ fontSize: 12, wordBreak: 'break-all' }}>{detail.id}</code>
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              <CalendarClock size={13} style={{ verticalAlign: '-2px', marginRight: 6, color: '#64748b' }} aria-hidden />
              {formatTime(detail.createdAt)}
            </Descriptions.Item>
            <Descriptions.Item label="更新时间">{formatTime(detail.updatedAt)}</Descriptions.Item>
          </Descriptions>
        </Card>

        <Card title="安全状态" bordered={false} style={{ background: '#f8fafc' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <UserRound size={16} style={{ color: '#64748b' }} aria-hidden />
              <span style={{ color: 'var(--color-muted)', fontSize: 13 }}>连续登录失败</span>
              <span style={{ fontWeight: 700, fontSize: 15, color: detail.failCount > 0 ? '#ef4444' : 'var(--color-primary)' }}>
                {detail.failCount} 次
              </span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <Lock size={16} style={{ color: '#64748b' }} aria-hidden />
              <span style={{ color: 'var(--color-muted)', fontSize: 13 }}>锁定状态</span>
              <span style={{ fontWeight: 700, fontSize: 14, color: detail.locked ? '#ef4444' : '#22c55e' }}>
                {detail.locked ? `已锁定（剩余 ${lockTtlText}）` : '未锁定'}
              </span>
            </div>
          </div>
          <div style={{ display: 'flex', gap: 10, marginTop: 20, flexWrap: 'wrap' }}>
            <Button
              danger
              loading={acting === 'unlock'}
              icon={<Unlock size={15} aria-hidden />}
              onClick={() => void run('unlock')}
              disabled={!detail.locked && detail.failCount === 0}
            >
              解锁账号
            </Button>
            {detail.status === 'frozen' ? (
              <Button
                type="primary"
                loading={acting === 'unfreeze'}
                icon={<UserCheck size={15} aria-hidden />}
                onClick={() => void run('unfreeze')}
              >
                解除冻结
              </Button>
            ) : (
              <Button
                danger
                loading={acting === 'freeze'}
                icon={<UserX size={15} aria-hidden />}
                disabled={detail.role === 'admin'}
                onClick={() =>
                  Modal.confirm({
                    title: '冻结该账号？',
                    content: '冻结后该用户将无法登录，且全部在线会话立即失效；可随时解除冻结。',
                    okText: '确认冻结',
                    okButtonProps: { danger: true },
                    cancelText: '取消',
                    onOk: () => run('freeze'),
                  })
                }
              >
                冻结账号
              </Button>
            )}
            <Button
              type="primary"
              loading={acting === 'reset'}
              icon={<KeyRound size={15} aria-hidden />}
              onClick={() => void run('reset')}
            >
              强制重置密码
            </Button>
            <Button
              loading={acting === 'revoke'}
              icon={<LogOut size={15} aria-hidden />}
              onClick={() =>
                Modal.confirm({
                  title: '吊销该用户全部会话？',
                  content: '该用户所有未过期的刷新令牌将立即失效，需重新登录。',
                  okText: '确认吊销',
                  cancelText: '取消',
                  onOk: () => run('revoke'),
                })
              }
            >
              吊销全部会话
            </Button>
          </div>
        </Card>
      </div>

      {resetToken && (
        <Alert
          style={{ marginTop: 20 }}
          type="success"
          showIcon
          icon={<KeyRound size={16} aria-hidden />}
          message={
            <span style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
              重置令牌已生成（30 分钟内有效，单次使用）：
              <code style={{ background: '#f1f5f9', padding: '2px 10px', borderRadius: 6, wordBreak: 'break-all' }}>
                {resetToken}
              </code>
              <Button size="small" icon={<Copy size={13} aria-hidden />} onClick={copyToken}>
                复制
              </Button>
            </span>
          }
          description={
            <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}>
              <Mail size={13} aria-hidden />
              可引导用户通过 {`/reset-password?token=`}
              {resetToken} 完成自助重置（开发模式同时已写入服务端日志）。
            </span>
          }
        />
      )}
    </div>
  )
}

function formatTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}
