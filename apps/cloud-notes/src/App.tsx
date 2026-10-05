import { useEffect } from 'react'
import { ConfigProvider, theme as antdTheme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { BrowserRouter, Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import { tokenStore, onAuthChange } from '@shared/api'
import NotesPage from './pages/Notes'
import EditorPage from './pages/Editor'
import LoginPage from './pages/Login'
import OAuthCallbackPage from './pages/OAuthCallback'
import { ThemeProvider, useTheme } from './theme'
import type { ReactNode } from 'react'

function RequireAuth({ children }: { children: ReactNode }) {
  if (!tokenStore.access) return <Navigate to="/login" replace />
  return <>{children}</>
}

// 跨标签页鉴权同步：另一标签页/登录系统登出时，当前应用立即感知并跳转登录页。
function AuthSync() {
  const navigate = useNavigate()
  useEffect(() => {
    return onAuthChange((hasToken) => {
      if (!hasToken) {
        navigate('/login', { replace: true })
      }
    })
  }, [navigate])
  return null
}

function Root() {
  const { dark } = useTheme()
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: dark ? '#38bdf8' : '#0f172a',
          colorBgLayout: dark ? '#020617' : '#f8fafc',
          colorText: dark ? '#f1f5f9' : '#0f172a',
          colorTextSecondary: dark ? '#94a3b8' : '#64748b',
          borderRadius: 8,
        },
      }}
    >
      <BrowserRouter>
        <AuthSync />
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/oauth/callback" element={<OAuthCallbackPage />} />
          <Route
            path="/"
            element={
              <RequireAuth>
                <NotesPage />
              </RequireAuth>
            }
          />
          <Route
            path="/notes/:id"
            element={
              <RequireAuth>
                <EditorPage />
              </RequireAuth>
            }
          />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <Root />
    </ThemeProvider>
  )
}
