import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { ToastProvider } from './components/Toast.jsx'
import { ErrorProvider } from './context/ErrorContext.jsx'
import { AuthProvider, useAuth } from './context/AuthContext.jsx'
import { ErrorModal } from './components/ErrorModal.jsx'
import { AxiosInterceptor } from './components/AxiosInterceptor.jsx'
import { IconProvider } from './components/IconProvider.jsx'

// Pages
import Login from './pages/Login.jsx'
import Callback from './pages/Callback.jsx'
import Dashboard from './pages/Dashboard.jsx'
import Workspace from './pages/Workspace.jsx'
import TableView from './pages/TableView.jsx'
import Settings from './pages/Settings.jsx'

import './index.css'

function AppRoutes() {
  const { token, loading } = useAuth()

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  return (
    <Routes>
      <Route path="/" element={token ? <Navigate to="/dashboard" /> : <Login />} />
      <Route path="/callback" element={<Callback />} />
      <Route 
        path="/dashboard" 
        element={token ? <Dashboard /> : <Navigate to="/" />} 
      />
      <Route 
        path="/workspace/:workspaceId" 
        element={token ? <Workspace /> : <Navigate to="/" />} 
      />
      <Route 
        path="/workspace/:workspaceId/tables/:tableId" 
        element={token ? <TableView /> : <Navigate to="/" />} 
      />
      <Route 
        path="/workspace/:workspaceId/settings" 
        element={token ? <Settings /> : <Navigate to="/" />} 
      />
      <Route path="*" element={<Navigate to="/" />} />
    </Routes>
  )
}

function App() {
  return (
    <AuthProvider>
      <ErrorProvider>
        <IconProvider>
          <AxiosInterceptor />
          <ErrorModal />
          <BrowserRouter>
            <ToastProvider>
              <AppRoutes />
            </ToastProvider>
          </BrowserRouter>
        </IconProvider>
      </ErrorProvider>
    </AuthProvider>
  )
}

export default App
