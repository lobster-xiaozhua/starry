import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, App } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { UserPlus } from 'lucide-react'
import { api, ApiResponse, extractError } from '@shared/api'

interface RegisterForm {
  username: string
  email: string
  password: string
  confirm: string
}

interface PasswordPolicy {
  passwordMinLength: number
  passwordRequireUpper: boolean
  passwordRequireLower: boolean
  passwordRequireDigit: boolean
  passwordRequireSpecial: boolean
  passwordMinCategories: number
}

export default function RegisterPage() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [policy, setPolicy] = useState<PasswordPolicy | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    api
      .get<ApiResponse<PasswordPolicy>>('/auth/password-policy')
      .then((res) => setPolicy(res.data.data))
      .catch(() => setPolicy(null))
  }, [])

  const policyText = policy
    ? `至少 ${policy.passwordMinLength} 位，包含 ${policy.passwordMinCategories} 类字符（大写字母 / 小写字母 / 数字 / 特殊字符）`
    : ''

  const onFinish = async (values: RegisterForm) => {
    setLoading(true)
    try {
      const res = await api.post<ApiResponse<null>>('/auth/register', {
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
    <div
      style={{
        display: 'grid',
        placeItems: 'center',
        minHeight: '100vh',
        background: 'var(--bg)',
      }}
    >
      <Card style={{ width: 380, background: 'var(--surface)' }} title="注册云笔记账号">
        {policyText && (
          <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 16 }}>
            密码要求：{policyText}
          </div>
        )}
        <Form form={form} onFinish={onFinish} layout="vertical" autoComplete="off">
          <Form.Item
            label="用户名"
            name="username"
            rules={[
              { required: true, message: '请输入用户名' },
              { min: 3, max: 64, message: '用户名长度 3-64 个字符' },
            ]}
          >
            <Input placeholder="用户名" autoComplete="username" />
          </Form.Item>
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input placeholder="email@example.com" autoComplete="email" />
          </Form.Item>
          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password placeholder="密码" autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            label="确认密码"
            name="confirm"
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
            <Input.Password placeholder="再次输入密码" autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block icon={<UserPlus size={14} />}>
            注 册
          </Button>
        </Form>
        <div style={{ textAlign: 'center', fontSize: 13, marginTop: 16 }}>
          已有账号？<Link to="/login">去登录</Link>
        </div>
      </Card>
    </div>
  )
}
