import { Button, Card, Form, Input, Typography, Result, App } from 'antd'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { MailQuestion, Send, CheckCircle2 } from 'lucide-react'
import { api, extractError } from '../../shared/api/client.ts'
import BrandPanel from '../../shared/ui/BrandPanel.tsx'

interface ForgotForm {
  email: string
}

export default function ForgotPassword() {
  const { message } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [sent, setSent] = useState(false)

  const onFinish = async (values: ForgotForm) => {
    setLoading(true)
    try {
      const res = await api.post('/auth/forgot-password', values)
      if (res.data.code === 0) {
        setSent(true)
        message.info(res.data.data.message)
      }
    } catch (err) {
      message.error(extractError(err).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-layout">
        <BrandPanel title="找回密码" subtitle="通过注册邮箱安全重置你的登录密码，全程防枚举保护。" />
        <Card className="auth-card" bordered={false}>
          <div className="auth-brand-header">
            <span className="brand-mark">
              <MailQuestion size={20} aria-hidden />
            </span>
            <span className="brand-name">忘记密码</span>
          </div>
          {sent ? (
            <Result
              status="success"
              icon={<CheckCircle2 size={56} aria-hidden style={{ color: '#22c55e' }} />}
              title="重置请求已发送"
              subTitle="如果该邮箱已注册，重置链接已发送。请查收邮件并按提示完成重置（链接 30 分钟内有效）。"
            />
          ) : (
            <Form<ForgotForm> layout="vertical" onFinish={onFinish}>
              <Form.Item
                name="email"
                label="注册邮箱"
                rules={[
                  { required: true, message: '请输入注册邮箱' },
                  { type: 'email', message: '邮箱格式不正确' },
                ]}
              >
                <Input placeholder="email@example.com" size="large" autoComplete="email" />
              </Form.Item>
              <Form.Item style={{ marginBottom: 8 }}>
                <Button type="primary" htmlType="submit" size="large" block loading={loading}>
                  <Send size={16} style={{ marginRight: 8 }} aria-hidden />
                  发送重置链接
                </Button>
              </Form.Item>
            </Form>
          )}
          <div className="auth-links">
            <span />
            <Link to="/login">返回登录</Link>
          </div>
        </Card>
      </div>
    </div>
  )
}
