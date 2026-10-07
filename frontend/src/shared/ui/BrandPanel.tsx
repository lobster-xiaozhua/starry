import { Lock, ShieldCheck, EyeOff, Fingerprint } from 'lucide-react'

const FEATURES = [
  {
    icon: ShieldCheck,
    title: '多层防护',
    desc: 'JWT 双令牌、登录锁定与图形验证码，防御暴力破解',
  },
  {
    icon: Lock,
    title: '密码策略',
    desc: '管理员可配置复杂度、失败锁定阈值与令牌有效期',
  },
  {
    icon: EyeOff,
    title: '隐私优先',
    desc: 'bcrypt 哈希存储密码，重置流程防账号枚举',
  },
  {
    icon: Fingerprint,
    title: '即时会话控制',
    desc: '登出与重置密码即刻吊销全部有效会话',
  },
]

export default function BrandPanel({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="brand-panel">
      <span className="panel-badge">
        <ShieldCheck size={14} aria-hidden />
        企业级认证
      </span>
      <h2>{title}</h2>
      <p className="panel-sub">{subtitle}</p>
      <ul className="feature-list">
        {FEATURES.map(({ icon: Icon, title: t, desc }) => (
          <li key={t}>
            <span className="feature-icon">
              <Icon size={18} aria-hidden />
            </span>
            <div>
              <strong>{t}</strong>
              {desc}
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}
