import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { ToastProvider } from './components/Toast.jsx'
import { ErrorProvider } from './context/ErrorContext.jsx'
import { AuthProvider, useAuth } from './context/AuthContext.jsx'
import { ErrorModal } from './components/ErrorModal.jsx'
import { AxiosInterceptor } from './components/AxiosInterceptor.jsx'
import { IconProvider } from './components/IconProvider.jsx'
import { useState, useEffect } from 'react'
import axios from 'axios'
import { API_URL } from './constants'

// Pages
import Login from './pages/Login.jsx'
import Callback from './pages/Callback.jsx'
import Dashboard from './pages/Dashboard.jsx'
import Workspace from './pages/Workspace.jsx'
import TableView from './pages/TableView.jsx'
import Settings from './pages/Settings.jsx'
import InstanceAdmin from './pages/InstanceAdmin.jsx'
import InitialSetup from './pages/InitialSetup.jsx'

import './index.css'

// Componente para verificar setup antes de mostrar rutas protegidas
function SetupCheck({ children }) {
  const { token } = useAuth()
  const [setupStatus, setSetupStatus] = useState(null)
  const [checking, setChecking] = useState(true)
  const [retryCount, setRetryCount] = useState(0)
  const [hasError, setHasError] = useState(false)

  useEffect(() => {
    if (!token) {
      setChecking(false)
      return
    }

    const checkSetup = async () => {
      try {
        const res = await axios.get(`${API_URL}/setup/status`)
        setSetupStatus(res.data.data)
        setHasError(false)
        setChecking(false) // Éxito - dejar de mostrar loading
      } catch (err) {
        console.error('Error checking setup:', err)
        const newRetryCount = retryCount + 1
        setRetryCount(newRetryCount)
        
        // Después de 3 reintentos, mostrar error
        if (newRetryCount >= 3) {
          setHasError(true)
          setSetupStatus({ needs_setup: false }) // Evitar redirección loop
          setChecking(false) // Dejar de mostrar loading después de 3 intentos
        } else {
          // Reintentar en 2 segundos
          setTimeout(() => {
            checkSetup()
          }, 2000)
        }
      }
    }

    checkSetup()
  }, [token, retryCount])

  if (checking && retryCount < 3) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  // Si hay error persistente, mostrar mensaje de error
  if (hasError) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)', padding: '2rem' }}>
        <div style={{ textAlign: 'center', maxWidth: '400px' }}>
          <h2 style={{ marginBottom: '1rem', color: 'var(--danger)' }}>Error de Conexión</h2>
          <p style={{ marginBottom: '1.5rem', color: 'var(--text-secondary)' }}>
            No se pudo verificar el estado de la instancia después de varios intentos.
          </p>
          <button 
            onClick={() => {
              setRetryCount(0)
              setHasError(false)
              setChecking(true)
            }}
            className="btn btn-primary"
          >
            Reintentar
          </button>
        </div>
      </div>
    )
  }

  // Si no hay token, dejar que el componente hijo maneje (normalmente redirige a login)
  if (!token) {
    return children
  }

  // Si necesita setup y no estamos ya en la página de setup, redirigir
  if (setupStatus?.needs_setup && window.location.pathname !== '/setup') {
    return <Navigate to="/setup" replace />
  }

  return children
}

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
        path="/setup" 
        element={token ? <InitialSetup /> : <Navigate to="/" />} 
      />
      <Route 
        path="/dashboard" 
        element={
          <SetupCheck>
            {token ? <Dashboard /> : <Navigate to="/" />}
          </SetupCheck>
        } 
      />
      <Route 
        path="/workspace/:workspaceId" 
        element={
          <SetupCheck>
            {token ? <Workspace /> : <Navigate to="/" />}
          </SetupCheck>
        } 
      />
      <Route 
        path="/workspace/:workspaceId/tables/:tableId" 
        element={
          <SetupCheck>
            {token ? <TableView /> : <Navigate to="/" />}
          </SetupCheck>
        } 
      />
      <Route 
        path="/workspace/:workspaceId/settings" 
        element={
          <SetupCheck>
            {token ? <Settings /> : <Navigate to="/" />}
          </SetupCheck>
        } 
      />
      <Route 
        path="/admin/instance" 
        element={
          <SetupCheck>
            {token ? <InstanceAdmin /> : <Navigate to="/" />}
          </SetupCheck>
        } 
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
