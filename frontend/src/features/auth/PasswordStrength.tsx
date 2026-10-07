import { useEffect, useState } from 'react'

export type StrengthLevel = 0 | 1 | 2 | 3 | 4

export function assessPasswordStrength(password: string, minCategories = 3): StrengthLevel {
  if (!password) return 0
  let categories = 0
  if (/[a-z]/.test(password)) categories++
  if (/[A-Z]/.test(password)) categories++
  if (/\d/.test(password)) categories++
  if (/[^a-zA-Z0-9]/.test(password)) categories++
  if (categories === 0 || password.length < minCategories) return 0
  if (password.length < 8) return 1
  if (password.length < 12 || categories < 3) return 2
  if (password.length < 16 || categories < 4) return 3
  return 4
}

const LEVELS: { label: string; color: string; width: string }[] = [
  { label: '太弱', color: '#ef4444', width: '20%' },
  { label: '较弱', color: '#f59e0b', width: '40%' },
  { label: '一般', color: '#eab308', width: '60%' },
  { label: '良好', color: '#84cc16', width: '80%' },
  { label: '强', color: '#22c55e', width: '100%' },
]

export default function PasswordStrength({
  password,
  minCategories = 3,
}: {
  password: string
  minCategories?: number
}) {
  const [level, setLevel] = useState<StrengthLevel>(0)

  useEffect(() => {
    setLevel(assessPasswordStrength(password, minCategories))
  }, [password, minCategories])

  if (!password) return null
  const meta = LEVELS[level]
  return (
    <div className="password-strength" role="status" aria-live="polite">
      <div className="password-strength-bar">
        <div
          className="password-strength-fill"
          style={{ width: meta.width, background: meta.color }}
        />
      </div>
      <span className="password-strength-label" style={{ color: meta.color }}>
        {meta.label}
      </span>
    </div>
  )
}
