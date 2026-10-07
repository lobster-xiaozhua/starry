// 共享 API 客户端从 @shared/api 导出，保持既有 import 路径不变。
export { api, tokenStore, extractError } from '@shared/api'
export type { ApiResponse, AuthUser } from '@shared/api'

export interface AuthSettings {
  passwordMinLength: number
  passwordRequireUpper: boolean
  passwordRequireLower: boolean
  passwordRequireDigit: boolean
  passwordRequireSpecial: boolean
  passwordMinCategories: number
  lockoutThreshold: number
  lockoutDurationMinutes: number
  accessTokenMinutes: number
  refreshTokenDays: number
  captchaEnabled: boolean
  updatedAt: string
}
