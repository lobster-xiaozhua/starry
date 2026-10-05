import { useState } from 'react'
import { Button, Card, Divider, Form, Input, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { LogIn, ExternalLink } from 'lucide-react'
import { api, ApiResponse, tokenStore, AuthUser } from '@shared/api'

const LOGIN_SYSTEM_PORT = 5173

export default function LoginPage() {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleLogin = async (values: {
    username: string
    password: string
    captchaId?: string
    captchaCode?: string
  }) => {
    setLoading(true)
    try {
      const res = await api.post<ApiResponse<{ accessToken: string; refreshToken: string; user: AuthUser }>>(
        '/auth/login',
        values,
      )
      if (res.data.code === 0 && res.data.data) {
        tokenStore.save(res.data.data.accessToken, res.data.data.refreshToken)
        navigate('/')
      }
    } catch {
      message.error('登录失败，请检查用户名密码')
    }
    setLoading(false)
  }

  const handleOAuthRedirect = () => {
    const protocol = window.location.protocol
    const host = window.location.hostname
    const callback = `${protocol}//${host}:5174/oauth/callback`
    const url = `${protocol}//${host}:${LOGIN_SYSTEM_PORT}/login?redirect_uri=${encodeURIComponent(callback)}`
    window.location.href = url
  }

  return (
    <div
      style={{
        display: 'grid',
        placeItems: 'center',
        height: '100vh',
        background: 'var(--bg)',
      }}
    >
      <Card style={{ width: 360, background: 'var(--surface)' }} title="云笔记登录">
        <Form onFinish={handleLogin} layout="vertical">
          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input placeholder="用户名" />
          </Form.Item>
          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password placeholder="密码" />
          </Form.Item>
          <Form.Item label="验证码">
            <div style={{ display: 'flex', gap: 8 }}>
              <Form.Item name="captchaId" noStyle>
                <Input placeholder="验证码ID" style={{ flex: 1 }} />
              </Form.Item>
              <Form.Item name="captchaCode" noStyle>
                <Input placeholder="验证码" style={{ flex: 1 }} />
              </Form.Item>
            </div>
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form>
        <Divider plain style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
          或
        </Divider>
        <Button block icon={<ExternalLink size={14} />} onClick={handleOAuthRedirect}>
          跳转到登录系统登录
        </Button>
      </Card>
    </div>
  )
}
