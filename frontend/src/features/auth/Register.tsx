import { Button, Card, Form, Input, App } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { UserPlus, ShieldCheck } from 'lucide-react'
import { api, extractError } from '../../shared/api/client.ts'
import type { AuthSettings } from '../../shared/api/client.ts'
import BrandPanel from '../../shared/ui/BrandPanel.tsx'
import PasswordStrength from './PasswordStrength.tsx'

interface RegisterForm {
  username: string
  email: string
  password: string
  confirm: string
}

export default function Register() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [settings, setSettings] = useState<AuthSettings | null>(null)

  useEffect(() => {
    api
      .get('/auth/password-policy')
      .then((res) => setSettings(res.data.data))
      .catch(() => setSettings(null))
  }, [])

  const policyText = settings
    ? [
        `至少 ${settings.passwordMinLength} 位`,
        [
          settings.passwordRequireUpper && '大写字母',
          settings.passwordRequireLower && '小写字母',
          settings.passwordRequireDigit && '数字',
          settings.passwordRequireSpecial && '特殊字符',
        ]
          .filter(Boolean)
          .join(' / ') + `（至少 ${settings.passwordMinCategories} 类）`,
      ].join('，')
    : ''

  const onFinish = async (values: RegisterForm) => {
    setLoading(true)
    try {
      const res = await api.post('/auth/register', {
        username: values.username,
        email: values.email,
        password: values.password,
      })
      if (res.data.code === 0) {
        message.success('注册成功，请登录')
        navigate('/login', { replace: true })
        return
      }
      message.error(res.data.message)
    } catch (err) {
      const { message: msg, fields } = extractError(err)
      message.warning(fields?.password ? `${msg}：${fields.password}` : msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-layout">
        <BrandPanel title="创建你的账号" subtitle="注册后即可使用，密码强度要求实时来自管理员安全策略。" />
        <Card className="auth-card" bordered={false}>
          <div className="auth-brand-header">
            <span className="brand-mark">
              <UserPlus size={20} aria-hidden />
            </span>
            <span className="brand-name">注册账号</span>
          </div>
          {policyText && (
            <div className="policy-note">
              <ShieldCheck size={14} aria-hidden style={{ display: 'inline-block', marginRight: 6, verticalAlign: '-2px' }} />
              密码要求：{policyText}
            </div>
          )}
          <Form<RegisterForm> layout="vertical" onFinish={onFinish} autoComplete="off" scrollToFirstError>
            <Form.Item
              name="username"
              label="用户名"
              rules={[
                { required: true, message: '请输入用户名' },
                { min: 3, max: 64, message: '用户名长度 3-64 个字符' },
              ]}
            >
              <Input placeholder="用户名" size="large" autoComplete="username" />
            </Form.Item>
            <Form.Item
              name="email"
              label="邮箱"
              rules={[
                { required: true, message: '请输入邮箱' },
                { type: 'email', message: '邮箱格式不正确' },
              ]}
            >
              <Input placeholder="email@example.com" size="large" autoComplete="email" />
            </Form.Item>
            <Form.Item
              name="password"
              label="密码"
              rules={[{ required: true, message: '请输入密码' }]}
            >
              <Input.Password placeholder="密码" size="large" autoComplete="new-password" />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.password !== cur.password}>
              {({ getFieldValue }) => (
                <PasswordStrength
                  password={getFieldValue('password') ?? ''}
                  minCategories={settings?.passwordMinCategories ?? 3}
                />
              )}
            </Form.Item>
            <Form.Item
              name="confirm"
              label="确认密码"
              dependencies={['password']}
              rules={[
                { required: true, message: '请再次输入密码' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('password') === value) return Promise.resolve()
                    return Promise.reject(new Error('两次输入的密码不一致'))
                  },
                }),
              ]}
            >
              <Input.Password placeholder="再次输入密码" size="large" autoComplete="new-password" />
            </Form.Item>
            <Form.Item style={{ marginBottom: 8 }}>
              <Button type="primary" htmlType="submit" size="large" block loading={loading}>
                <UserPlus size={16} style={{ marginRight: 8 }} aria-hidden />
                注 册
              </Button>
            </Form.Item>
            <div className="auth-links">
              <span />
              <Link to="/login">已有账号？去登录</Link>
            </div>
          </Form>
        </Card>
      </div>
    </div>
  )
}
