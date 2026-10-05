import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios'

export interface AuthUser {
  id: string
  username: string
  email: string
  role: string
  status: string
}

export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

const TOKEN_KEY = 'accessToken'
const REFRESH_KEY = 'refreshToken'

type AuthChangeHandler = (hasToken: boolean) => void
const authListeners = new Set<AuthChangeHandler>()

export function onAuthChange(handler: AuthChangeHandler): () => void {
  authListeners.add(handler)
  return () => {
    authListeners.delete(handler)
  }
}

function notifyAuthChange(hasToken: boolean) {
  authListeners.forEach((h) => h(hasToken))
}

export const tokenStore = {
  get access(): string | null {
    return localStorage.getItem(TOKEN_KEY)
  },
  get refresh(): string | null {
    return localStorage.getItem(REFRESH_KEY)
  },
  save(access: string, refresh?: string) {
    localStorage.setItem(TOKEN_KEY, access)
    if (refresh) localStorage.setItem(REFRESH_KEY, refresh)
    startProactiveRefresh()
  },
  clear() {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_KEY)
    notifyAuthChange(false)
  },
}

export const api = axios.create({ baseURL: '/api', timeout: 15000 })

api.interceptors.request.use((config) => {
  const token = tokenStore.access
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

let refreshing: Promise<string | null> | null = null

async function tryRefresh(): Promise<string | null> {
  const refresh = tokenStore.refresh
  if (!refresh) return null
  try {
    const res = await axios.post<ApiResponse<{ accessToken: string }>>(
      '/api/auth/refresh',
      { refreshToken: refresh },
      { timeout: 10000 },
    )
    if (res.data.code === 0 && res.data.data.accessToken) {
      tokenStore.save(res.data.data.accessToken)
      return res.data.data.accessToken
    }
    return null
  } catch {
    return null
  }
}

function decodeExp(token: string): number | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1] ?? ''))
    return payload.exp ? Number(payload.exp) : null
  } catch {
    return null
  }
}

let proactiveTimer: ReturnType<typeof setTimeout> | null = null

// 在 access token 过期前主动刷新，避免 SSE 断连和 REST 请求 401 后才反应式刷新的效率损失。
// 在 80% 生命周期或过期前 60 秒（取较早者）触发；刷新成功后由 tokenStore.save 自动安排下一轮。
export function startProactiveRefresh(): void {
  if (proactiveTimer) {
    clearTimeout(proactiveTimer)
    proactiveTimer = null
  }
  const token = tokenStore.access
  if (!token) return
  const exp = decodeExp(token)
  if (!exp) return
  const nowSec = Math.floor(Date.now() / 1000)
  const ttlSec = exp - nowSec
  if (ttlSec <= 0) return
  const refreshInMs = Math.min(ttlSec * 0.8, Math.max(ttlSec - 60, 0)) * 1000
  if (refreshInMs <= 0) return
  proactiveTimer = setTimeout(async () => {
    proactiveTimer = null
    const newToken = await tryRefresh()
    if (!newToken) {
      tokenStore.clear()
    }
  }, refreshInMs)
}

api.interceptors.response.use(
  (res) => res,
  async (error: AxiosError<ApiResponse>) => {
    const config = error.config as InternalAxiosRequestConfig & { _retried?: boolean }
    const url = config?.url ?? ''
    const isAuthEndpoint = ['/auth/login', '/auth/refresh', '/auth/captcha', '/auth/register'].some(
      (p) => url.includes(p),
    )
    if (error.response?.status === 401 && config && !config._retried && !isAuthEndpoint) {
      config._retried = true
      refreshing = refreshing ?? tryRefresh()
      const newToken = await refreshing
      refreshing = null
      if (newToken) {
        config.headers.Authorization = `Bearer ${newToken}`
        return api(config)
      }
      tokenStore.clear()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  },
)

export function extractError(err: unknown): { message: string; fields?: Record<string, string> } {
  if (axios.isAxiosError(err) && err.response?.data) {
    const body = err.response.data as ApiResponse & { data?: { fields?: Record<string, string> } }
    return { message: body.message || '请求失败', fields: body.data?.fields }
  }
  return { message: '网络异常，请稍后重试' }
}

export async function fetchMe(): Promise<AuthUser | null> {
  try {
    const res = await api.get<ApiResponse<{ user: AuthUser }>>('/auth/me')
    return res.data.code === 0 ? res.data.data.user : null
  } catch {
    return null
  }
}

// 跨标签页鉴权状态同步：另一标签页登出或令牌刷新时，当前标签页感知并响应。
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => {
    if (e.key === TOKEN_KEY) {
      if (e.newValue) {
        startProactiveRefresh()
      } else {
        notifyAuthChange(false)
      }
    }
  })
  startProactiveRefresh()
}
