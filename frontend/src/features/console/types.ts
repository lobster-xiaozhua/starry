// Console API response types — mirror the backend JSON contracts.

export interface PathSnapshot {
  path: string
  count: number
  avgLatencyMs: number
  errorRate: number
}

export interface StatusSnapshot {
  status: string
  count: number
}

export interface TrafficSnapshot {
  startedAt: string
  totalRequests: number
  totalErrors: number
  errorRate: number
  avgLatencyMs: number
  uptimeSeconds: number
  byPath: PathSnapshot[]
  byStatus: StatusSnapshot[]
}

export interface BootCheck {
  name: string
  status: string
  detail?: string
  count?: number
}

export interface ModuleStatus {
  name: string
  enabled: boolean
}

export interface BootResult {
  timestamp: string
  appEnv: string
  version: string
  checks: BootCheck[]
  modules: ModuleStatus[]
}

export interface HealthReport {
  boot: BootResult | null
  traffic: TrafficSnapshot
  db: boolean
  redis: boolean
}

export interface SettingsReport {
  appEnv: string
  modules: ModuleStatus[]
  driveMaxBytes: number
  driveQuotaBytes: number
  maxBodyBytes: number
  agentUrl: string
  embedUrl: string
}

export function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatUptime(sec: number): string {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

export function formatNumber(n: number): string {
  return n.toLocaleString('en-US')
}

export function errorRateColor(rate: number): string {
  if (rate < 0.01) return '#22c55e'
  if (rate <= 0.05) return '#f59e0b'
  return '#ef4444'
}

export function checkStatusColor(status: string): string {
  switch (status) {
    case 'ok':
      return 'success'
    case 'warn':
      return 'warning'
    case 'fail':
      return 'error'
    default:
      return 'default'
  }
}

export function checkStatusLabel(status: string): string {
  switch (status) {
    case 'ok':
      return '正常'
    case 'warn':
      return '警告'
    case 'fail':
      return '失败'
    default:
      return status
  }
}
