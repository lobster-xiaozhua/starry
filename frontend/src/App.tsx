import { ConfigProvider, App as AntApp, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { tokenStore } from './api/client'
import Login from './pages/Login'
import Register from './pages/Register'
import ForgotPassword from './pages/ForgotPassword'
import ResetPassword from './pages/ResetPassword'
import Chat from './pages/Chat'
import Home from './pages/Home'
import AdminSettings from './pages/AdminSettings'
import AdminUserDetail from './pages/AdminUserDetail'
import NotesPage from './pages/Notes'
import EditorPage from './pages/Editor'
import KnowledgePage from './pages/Knowledge'
import BoardsPage from './pages/Boards'
import DrivePage from './pages/Drive'
import VaultPage from './pages/Vault'
import AppShell from './components/AppShell'
import { ThemeProvider, useTheme } from './theme'
import './styles.css'

const antdTheme = {
  token: {
    colorPrimary: '#0f172a',
    colorPrimaryHover: '#1e293b',
    colorPrimaryActive: '#1e293b',
    colorSuccess: '#22c55e',
    colorInfo: '#22c55e',
    colorWarning: '#f59e0b',
    colorError: '#ef4444',
    colorText: '#0f172a',
    colorTextSecondary: '#64748b',
    colorBgLayout: '#020617',
    borderRadius: 10,
    borderRadiusLG: 16,
    fontFamily:
      "'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
  },
}

function RequireAuth({ children, admin }: { children: JSX.Element; admin?: boolean }) {
  if (!tokenStore.access) return <Navigate to="/login" replace />
  if (admin) {
    try {
      const payload = JSON.parse(atob(tokenStore.access.split('.')[1] ?? ''))
      if (payload.role !== 'admin') return <Navigate to="/" replace />
    } catch {
      return <Navigate to="/login" replace />
    }
  }
  return children
}

function AppRoot() {
  const { dark } = useTheme()
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{ ...antdTheme, algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm }}
    >
      <AntApp>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<Login />} />
            <Route path="/register" element={<Register />} />
            <Route path="/forgot-password" element={<ForgotPassword />} />
            <Route path="/reset-password" element={<ResetPassword />} />
            <Route
              path="/"
              element={
                <RequireAuth>
                  <AppShell />
                </RequireAuth>
              }
            >
              <Route index element={<Home />} />
              <Route path="chat" element={<Chat mode="daily" />} />
              <Route path="agent" element={<Chat />} />
              <Route path="notes" element={<NotesPage />} />
              <Route path="notes/:id" element={<EditorPage />} />
              <Route path="knowledge" element={<KnowledgePage />} />
              <Route path="boards" element={<BoardsPage />} />
              <Route path="drive" element={<DrivePage />} />
              <Route path="vault" element={<VaultPage />} />
              <Route
                path="admin"
                element={
                  <RequireAuth admin>
                    <AdminSettings />
                  </RequireAuth>
                }
              />
              <Route
                path="admin/users/:id"
                element={
                  <RequireAuth admin>
                    <AdminUserDetail />
                  </RequireAuth>
                }
              />
            </Route>
            <Route path="*" element={<Navigate to="/login" replace />} />
          </Routes>
        </BrowserRouter>
      </AntApp>
    </ConfigProvider>
  )
}

export default function App() {
  return (
    <ThemeProvider>
      <AppRoot />
    </ThemeProvider>
  )
}
