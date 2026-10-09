import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Card,
  Col,
  Empty,
  Progress,
  Row,
  Spin,
  Statistic,
  Table,
  Tag,
  Tooltip,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  Activity,
  Database,
  Server,
  CheckCircle,
  AlertTriangle,
  AlertCircle,
  Clock,
  Zap,
  TrendingUp,
  Cpu,
  RefreshCw,
} from 'lucide-react'
import { api } from '../../shared/api/client.ts'
import { formatError } from '../../shared/lib/errors.ts'
import {
  type HealthReport,
  type TrafficSnapshot,
  type PathSnapshot,
  type BootCheck,
  formatUptime,
  formatNumber,
  errorRateColor,
  checkStatusColor,
  checkStatusLabel,
} from './types.ts'

const POLL_INTERVAL = 10_000

const STATUS_ICON: Record<string, React.ElementType> = {
  ok: CheckCircle,
  warn: AlertTriangle,
  fail: AlertCircle,
}

function StatusCard({
  label,
  ok,
  pending,
  icon: Icon,
}: {
  label: string
  ok: boolean
  pending: boolean
  icon: React.ElementType
}) {
  const color = pending ? '#94a3b8' : ok ? '#22c55e' : '#ef4444'
  return (
    <Card
      size="small"
      style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
      styles={{ body: { display: 'flex', alignItems: 'center', gap: 12 } }}
    >
      <div
        style={{
          width: 36,
          height: 36,
          borderRadius: 8,
          display: 'grid',
          placeItems: 'center',
          background: pending
            ? 'var(--accent-bg)'
            : ok
              ? 'rgba(34,197,94,0.1)'
              : 'rgba(239,68,68,0.1)',
          color,
          flexShrink: 0,
        }}
      >
        <Icon size={18} />
      </div>
      <div>
        <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>{label}</div>
        <div style={{ fontWeight: 600, color }}>
          {pending ? '检测中' : ok ? '正常' : '异常'}
        </div>
      </div>
    </Card>
  )
}

