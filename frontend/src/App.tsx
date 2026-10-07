import { ConfigProvider, App as AntApp, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { tokenStore } from './shared/api/client.ts'
import Login from './features/auth/Login.tsx'
import Register from './features/auth/Register.tsx'
import ForgotPassword from './features/auth/ForgotPassword.tsx'
import ResetPassword from './features/auth/ResetPassword.tsx'
import Chat from './features/chat/Chat.tsx'
import Home from './features/home/Home.tsx'
import AdminSettings from './features/admin/AdminSettings.tsx'
import AdminUserDetail from './features/admin/AdminUserDetail.tsx'
import NotesPage from './features/notes/Notes.tsx'
import EditorPage from './features/notes/Editor.tsx'
import KnowledgePage from './features/knowledge/Knowledge.tsx'
import BoardsPage from './features/boards/Boards.tsx'
import DrivePage from './features/drive/Drive.tsx'
import VaultPage from './features/vault/Vault.tsx'
import AppShell from './shared/ui/AppShell.tsx'
import { ThemeProvider, useTheme } from './theme.tsx'
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
