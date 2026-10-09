import { useEffect, useState } from 'react'
import { Card, Descriptions, Empty, Spin, Switch, Tag } from 'antd'
import { Settings, HardDrive, FileText, Cpu, ToggleRight } from 'lucide-react'
import { api } from '../../shared/api/client.ts'
import { formatError } from '../../shared/lib/errors.ts'
import { type SettingsReport, formatBytes } from './types.ts'

const MODULE_LABELS: Record<string, string> = {
  auth: '认证',
  admin: '管理',
  knowledge: '知识库',
  notes: '笔记',
  chat: '对话',
  boards: '任务看板',
  drive: '网盘',
  vault: '保险箱',
  console: '控制台',
}

export default function ProjectSettings() {
  const [settings, setSettings] = useState<SettingsReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .get<{ data: SettingsReport }>('/console/settings')
      .then((res) => {
        setSettings(res.data.data)
        setError('')
      })
      .catch((err) => {
        setError(formatError(err, '获取项目设置失败'))
      })
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '80px 0' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (error || !settings) {
    return (
      <div style={{ padding: '40px', textAlign: 'center' }}>
        <Empty description={error || '无法加载项目设置'} />
      </div>
    )
  }

  const envColor =
    settings.appEnv === 'production'
      ? 'red'
      : settings.appEnv === 'staging'
        ? 'orange'
        : 'blue'

  return (
    <div style={{ color: 'var(--text)' }}>
      {/* 配置概览 */}
      <Card
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Settings size={16} style={{ color: 'var(--accent)' }} />
            配置概览
          </span>
        }
        size="small"
        style={{ background: 'var(--surface)', borderColor: 'var(--border)', marginBottom: 16 }}
      >
        <Descriptions column={{ xs: 1, sm: 2, lg: 3 }} size="small" bordered>
          <Descriptions.Item label="运行环境">
            <Tag color={envColor}>{settings.appEnv}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Agent 服务地址">
            <span style={{ fontFamily: 'monospace', fontSize: 12 }}>
              {settings.agentUrl || '—'}
            </span>
          </Descriptions.Item>
          <Descriptions.Item label="Embed 服务地址">
            <span style={{ fontFamily: 'monospace', fontSize: 12 }}>
              {settings.embedUrl || '—'}
            </span>
          </Descriptions.Item>
          <Descriptions.Item label="网盘单文件上限">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <HardDrive size={14} style={{ color: 'var(--text-secondary)' }} />
              {formatBytes(settings.driveMaxBytes)}
            </span>
          </Descriptions.Item>
          <Descriptions.Item label="网盘用户配额">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <HardDrive size={14} style={{ color: 'var(--text-secondary)' }} />
              {formatBytes(settings.driveQuotaBytes)}
            </span>
          </Descriptions.Item>
          <Descriptions.Item label="请求体大小限制">
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <FileText size={14} style={{ color: 'var(--text-secondary)' }} />
              {formatBytes(settings.maxBodyBytes)}
            </span>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 模块启停状态 */}
      <Card
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <ToggleRight size={16} style={{ color: 'var(--accent)' }} />
            模块启停状态
            <span
              style={{
                fontSize: 12,
                color: 'var(--text-secondary)',
                fontWeight: 400,
              }}
            >
              （由环境变量控制，只读展示）
            </span>
          </span>
        }
        size="small"
        style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
      >
        {settings.modules.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无模块信息" />
        ) : (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
              gap: 12,
            }}
          >
            {settings.modules.map((m) => (
              <div
                key={m.name}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '10px 14px',
                  borderRadius: 8,
                  border: '1px solid var(--border)',
                  background: 'var(--bg)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Cpu size={15} style={{ color: 'var(--text-secondary)' }} />
                  <span style={{ fontWeight: 500 }}>
                    {MODULE_LABELS[m.name] ?? m.name}
                  </span>
                  <span
                    style={{
                      fontSize: 11,
                      color: 'var(--text-secondary)',
                      fontFamily: 'monospace',
                    }}
                  >
                    {m.name}
                  </span>
                </div>
                <Switch
                  checked={m.enabled}
                  disabled
                  checkedChildren="启用"
                  unCheckedChildren="禁用"
                  size="small"
                />
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
