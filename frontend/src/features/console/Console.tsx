import { useState } from 'react'
import { Tabs } from 'antd'
import { Activity, Settings, Users } from 'lucide-react'
import HealthDashboard from './HealthDashboard.tsx'
import ProjectSettings from './ProjectSettings.tsx'
import AdminSettings from '../admin/AdminSettings.tsx'

type TabKey = 'health' | 'settings' | 'users'

export default function Console() {
  const [activeTab, setActiveTab] = useState<TabKey>('health')

  const items = [
    {
      key: 'health',
      label: (
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Activity size={15} />
          系统健康
        </span>
      ),
      children: <HealthDashboard />,
    },
    {
      key: 'settings',
      label: (
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Settings size={15} />
          项目设置
        </span>
      ),
      children: <ProjectSettings />,
    },
    {
      key: 'users',
      label: (
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <Users size={15} />
          用户与安全
        </span>
      ),
      children: <AdminSettings embedded />,
    },
  ]

  return (
    <div
      style={{
        maxWidth: 1200,
        margin: '0 auto',
        padding: '28px 24px 48px',
        color: 'var(--text)',
      }}
    >
      <div style={{ marginBottom: 20 }}>
        <div style={{ fontSize: 22, fontWeight: 800 }}>控制台</div>
        <div style={{ color: 'var(--text-secondary)', marginTop: 4 }}>
          系统健康监控、项目配置与用户管理
        </div>
      </div>
      <Tabs
        activeKey={activeTab}
        onChange={(k) => setActiveTab(k as TabKey)}
        items={items}
        size="large"
        destroyOnHidden
      />
    </div>
  )
}
