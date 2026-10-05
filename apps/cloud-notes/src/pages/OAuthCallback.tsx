import { useEffect, useRef } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Card, Spin, message } from 'antd'

export default function OAuthCallbackPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const handled = useRef(false)

  useEffect(() => {
    if (handled.current) return
    handled.current = true

    const access = params.get('access_token')
    const refresh = params.get('refresh_token')

    if (access && refresh) {
      localStorage.setItem('accessToken', access)
      localStorage.setItem('refreshToken', refresh)
      message.success('登录成功')
      navigate('/', { replace: true })
    } else {
      message.error('登录回调缺少令牌，请重新登录')
      navigate('/login', { replace: true })
    }
  }, [params, navigate])

  return (
    <div
      style={{
        display: 'grid',
        placeItems: 'center',
        height: '100vh',
        background: 'var(--bg)',
      }}
    >
      <Card style={{ width: 340, textAlign: 'center', background: 'var(--surface)' }}>
        <Spin />
        <p style={{ marginTop: 16, color: 'var(--text-secondary)' }}>
          正在完成登录，请稍候...
        </p>
      </Card>
    </div>
  )
}