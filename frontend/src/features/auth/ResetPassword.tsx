import { Button, Card, Form, Input, Result, App } from 'antd'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useState } from 'react'
import { KeyRound, KeySquare, XCircle } from 'lucide-react'
import { api, extractError } from '../../shared/api/client.ts'
import BrandPanel from '../../shared/ui/BrandPanel.tsx'
import PasswordStrength from './PasswordStrength.tsx'

interface ResetForm {
  newPassword: string
  confirm: string
}

export default function ResetPassword() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [loading, setLoading] = useState(false)
  const token = params.get('token') ?? ''

  const onFinish = async (values: ResetForm) => {
    setLoading(true)
    try {
      const res = await api.post('/auth/reset-password', {
        token,
        newPassword: values.newPassword,
      })
      if (res.data.code === 0) {
        message.success('密码重置成功，请使用新密码登录')
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

  if (!token) {
    return (
      <div className="auth-page">
        <div className="auth-layout">
          <BrandPanel title="设置新密码" subtitle="重置流程完成后，该账号的所有既有会话将被吊销。" />
          <Card className="auth-card" bordered={false}>
            <Result
              status="warning"
              icon={<XCircle size={56} aria-hidden style={{ color: '#f59e0b' }} />}
              title="重置链接无效"
              subTitle="链接缺少重置令牌，请从邮件中的完整链接进入，或重新发起重置申请。"
              extra={
                <Link to="/forgot-password">
                  <Button type="primary" size="large">
                    重新申请重置
                  </Button>
                </Link>
              }
            />
          </Card>
        </div>
      </div>
    )
  }

  return (
    <div className="auth-page">
      <div className="auth-layout">
        <BrandPanel title="设置新密码" subtitle="重置完成后，该账号的所有既有会话将被吊销，需使用新密码登录。" />
        <Card className="auth-card" bordered={false}>
          <div className="auth-brand-header">
            <span className="brand-mark">
              <KeySquare size={20} aria-hidden />
            </span>
            <span className="brand-name">设置新密码</span>
          </div>
          <Form<ResetForm> layout="vertical" onFinish={onFinish} scrollToFirstError>
            <Form.Item
              name="newPassword"
              label="新密码"
              rules={[{ required: true, message: '请输入新密码' }]}
            >
              <Input.Password placeholder="新密码" size="large" autoComplete="new-password" />
            </Form.Item>
            <Form.Item noStyle shouldUpdate={(prev, cur) => prev.newPassword !== cur.newPassword}>
              {({ getFieldValue }) => <PasswordStrength password={getFieldValue('newPassword') ?? ''} />}
            </Form.Item>
            <Form.Item
              name="confirm"
              label="确认新密码"
              dependencies={['newPassword']}
              rules={[
                { required: true, message: '请再次输入新密码' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('newPassword') === value) return Promise.resolve()
                    return Promise.reject(new Error('两次输入的密码不一致'))
                  },
                }),
              ]}
            >
              <Input.Password placeholder="再次输入新密码" size="large" autoComplete="new-password" />
            </Form.Item>
            <Form.Item style={{ marginBottom: 8 }}>
              <Button type="primary" htmlType="submit" size="large" block loading={loading}>
                <KeyRound size={16} style={{ marginRight: 8 }} aria-hidden />
                重置密码
              </Button>
            </Form.Item>
            <div className="auth-links">
              <span />
              <Link to="/login">返回登录</Link>
            </div>
          </Form>
        </Card>
      </div>
    </div>
  )
}
