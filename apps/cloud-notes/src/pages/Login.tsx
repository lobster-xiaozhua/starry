import { useState } from 'react'
import { Button, Card, Divider, Form, Input, message } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { ExternalLink, RefreshCw } from 'lucide-react'
import { api, ApiResponse, tokenStore, AuthUser, extractError } from '@shared/api'
import { useCaptcha } from '../api/captcha'

const LOGIN_SYSTEM_PORT = 5173
const CLOUD_NOTES_PORT = 5174

// 预览平台按 "{port}-{suffix}.monkeycode-ai.online" 子域映射端口，浏览器地址不含端口号。
// 需把当前 5174 子域重写成 5173 子域；本地开发则用 localhost:5173。
function buildLoginSystemUrl(callback: string): string {
  const { protocol, hostname } = window.location
  const match = hostname.match(/^(\d+)-(.*)$/)
  if (match && match[1] === String(CLOUD_NOTES_PORT)) {
    const host = `${LOGIN_SYSTEM_PORT}-${match[2]}`
    return `${protocol}//${host}/login?redirect_uri=${encodeURIComponent(callback)}`
  }
  const baseHost = hostname.replace(/:\d+$/, '')
  const callbackUrl = `${protocol}//${baseHost}:${CLOUD_NOTES_PORT}/oauth/callback`
  const loginHost = `${baseHost}:${LOGIN_SYSTEM_PORT}`
  return `${protocol}//${loginHost}/login?redirect_uri=${encodeURIComponent(callbackUrl)}`
}

export default function LoginPage() {
  const [loading, setLoading] = useState(false)
  const [form] = Form.useForm()
  const captcha = useCaptcha()
  const navigate = useNavigate()

  const handleLogin = async (values: {
    username: string
    password: string
  }) => {
    if (captcha.enabled && (!captcha.code.trim() || !captcha.captchaId)) {
      message.error('请输入验证码')
      return
    }
    setLoading(true)
    try {
      const res = await api.post<ApiResponse<{ accessToken: string; refreshToken: string; user: AuthUser }>>(
        '/auth/login',
        {
          username: values.username,
          password: values.password,
          captchaId: captcha.enabled ? captcha.captchaId : undefined,
          captchaCode: captcha.enabled ? captcha.code.trim() : undefined,
        },
      )
      if (res.data.code === 0 && res.data.data) {
        tokenStore.save(res.data.data.accessToken, res.data.data.refreshToken)
        navigate('/')
      }
    } catch (err) {
      message.error(extractError(err).message || '登录失败，请检查用户名密码')
      captcha.reload()
    }
    setLoading(false)
  }

  const handleOAuthRedirect = () => {
    const { protocol, hostname } = window.location
    const match = hostname.match(/^(\d+)-(.*)$/)
    if (match && match[1] === String(CLOUD_NOTES_PORT)) {
      const callback = `${protocol}//${CLOUD_NOTES_PORT}-${match[2]}/oauth/callback`
      window.location.href = buildLoginSystemUrl(callback)
      return
    }
    const baseHost = hostname.replace(/:\d+$/, '')
    const callback = `${protocol}//${baseHost}:${CLOUD_NOTES_PORT}/oauth/callback`
    window.location.href = buildLoginSystemUrl(callback)
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
        <Form form={form} onFinish={handleLogin} layout="vertical">
          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input placeholder="用户名" autoComplete="username" />
          </Form.Item>
          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password placeholder="密码" autoComplete="current-password" />
          </Form.Item>
          {captcha.enabled && (
            <Form.Item label="验证码" required>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <Input
                  placeholder="验证码"
                  value={captcha.code}
                  onChange={(e) => captcha.setCode(e.target.value)}
                  style={{ flex: 1 }}
                  maxLength={6}
                />
                <Button
                  loading={captcha.loading}
                  onClick={() => void captcha.reload()}
                  icon={<RefreshCw size={14} />}
                  title="点击刷新验证码"
                />
                {captcha.image && (
                  <img
                    src={captcha.image}
                    alt="验证码"
                    onClick={() => void captcha.reload()}
                    style={{ height: 32, cursor: 'pointer', borderRadius: 4 }}
                  />
                )}
              </div>
            </Form.Item>
          )}
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
        <Divider plain style={{ fontSize: 12, color: 'var(--text-secondary)' }} />
        <div style={{ textAlign: 'center', fontSize: 13 }}>
          没有账号？<Link to="/register">立即注册</Link>
        </div>
      </Card>
    </div>
  )
}