export default function HealthDashboard() {
  const [health, setHealth] = useState<HealthReport | null>(null)
  const [traffic, setTraffic] = useState<TrafficSnapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const loadHealth = useCallback(async () => {
    try {
      const res = await api.get<{ data: HealthReport }>('/console/health')
      setHealth(res.data.data)
      setTraffic(res.data.data.traffic)
      setError('')
    } catch (err) {
      setError(formatError(err, '获取健康数据失败'))
    } finally {
      setLoading(false)
    }
  }, [])

  const pollTraffic = useCallback(async () => {
    try {
      const res = await api.get<{ data: TrafficSnapshot }>('/console/traffic')
      setTraffic(res.data.data)
    } catch {
      // 静默失败，轮询不打扰用户
    }
  }, [])

  useEffect(() => {
    void loadHealth()
    timerRef.current = setInterval(() => void pollTraffic(), POLL_INTERVAL)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [loadHealth, pollTraffic])

  if (loading && !health) {
    return (
      <div style={{ textAlign: 'center', padding: '80px 0' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (error && !health) {
    return (
      <div style={{ padding: '40px', textAlign: 'center' }}>
        <AlertCircle size={32} style={{ color: '#ef4444', marginBottom: 12 }} />
        <div style={{ color: 'var(--text-secondary)' }}>{error}</div>
      </div>
    )
  }

  const boot = health?.boot
  const tr = traffic
  const errRate = tr ? tr.errorRate : 0
  const errPct = (errRate * 100).toFixed(2)

  const pathColumns: ColumnsType<PathSnapshot> = [
    {
      title: '路径',
      dataIndex: 'path',
      key: 'path',
      ellipsis: true,
      render: (path: string) => (
        <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{path}</span>
      ),
    },
    {
      title: '请求数',
      dataIndex: 'count',
      key: 'count',
      width: 90,
      render: (n: number) => formatNumber(n),
      sorter: (a, b) => a.count - b.count,
    },
    {
      title: '平均延迟',
      dataIndex: 'avgLatencyMs',
      key: 'avgLatencyMs',
      width: 100,
      render: (ms: number) => `${ms.toFixed(1)}ms`,
      sorter: (a, b) => a.avgLatencyMs - b.avgLatencyMs,
    },
    {
      title: '错误率',
      dataIndex: 'errorRate',
      key: 'errorRate',
      width: 90,
      render: (rate: number) => {
        const pct = (rate * 100).toFixed(2)
        return (
          <Tag style={{ color: errorRateColor(rate), borderColor: errorRateColor(rate) }}>
            {pct}%
          </Tag>
        )
      },
      sorter: (a, b) => a.errorRate - b.errorRate,
    },
  ]

  const sortedPaths = tr
    ? [...tr.byPath].sort((a, b) => b.count - a.count).slice(0, 15)
    : []

  const totalStatus = tr
    ? tr.byStatus.reduce((sum, s) => sum + s.count, 0)
    : 0

  const statusColors: Record<string, string> = {
    '2xx': '#22c55e',
    '3xx': '#38bdf8',
    '4xx': '#f59e0b',
    '5xx': '#ef4444',
    '1xx': '#94a3b8',
  }

  return (
    <div style={{ color: 'var(--text)' }}>
      {/* 服务状态 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} sm={6}>
          <StatusCard label="数据库" ok={health?.db ?? false} pending={!health} icon={Database} />
        </Col>
        <Col xs={12} sm={6}>
          <StatusCard label="Redis" ok={health?.redis ?? false} pending={!health} icon={Server} />
        </Col>
        <Col xs={12} sm={6}>
          <Card
            size="small"
            style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
            styles={{ body: { display: 'flex', alignItems: 'center', gap: 12 } }}
          >
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                display: 'grid',
                placeItems: 'center',
                background: 'var(--accent-bg)',
                color: 'var(--accent)',
                flexShrink: 0,
              }}
            >
              <Cpu size={18} />
            </div>
            <div>
              <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>运行环境</div>
              <div style={{ fontWeight: 600 }}>{boot?.appEnv ?? '—'}</div>
            </div>
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card
            size="small"
            style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
            styles={{ body: { display: 'flex', alignItems: 'center', gap: 12 } }}
          >
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                display: 'grid',
                placeItems: 'center',
                background: 'var(--accent-bg)',
                color: 'var(--accent)',
                flexShrink: 0,
              }}
            >
              <Activity size={18} />
            </div>
            <div>
              <div style={{ fontSize: 12, color: 'var(--text-secondary)' }}>自检状态</div>
              <div style={{ fontWeight: 600 }}>
                {boot
                  ? boot.checks.every((c) => c.status !== 'fail')
                    ? '通过'
                    : '有异常'
                  : '—'}
              </div>
            </div>
          </Card>
        </Col>
      </Row>

      {/* 实时流量统计 */}
      <Card
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <TrendingUp size={16} style={{ color: 'var(--accent)' }} />
            实时流量统计
            <Tooltip title={`每 ${POLL_INTERVAL / 1000}s 自动刷新`}>
              <RefreshCw size={13} style={{ color: 'var(--text-secondary)' }} />
            </Tooltip>
          </span>
        }
        style={{ background: 'var(--surface)', borderColor: 'var(--border)', marginBottom: 16 }}
      >
        <Row gutter={[24, 16]}>
          <Col xs={12} sm={6}>
            <Statistic
              title="总请求数"
              value={tr ? formatNumber(tr.totalRequests) : '—'}
              prefix={<Zap size={16} style={{ color: 'var(--accent)' }} />}
            />
          </Col>
          <Col xs={12} sm={6}>
            <Statistic
              title="错误率"
              value={tr ? `${errPct}%` : '—'}
              valueStyle={{ color: errorRateColor(errRate) }}
            />
          </Col>
          <Col xs={12} sm={6}>
            <Statistic
              title="平均延迟"
              value={tr ? tr.avgLatencyMs.toFixed(1) : '—'}
              suffix="ms"
            />
          </Col>
          <Col xs={12} sm={6}>
            <Statistic
              title="运行时间"
              value={tr ? formatUptime(tr.uptimeSeconds) : '—'}
              prefix={<Clock size={16} style={{ color: 'var(--text-secondary)' }} />}
            />
          </Col>
        </Row>
        <div style={{ marginTop: 12 }}>
          <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>
            错误率监控
          </div>
          <Progress
            percent={Math.min(100, errRate * 100)}
            showInfo={false}
            strokeColor={errorRateColor(errRate)}
            trailColor="var(--border)"
          />
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              fontSize: 11,
              color: 'var(--text-secondary)',
              marginTop: 2,
            }}
          >
            <span>0%</span>
            <span>1%</span>
            <span>5%</span>
            <span>100%</span>
          </div>
        </div>
      </Card>

      {/* 路径分布 + 状态码分布 */}
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={15}>
          <Card
            title="路径请求分布"
            size="small"
            style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
          >
            {sortedPaths.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无请求数据" />
            ) : (
              <Table<PathSnapshot>
                dataSource={sortedPaths}
                columns={pathColumns}
                rowKey="path"
                size="small"
                pagination={false}
                scroll={{ y: 320 }}
              />
            )}
          </Card>
        </Col>
        <Col xs={24} lg={9}>
          <Card
            title="状态码分布"
            size="small"
            style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
          >
            {!tr || tr.byStatus.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无数据" />
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                {tr.byStatus
                  .sort((a, b) => a.status.localeCompare(b.status))
                  .map((s) => {
                    const pct = totalStatus > 0 ? (s.count / totalStatus) * 100 : 0
                    const color = statusColors[s.status] ?? '#94a3b8'
                    return (
                      <div key={s.status}>
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            marginBottom: 4,
                          }}
                        >
                          <Tag style={{ color, borderColor: color }}>{s.status}</Tag>
                          <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>
                            {formatNumber(s.count)} ({pct.toFixed(1)}%)
                          </span>
                        </div>
                        <Progress
                          percent={pct}
                          showInfo={false}
                          strokeColor={color}
                          trailColor="var(--border)"
                          size="small"
                        />
                      </div>
                    )
                  })}
              </div>
            )}
          </Card>
        </Col>
      </Row>

      {/* 开机自检结果 */}
      {boot && (
        <Card
          title={
            <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <CheckCircle size={16} style={{ color: 'var(--accent)' }} />
              开机自检结果
              <span style={{ fontSize: 12, color: 'var(--text-secondary)', fontWeight: 400 }}>
                {new Date(boot.timestamp).toLocaleString('zh-CN')}
              </span>
            </span>
          }
          size="small"
          style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}
        >
          {boot.checks.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无自检数据" />
          ) : (
            <Row gutter={[12, 12]}>
              {boot.checks.map((check: BootCheck) => {
                const Icon = STATUS_ICON[check.status] ?? AlertCircle
                const color =
                  check.status === 'ok'
                    ? '#22c55e'
                    : check.status === 'warn'
                      ? '#f59e0b'
                      : '#ef4444'
                return (
                  <Col xs={24} sm={12} key={check.name}>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: 10,
                        padding: '10px 12px',
                        borderRadius: 8,
                        border: '1px solid var(--border)',
                        background: 'var(--bg)',
                      }}
                    >
                      <Icon size={16} style={{ color, flexShrink: 0, marginTop: 2 }} />
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <span style={{ fontWeight: 600, fontSize: 13 }}>{check.name}</span>
                          <Tag color={checkStatusColor(check.status)} style={{ fontSize: 11 }}>
                            {checkStatusLabel(check.status)}
                          </Tag>
                          {(check.count ?? 0) > 0 && (
                            <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>
                              数量: {check.count}
                            </span>
                          )}
                        </div>
                        {check.detail && (
                          <div
                            style={{
                              fontSize: 12,
                              color: 'var(--text-secondary)',
                              marginTop: 2,
                            }}
                          >
                            {check.detail}
                          </div>
                        )}
                      </div>
                    </div>
                  </Col>
                )
              })}
            </Row>
          )}
          {boot.version && (
            <div style={{ marginTop: 12, fontSize: 12, color: 'var(--text-secondary)' }}>
              版本: {boot.version}
            </div>
          )}
        </Card>
      )}
    </div>
  )
}
