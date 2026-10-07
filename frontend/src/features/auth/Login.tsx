import { Button, Card, Form, Input, App } from 'antd'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useEffect, useRef, useState } from 'react'
import { LogIn, KeyRound, RefreshCw } from 'lucide-react'
import { api, extractError, tokenStore } from '../../shared/api/client.ts'
import { useCaptcha } from './captcha.ts'
import BrandPanel from '../../shared/ui/BrandPanel.tsx'

interface LoginForm {
  username: string
  password: string
  captchaCode?: string
}

export default function Login() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const redirectUri = searchParams.get('redirect_uri') || ''
  const { message } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [shaking, setShaking] = useState(false)
  const formRef = useRef<HTMLDivElement | null>(null)
  const captcha = useCaptcha()

  // 已登录会话 + 携带 redirect_uri：校验会话有效后直接回传令牌，免重复登录
  const autoReturnRef = useRef(false)
  useEffect(() => {
    if (!redirectUri || !tokenStore.access || autoReturnRef.current) return
    autoReturnRef.current = true
    let cancelled = false
    ;(async () => {
      try {
        const res = await api.get('/auth/me')
        if (cancelled || res.data.code !== 0) return
        const params = new URLSearchParams({
          access_token: tokenStore.access ?? '',
          refresh_token: tokenStore.refresh ?? '',
        })
        window.location.href = `${redirectUri}?${params.toString()}`
      } catch {
        // 会话失效：刷新令牌由 401 拦截器处理，用户重新登录后仍会跳回 redirect_uri
      }
    })()
    return () => {
      cancelled = true
    }
  }, [redirectUri])

  const triggerShake = () => {
    setShaking(false)
    requestAnimationFrame(() => setShaking(true))
  }

  const onFinish = async (values: LoginForm) => {
    setLoading(true)
    try {
      const res = await api.post('/auth/login', {
        username: values.username,
        password: values.password,
        captchaId: captcha.captchaId || undefined,
        captchaCode: captcha.code || undefined,
      })
      if (res.data.code === 0) {
        tokenStore.save(res.data.data.accessToken, res.data.data.refreshToken)
        message.success('登录成功')
        if (redirectUri) {
          const params = new URLSearchParams({
            access_token: res.data.data.accessToken,
            refresh_token: res.data.data.refreshToken,
          })
          window.location.href = `${redirectUri}?${params.toString()}`
          return
        }
        navigate(res.data.data.user.role === 'admin' ? '/admin' : '/', { replace: true })
        return
      }
      triggerShake()
      message.error(res.data.message)
    } catch (err) {
      triggerShake()
      message.error(extractError(err).message)
    } finally {
      setLoading(false)
      void captcha.reload()
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-layout">
        <BrandPanel title="安全的账号认证体系" subtitle="为企业内部系统提供可管理的登录安全策略与完整会话控制。" />
        <Card className="auth-card" bordered={false}>
          <div className="auth-brand-header">
            <span className="brand-mark">
              <KeyRound size={20} aria-hidden />
            </span>
            <span className="brand-name">账号登录</span>
          </div>
          <div ref={formRef} className={shaking ? 'form-shake' : undefined} onAnimationEnd={() => setShaking(false)}>
            <Form<LoginForm> layout="vertical" onFinish={onFinish} autoComplete="off" scrollToFirstError>
              <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
                <Input placeholder="用户名" size="large" autoComplete="username" />
              </Form.Item>
              <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
                <Input.Password placeholder="密码" size="large" autoComplete="current-password" />
              </Form.Item>
              {(captcha.image || captcha.loading) && (
                <Form.Item label="验证码" required style={{ marginBottom: 8 }}>
                  <div className="captcha-row">
                    <Input
                      size="large"
                      placeholder="输入验证码"
                      value={captcha.code}
                      onChange={(e) => captcha.setCode(e.target.value)}
                      maxLength={6}
                      autoComplete="off"
                    />
                    <Button
                      size="large"
                      icon={<RefreshCw size={16} style={{ transform: captcha.loading ? 'rotate(180deg)' : undefined, transition: 'transform 300ms ease' }} aria-hidden />}
                      loading={captcha.loading}
                      onClick={() => void captcha.reload()}
                      title="刷新验证码"
                    >
                      刷新
                    </Button>
                  </div>
                  <div style={{ marginTop: 8 }}>
                    {captcha.loading && !captcha.image ? (
                      <div className="captcha-skeleton" aria-label="验证码加载中" />
                    ) : (
                      <img
                        className="captcha-img"
                        src={captcha.image}
                        alt="图形验证码"
                        title="点击刷新"
                        onClick={() => void captcha.reload()}
                      />
                    )}
                  </div>
                </Form.Item>
              )}
              <Form.Item style={{ marginBottom: 8 }}>
                <Button type="primary" htmlType="submit" size="large" block loading={loading}>
                  <LogIn size={16} style={{ marginRight: 8 }} aria-hidden />
                  登 录
                </Button>
              </Form.Item>
            </Form>
          </div>
          <div className="auth-links">
            <Link to="/forgot-password">忘记密码？</Link>
            <Link to="/register">注册新账号</Link>
          </div>
        </Card>
      </div>
    </div>
  )
}
