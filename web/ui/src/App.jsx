import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Link, useNavigate, useParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import axios from 'axios'
import { ThemeToggle, Button, Badge } from './components/index.jsx'
import { PermissionMatrix } from './components/PermissionMatrix.jsx'
import { ToastProvider, notify } from './components/Toast.jsx'
import { ErrorProvider } from './context/ErrorContext.jsx'
import { ErrorModal } from './components/ErrorModal.jsx'
import { AxiosInterceptor } from './components/AxiosInterceptor.jsx'
import { IconProvider } from './components/IconProvider.jsx'
import horneroLogo from './assets/hornero solo.png'
import './index.css'

const API_URL = 'http://localhost:8080/api/v1'

function App() {
  const [token, setToken] = useState(localStorage.getItem('hornero_token'))
  const [user, setUser] = useState(null)

  useEffect(() => {
    if (token) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`
      axios.get(`${API_URL}/auth/me`)
        .then(res => setUser(res.data))
        .catch(() => {
          localStorage.removeItem('hornero_token')
          setToken(null)
        })
    }
  }, [token])

  const handleLogout = () => {
    localStorage.removeItem('hornero_token')
    setToken(null)
    setUser(null)
  }

  return (
    <ErrorProvider>
      <IconProvider>
        <AxiosInterceptor />
        <ErrorModal />
        <BrowserRouter>
          <ToastProvider>
            <Routes>
              <Route path="/" element={<Login />} />
              <Route path="/callback" element={<Callback onLogin={setToken} onUser={setUser} />} />
              <Route path="/dashboard" element={token ? <Dashboard user={user} onLogout={handleLogout} /> : <Login />} />
              <Route path="/workspace/:workspaceId" element={token ? <Workspace user={user} onLogout={handleLogout} /> : <Login />} />
              <Route path="/workspace/:workspaceId/tables/:tableId" element={token ? <TableView user={user} /> : <Login />} />
              <Route path="/workspace/:workspaceId/settings" element={token ? <Settings /> : <Login />} />
            </Routes>
          </ToastProvider>
        </BrowserRouter>
      </IconProvider>
    </ErrorProvider>
  )
}

/* ═══════════════════════════════════════════
   LOGIN PAGE
   ═══════════════════════════════════════════ */
function Login() {
  const handleLogin = () => {
    window.location.href = `${API_URL}/auth/oidc/login?redirect=${encodeURIComponent(window.location.origin + '/callback')}`
  }

  return (
    <div className="login-container">
      {/* Left panel — bold yellow brand */}
      <div className="login-left">
        <motion.div
          style={{ textAlign: 'center' }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
        >
          <img src={horneroLogo} alt="HorneroDB" style={{ width: '180px', height: '180px', objectFit: 'contain', marginBottom: '1rem', filter: 'drop-shadow(0 4px 6px rgba(0,0,0,0.1))' }} />
          <h1 style={{ fontSize: '3rem', fontWeight: 900, color: '#FFFFFF', letterSpacing: '-0.03em', marginBottom: '0.5rem' }}>
            HorneroDB
          </h1>
          <p style={{ fontSize: '1.125rem', color: 'rgba(255,255,255,0.7)', fontWeight: 500 }}>
            Tu base de datos personal
          </p>
        </motion.div>
      </div>

      {/* Right panel — login form */}
      <div className="login-right">
        <motion.div
          style={{ width: '100%', maxWidth: '400px' }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2 }}
        >
          <h2 style={{ fontSize: '2rem', fontWeight: 800, marginBottom: '0.5rem', letterSpacing: '-0.02em' }}>
            Bienvenido 👋
          </h2>
          <p style={{ color: 'var(--text-secondary)', marginBottom: '2rem', fontSize: '1rem' }}>
            Iniciá sesión para continuar
          </p>

          <button
            onClick={handleLogin}
            className="btn btn-primary btn-lg"
            style={{ width: '100%', gap: '0.75rem', fontSize: '1rem', padding: '0.875rem 1.5rem' }}
          >
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
            Iniciar sesión con PocketID
          </button>

          <p style={{ color: 'var(--text-muted)', fontSize: '0.8125rem', textAlign: 'center', marginTop: '2rem' }}>
            🔒 Acceso seguro con OIDC
          </p>
        </motion.div>
      </div>
    </div>
  )
}

/* ═══════════════════════════════════════════
   CALLBACK
   ═══════════════════════════════════════════ */
function Callback({ onLogin, onUser }) {
  const navigate = useNavigate()

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')

    if (token) {
      localStorage.setItem('hornero_token', token)
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`
      onLogin(token)
      axios.get(`${API_URL}/auth/me`)
        .then(res => {
          onUser(res.data)
          navigate('/dashboard')
        })
        .catch(() => navigate('/'))
    } else {
      navigate('/')
    }
  }, [])

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
      <div className="loading-spinner" />
    </div>
  )
}

/* ═══════════════════════════════════════════
   SIDEBAR
   ═══════════════════════════════════════════ */
function Sidebar({ user, onLogout, workspaceId }) {
  const navigate = useNavigate()
  const { pathname } = window.location

  const links = [
    { id: 'data', label: 'Datos', icon: 'M3 10h18M3 14h18m-9-4v8m-7 0h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z', path: `/workspace/${workspaceId}` },
    { id: 'settings', label: 'Configuración', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z', path: `/workspace/${workspaceId}/settings` },
  ]

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <Link to="/dashboard" className="sidebar-logo">
          <img src={horneroLogo} alt="Logo" style={{ width: '32px', height: '32px', objectFit: 'contain' }} />
          <span>HorneroDB</span>
        </Link>
      </div>

      <nav className="sidebar-nav">
        {links.map(link => (
          <button
            key={link.id}
            className={`sidebar-link ${pathname === link.path ? 'active' : ''}`}
            onClick={() => navigate(link.path)}
          >
            <svg style={{ width: '1.25rem', height: '1.25rem', flexShrink: 0 }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={link.icon} />
            </svg>
            {link.label}
          </button>
        ))}
      </nav>

      <div className="sidebar-footer">
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', padding: '0.5rem 0' }}>
          <div className="avatar">
            {user?.email?.charAt(0).toUpperCase() || 'U'}
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: '0.875rem', color: '#FFFFFF', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {user?.email || 'Usuario'}
            </div>
            <div style={{ fontSize: '0.75rem', color: '#666' }}>
              {user?.role || 'user'}
            </div>
          </div>
        </div>
        <button
          onClick={onLogout}
          className="sidebar-link"
          style={{ marginTop: '0.5rem', fontSize: '0.8125rem', color: '#888', padding: '0.5rem 1rem' }}
        >
          <svg style={{ width: '1rem', height: '1rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          Cerrar sesión
        </button>
      </div>
    </div>
  )
}

/* ═══════════════════════════════════════════
   DASHBOARD — workspace selector
   ═══════════════════════════════════════════ */
function Dashboard({ user, onLogout }) {
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(Array.isArray(res.data) ? res.data : []))
      .catch(() => setWorkspaces([]))
      .finally(() => setLoading(false))
  }, [])

  const handleCreate = async () => {
    if (!newName.trim()) return
    setCreating(true)
    try {
      await axios.post(`${API_URL}/workspaces`, {
        name: newName,
        slug: newName.toLowerCase().replace(/\s+/g, '-'),
        owner_id: user?.id || user?.user_id
      })
      setShowCreate(false)
      setNewName('')
      const res = await axios.get(`${API_URL}/workspaces`)
      setWorkspaces(Array.isArray(res.data) ? res.data : [])
    } catch (err) {
      console.error(err)
      notify('Error al crear workspace', 'error')
    } finally {
      setCreating(false)
    }
  }

  const renameWorkspace = async (id, currentName, e) => {
    e.stopPropagation()
    e.preventDefault()
    const newName = prompt('Nuevo nombre:', currentName)
    if (!newName || newName.trim() === currentName) return

    try {
      await axios.put(`${API_URL}/workspaces/${id}`, { name: newName })
      setWorkspaces(prev => prev.map(w => w.id === id ? { ...w, name: newName } : w))
      notify('Workspace renombrado', 'success')
    } catch (err) {
      console.error(err)
      notify('Error al renombrar', 'error')
    }
  }

  const deleteWorkspace = async (id, name, e) => {
    e.stopPropagation()
    e.preventDefault()
    if (!confirm(`¿Seguro que querés borrar el workspace "${name}"?\nSe perderán todas las tablas y datos.`)) return

    try {
      await axios.delete(`${API_URL}/workspaces/${id}`)
      setWorkspaces(prev => prev.filter(w => w.id !== id))
      notify('Workspace eliminado', 'success')
    } catch (err) {
      console.error(err)
      notify('Error al eliminar workspace', 'error')
    }
  }

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg)', display: 'flex', flexDirection: 'column' }}>
      {/* Top bar */}
      <header style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '1rem 2rem',
        borderBottom: 'var(--border-thick) solid var(--border-color)',
        background: 'var(--bg-elevated)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <img src={horneroLogo} alt="Logo" style={{ width: '40px', height: '40px', objectFit: 'contain' }} />
          <span style={{ fontSize: '1.25rem', fontWeight: 800, letterSpacing: '-0.02em' }}>HorneroDB</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <ThemeToggle />
          <div className="avatar" style={{ width: '2rem', height: '2rem', fontSize: '0.8rem' }}>
            {user?.email?.charAt(0).toUpperCase() || 'U'}
          </div>
          <button onClick={onLogout} className="btn btn-ghost btn-sm" style={{ fontSize: '0.8125rem' }}>
            Salir
          </button>
        </div>
      </header>

      {/* Content */}
      <div style={{ flex: 1, maxWidth: '960px', width: '100%', margin: '0 auto', padding: '3rem 2rem' }}>
        <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '2rem' }}>
            <div>
              <h1 style={{ fontSize: '2rem', fontWeight: 900, letterSpacing: '-0.03em', marginBottom: '0.25rem' }}>
                Tus Workspaces
              </h1>
              <p style={{ color: 'var(--text-secondary)', fontSize: '1rem' }}>
                Seleccioná un workspace para comenzar
              </p>
            </div>
            <Button onClick={() => setShowCreate(true)}>
              + Nuevo workspace
            </Button>
          </div>

          {workspaces.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon">📁</div>
              <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>
                Sin workspaces
              </h3>
              <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>
                Creá tu primer workspace para organizar tus datos
              </p>
              <Button onClick={() => setShowCreate(true)}>
                + Crear workspace
              </Button>
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '1.25rem' }}>
              {workspaces.map((ws, index) => (
                <motion.div
                  key={ws.id}
                  initial={{ opacity: 0, y: 20 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: index * 0.06 }}
                >
                  <div
                    className="card cursor-pointer"
                    onClick={() => navigate(`/workspace/${ws.id}`)}
                    style={{ minHeight: '140px', display: 'flex', flexDirection: 'column', position: 'relative' }}
                  >
                    <button
                      onClick={(e) => renameWorkspace(ws.id, ws.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '40px',
                        opacity: 0.6, hover: { opacity: 1 }, padding: '4px', zIndex: 10
                      }}
                      title="Renombrar"
                    >
                      <svg style={{ width: '1rem', height: '1rem', color: 'var(--text-secondary)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                      </svg>
                    </button>
                    <button
                      onClick={(e) => renameWorkspace(ws.id, ws.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '40px',
                        opacity: 0.6, hover: { opacity: 1 }, padding: '4px', zIndex: 10
                      }}
                      title="Renombrar"
                    >
                      <svg style={{ width: '1rem', height: '1rem', color: 'var(--text-secondary)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                      </svg>
                    </button>
                    <button
                      onClick={(e) => deleteWorkspace(ws.id, ws.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '10px',
                        opacity: 0.6, hover: { opacity: 1 }, padding: '4px', zIndex: 10
                      }}
                      title="Eliminar workspace"
                    >
                      <svg style={{ width: '1rem', height: '1rem', color: 'var(--danger)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.75rem' }}>
                      <div style={{
                        width: '3rem',
                        height: '3rem',
                        borderRadius: '12px',
                        background: 'var(--primary-light)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        border: '2px solid var(--primary)',
                        fontSize: '1.25rem',
                      }}>
                        {ws.name?.charAt(0)?.toUpperCase() || '📁'}
                      </div>
                      <div>
                        <div style={{ fontWeight: 800, fontSize: '1.0625rem' }}>{ws.name}</div>
                        <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                          @{ws.slug}
                        </div>
                      </div>
                    </div>
                    <div style={{ flex: 1 }} />
                    <div style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      borderTop: '1px solid var(--border-light)',
                      paddingTop: '0.75rem',
                      marginTop: '0.5rem',
                    }}>
                      <Badge variant="primary">Workspace</Badge>
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                        Abrir →
                      </span>
                    </div>
                  </div>
                </motion.div>
              ))}

              {/* Create new card */}
              <div
                className="card border-dashed cursor-pointer"
                onClick={() => setShowCreate(true)}
                style={{ minHeight: '140px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
              >
                <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                  <div style={{ fontSize: '2rem', marginBottom: '0.25rem' }}>+</div>
                  <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>Nuevo workspace</div>
                </div>
              </div>
            </div>
          )}
        </motion.div>
      </div>

      {/* Create Workspace Modal */}
      {showCreate && (
        <div className="modal-overlay" onClick={() => setShowCreate(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nuevo Workspace</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreate(false)} style={{ borderRadius: '8px' }}>
                <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">Nombre</label>
                <input
                  type="text"
                  className="form-input"
                  value={newName}
                  onChange={e => setNewName(e.target.value)}
                  placeholder="Mi Empresa"
                  autoFocus
                />
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreate(false)}>Cancelar</Button>
              <Button onClick={handleCreate} loading={creating} disabled={!newName.trim()}>Crear</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

/* ═══════════════════════════════════════════
   WORKSPACE
   ═══════════════════════════════════════════ */
function Workspace({ user, onLogout, workspaceProp }) {
  const { workspaceId } = useParams()
  const navigate = useNavigate()
  const [workspace, setWorkspace] = useState(workspaceProp || null)
  const [tables, setTables] = useState([])
  const [loading, setLoading] = useState(true)
  const [showCreateTable, setShowCreateTable] = useState(false)
  const [tableName, setTableName] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    const wsId = workspaceId || workspace?.id
    if (wsId) {
      axios.get(`${API_URL}/workspaces/${wsId}`)
        .then(res => setWorkspace(res.data))
        .catch(console.error)

      axios.get(`${API_URL}/workspaces/${wsId}/tables`)
        .then(res => setTables(res.data))
        .catch(() => setTables([]))
        .finally(() => setLoading(false))
    }
  }, [workspaceId, workspace?.id])

  const handleCreateTable = async () => {
    if (!tableName.trim()) return
    setCreating(true)
    try {
      const wsId = workspaceId || workspace?.id
      await axios.post(`${API_URL}/workspaces/${wsId}/tables`, {
        name: tableName,
        slug: tableName.toLowerCase().replace(/\s+/g, '_')
      })
      setShowCreateTable(false)
      setTableName('')
      const res = await axios.get(`${API_URL}/workspaces/${wsId}/tables`)
      setTables(res.data)
    } catch (err) {
      console.error(err)
      notify('Error al crear tabla', 'error')
    } finally {
      setCreating(false)
    }
  }

  const renameTable = async (id, currentName, e) => {
    e.stopPropagation()
    e.preventDefault()
    const newName = prompt('Nuevo nombre:', currentName)
    if (!newName || newName.trim() === currentName) return

    try {
      const wsId = workspaceId || workspace?.id
      await axios.put(`${API_URL}/workspaces/${wsId}/tables/${id}`, { name: newName })
      setTables(prev => prev.map(t => t.id === id ? { ...t, name: newName } : t))
      notify('Tabla renombrada', 'success')
    } catch (err) {
      console.error(err)
      notify('Error al renombrar', 'error')
    }
  }

  const deleteTable = async (id, name, e) => {
    e.stopPropagation()
    e.preventDefault()
    if (!confirm(`¿Seguro que querés borrar la tabla "${name}"?\nSe perderán todos los datos.`)) return

    try {
      const wsId = workspaceId || workspace?.id
      await axios.delete(`${API_URL}/workspaces/${wsId}/tables/${id}`)
      setTables(prev => prev.filter(t => t.id !== id))
      notify('Tabla eliminada', 'success')
    } catch (err) {
      console.error(err)
      notify('Error al eliminar tabla', 'error')
    }
  }

  const wsId = workspaceId || workspace?.id

  if (loading || !wsId) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex' }}>
      <Sidebar user={user} onLogout={onLogout} workspaceId={wsId} />

      <div className="main-content">
        <div className="main-body">
          <motion.div
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
          >
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '2rem' }}>
              <div>
                <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '0.25rem' }}>
                  {workspace?.name || 'Workspace'}
                </h1>
                <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', fontFamily: 'var(--font-mono)' }}>
                  @{workspace?.slug}
                </p>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <ThemeToggle />
              </div>
            </div>

            {/* Tables section header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
              <h2 style={{ fontSize: '1.125rem', fontWeight: 700 }}>Tablas</h2>
              <Button size="sm" onClick={() => setShowCreateTable(true)}>
                + Nueva tabla
              </Button>
            </div>

            {/* Tables grid */}
            {tables.length === 0 ? (
              <div className="empty-state">
                <div className="empty-icon">📋</div>
                <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>Sin tablas todavía</h3>
                <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>Creá tu primera tabla para organizar datos</p>
                <Button onClick={() => setShowCreateTable(true)}>
                  + Crear primera tabla
                </Button>
              </div>
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem' }}>
                {tables.map((table, index) => (
                  <motion.div
                    key={table.id}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: index * 0.05 }}
                  >
                    <div
                      className="card cursor-pointer"
                      onClick={() => navigate(`/workspace/${wsId}/tables/${table.id}`)}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem' }}>
                        <div style={{
                          width: '2.5rem',
                          height: '2.5rem',
                          borderRadius: '10px',
                          background: 'var(--primary-light)',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          border: '2px solid var(--primary)',
                        }}>
                          <svg style={{ width: '1.25rem', height: '1.25rem', color: 'var(--primary)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
                          </svg>
                        </div>
                        <div style={{ fontWeight: 700 }}>{table.name}</div>
                      </div>
                      <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                        @{table.slug}
                      </div>
                    </div>
                    <button
                      onClick={(e) => renameTable(table.id, table.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '40px',
                        padding: '4px', opacity: 0.6
                      }}
                      title="Renombrar"
                    >
                      <svg style={{ width: '1rem', height: '1rem', color: 'var(--text-secondary)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                      </svg>
                    </button>
                    <button
                      onClick={(e) => renameTable(table.id, table.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '40px',
                        padding: '4px', opacity: 0.6
                      }}
                      title="Renombrar"
                    >
                      <svg style={{ width: '1rem', height: '1rem', color: 'var(--text-secondary)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                      </svg>
                    </button>
                    <button
                      onClick={(e) => deleteTable(table.id, table.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '10px',
                        padding: '4px', opacity: 0.6
                      }}
                      title="Eliminar tabla"
                    >
                      <svg style={{ width: '1rem', height: '1rem', color: 'var(--danger)' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </motion.div>
                ))}

                {/* New table card */}
                <div
                  className="card border-dashed cursor-pointer"
                  onClick={() => setShowCreateTable(true)}
                  style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '120px' }}
                >
                  <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                    <div style={{ fontSize: '1.75rem', marginBottom: '0.25rem' }}>+</div>
                    <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>Nueva tabla</div>
                  </div>
                </div>
              </div>
            )}
          </motion.div>
        </div>
      </div>

      {/* Create Table Modal */}
      {showCreateTable && (
        <div className="modal-overlay" onClick={() => setShowCreateTable(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nueva Tabla</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateTable(false)} style={{ borderRadius: '8px' }}>
                <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">Nombre de la tabla</label>
                <input
                  type="text"
                  className="form-input"
                  value={tableName}
                  onChange={e => setTableName(e.target.value)}
                  placeholder="Ej: clientes, productos"
                  autoFocus
                />
                <p className="form-hint">
                  Se creará como: <code style={{ fontFamily: 'var(--font-mono)', background: 'var(--bg-surface)', padding: '0.125rem 0.375rem', borderRadius: '4px', fontSize: '0.8125rem' }}>{tableName.toLowerCase().replace(/\s+/g, '_') || '...'}</code>
                </p>
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateTable(false)}>Cancelar</Button>
              <Button onClick={handleCreateTable} loading={creating} disabled={!tableName.trim()}>Crear tabla</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

/* ═══════════════════════════════════════════
   TABLE VIEW
   ═══════════════════════════════════════════ */
function TableView() {
  const { workspaceId, tableId } = useParams()
  const navigate = useNavigate()

  const [table, setTable] = useState(null)
  const [columns, setColumns] = useState([])
  const [records, setRecords] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')
  const [showCreateRecord, setShowCreateRecord] = useState(false)
  const [showCreateColumn, setShowCreateColumn] = useState(false)
  const [newRecord, setNewRecord] = useState({})
  const [newColumnName, setNewColumnName] = useState('')
  const [newColumnType, setNewColumnType] = useState('text')

  useEffect(() => {
    loadData()
  }, [workspaceId, tableId])

  const loadData = async () => {
    try {
      const [tableRes, columnsRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`)
      ])
      setTable(tableRes.data)
      setColumns(columnsRes.data)

      const recordsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/data/${tableRes.data.slug}`)
      setRecords(recordsRes.data.data || [])
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const handleCreateRecord = async () => {
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, newRecord)
      setShowCreateRecord(false)
      setNewRecord({})
      loadData()
    } catch (err) {
      console.error(err)
      notify('Error al crear registro', 'error')
    }
  }

  const deleteRecord = async (id) => {
    if (!confirm('¿Borrar este registro?')) return
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${id}`)
      setRecords(prev => prev.filter(r => r.id !== id))
      notify('Registro eliminado', 'success')
    } catch (err) {
      console.error(err)
      notify('Error al eliminar registro', 'error')
    }
  }

  const handleCreateColumn = async () => {
    if (!newColumnName.trim()) return
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`, {
        name: newColumnName,
        slug: newColumnName.toLowerCase().replace(/\s+/g, '_'),
        field_type: newColumnType
      })
      setShowCreateColumn(false)
      setNewColumnName('')
      loadData()
    } catch (err) {
      console.error(err)
      notify('Error al crear columna', 'error')
    }
  }

  const fieldTypes = [
    { value: 'text', label: 'Texto' },
    { value: 'long_text', label: 'Texto largo' },
    { value: 'number', label: 'Número' },
    { value: 'boolean', label: 'Booleano' },
    { value: 'date', label: 'Fecha' },
    { value: 'email', label: 'Email' },
  ]

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex' }}>
      {/* Sidebar */}
      <div className="sidebar">
        <div className="sidebar-header">
          <Link to={`/workspace/${workspaceId}`} className="sidebar-logo">
            <img src={horneroLogo} alt="Logo" style={{ width: '32px', height: '32px', objectFit: 'contain' }} />
            <span>HorneroDB</span>
          </Link>
        </div>
        <nav className="sidebar-nav">
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}`)}>
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M3 14h18m-9-4v8m-7 0h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            Datos
          </button>
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}/settings`)}>
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            Configuración
          </button>
        </nav>
      </div>

      {/* Main content */}
      <div className="main-content">
        <div className="main-body">
          {/* Back button */}
          <button
            onClick={() => navigate(`/workspace/${workspaceId}`)}
            className="btn btn-ghost btn-sm"
            style={{ marginBottom: '1.25rem' }}
          >
            ← Volver
          </button>

          {/* Table header */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
            <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em' }}>{table?.name}</h1>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <Button size="sm" variant="secondary" onClick={() => setShowCreateColumn(true)}>
                + Columna
              </Button>
              <Button size="sm" onClick={() => setShowCreateRecord(true)}>
                + Registro
              </Button>
            </div>
          </div>

          {/* Tabs */}
          <div className="tabs">
            <button
              className={`tab ${activeTab === 'data' ? 'active' : ''}`}
              onClick={() => setActiveTab('data')}
            >
              📊 Datos
            </button>
            <button
              className={`tab ${activeTab === 'columns' ? 'active' : ''}`}
              onClick={() => setActiveTab('columns')}
            >
              📐 Columnas ({columns.length})
            </button>
          </div>

          {/* Data Tab */}
          {activeTab === 'data' && (
            <div className="table-container">
              {records.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">📝</div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>Sin datos</h3>
                  <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>Agregá el primer registro</p>
                  <Button onClick={() => setShowCreateRecord(true)}>
                    + Agregar registro
                  </Button>
                </div>
              ) : (
                <table className="table">
                  <thead>
                    <tr>
                      <th>ID</th>
                      {columns.map(col => (
                        <th key={col.id}>{col.name}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((record, i) => (
                      <tr key={i}>
                        <td>
                          <code style={{
                            fontSize: '0.75rem',
                            fontFamily: 'var(--font-mono)',
                            background: 'var(--bg-surface)',
                            padding: '0.25rem 0.5rem',
                            borderRadius: '4px',
                            border: '1px solid var(--border-light)',
                          }}>
                            {String(record.id)?.slice(0, 8)}
                          </code>
                        </td>
                        {columns.map(col => (
                          <td key={col.id}>{String(record[col.slug] || '-')}</td>
                        ))}
                        <td style={{ width: '50px' }}>
                          <button
                            onClick={() => deleteRecord(record.id)}
                            className="btn btn-ghost btn-sm"
                            style={{ color: 'var(--danger)', padding: '4px' }}
                            title="Borrar"
                          >
                            <svg style={{ width: '1rem', height: '1rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                            </svg>
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {/* Columns Tab */}
          {activeTab === 'columns' && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem' }}>
              {columns.map(col => (
                <div key={col.id} className="card">
                  <div style={{ fontWeight: 700, marginBottom: '0.25rem' }}>{col.name}</div>
                  <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '0.75rem' }}>
                    @{col.slug}
                  </div>
                  <Badge variant="primary">{col.field_type}</Badge>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Create Record Modal */}
      {showCreateRecord && (
        <div className="modal-overlay" onClick={() => setShowCreateRecord(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nuevo Registro</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateRecord(false)} style={{ borderRadius: '8px' }}>✕</button>
            </div>
            <div className="modal-body">
              {columns.map(col => (
                <div className="form-group" key={col.id}>
                  <label className="form-label">{col.name}</label>
                  <input
                    type={col.field_type === 'number' ? 'number' : 'text'}
                    className="form-input"
                    value={newRecord[col.slug] || ''}
                    onChange={e => setNewRecord({ ...newRecord, [col.slug]: e.target.value })}
                  />
                </div>
              ))}
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateRecord(false)}>Cancelar</Button>
              <Button onClick={handleCreateRecord}>Crear</Button>
            </div>
          </div>
        </div>
      )}

      {/* Create Column Modal */}
      {showCreateColumn && (
        <div className="modal-overlay" onClick={() => setShowCreateColumn(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nueva Columna</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateColumn(false)} style={{ borderRadius: '8px' }}>✕</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">Nombre</label>
                <input
                  type="text"
                  className="form-input"
                  value={newColumnName}
                  onChange={e => setNewColumnName(e.target.value)}
                  placeholder="Ej: telefono"
                  autoFocus
                />
              </div>
              <div className="form-group">
                <label className="form-label">Tipo de dato</label>
                <select
                  className="form-select"
                  value={newColumnType}
                  onChange={e => setNewColumnType(e.target.value)}
                >
                  {fieldTypes.map(ft => (
                    <option key={ft.value} value={ft.value}>{ft.label}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateColumn(false)}>Cancelar</Button>
              <Button onClick={handleCreateColumn} disabled={!newColumnName.trim()}>Crear</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

/* ═══════════════════════════════════════════
   SETTINGS
   ═══════════════════════════════════════════ */
function Settings() {
  const { workspaceId } = useParams()
  const navigate = useNavigate()
  const [activeSection, setActiveSection] = useState('roles')
  const [roles, setRoles] = useState([])
  const [tables, setTables] = useState([])
  const [apiKeys, setAPIKeys] = useState([])
  const [loading, setLoading] = useState(true)
  const [showPermissionMatrix, setShowPermissionMatrix] = useState(false)
  const [showCreateRole, setShowCreateRole] = useState(false)
  const [showCreateKey, setShowCreateKey] = useState(false)
  const [newRoleName, setNewRoleName] = useState('')
  const [newKeyName, setNewKeyName] = useState('')

  useEffect(() => {
    loadData()
  }, [workspaceId, activeSection])

  const loadData = async () => {
    setLoading(true)
    try {
      if (activeSection === 'roles') {
        const [rolesRes, tablesRes] = await Promise.all([
          axios.get(`${API_URL}/workspaces/${workspaceId}/roles`),
          axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
        ])
        setRoles(rolesRes.data)
        setTables(tablesRes.data)
      } else if (activeSection === 'keys') {
        const keysRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/keys`)
        setAPIKeys(keysRes.data)
      }
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const handleCreateRole = async () => {
    if (!newRoleName.trim()) return
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/roles`, {
        name: newRoleName,
        description: 'Nuevo rol',
        permissions: {}
      })
      setShowCreateRole(false)
      setNewRoleName('')
      loadData()
    } catch (err) {
      console.error(err)
      notify('Error al crear rol', 'error')
    }
  }

  const handleCreateKey = async () => {
    if (!newKeyName.trim()) return
    try {
      const res = await axios.post(`${API_URL}/workspaces/${workspaceId}/keys`, {
        name: newKeyName,
        expires_in_days: 365
      })
      notify(`API Key: ${res.data.key}`, 'success')
      setShowCreateKey(false)
      setNewKeyName('')
      loadData()
    } catch (err) {
      console.error(err)
      notify('Error al crear API key', 'error')
    }
  }

  const deleteAPIKey = async (id) => {
    if (confirm('¿Eliminar esta API key?')) {
      try {
        await axios.delete(`${API_URL}/workspaces/${workspaceId}/keys/${id}`)
        loadData()
      } catch (err) {
        console.error(err)
      }
    }
  }

  const savePermissions = async ({ roleId, permissions }) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/roles/${roleId}`, { permissions })
      loadData()
      notify('Permisos guardados', 'success')
    } catch (err) {
      console.error(err)
      notify('Error al guardar permisos', 'error')
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex' }}>
      {/* Sidebar */}
      <div className="sidebar">
        <div className="sidebar-header">
          <Link to={`/workspace/${workspaceId}`} className="sidebar-logo">
            <img src={horneroLogo} alt="Logo" style={{ width: '32px', height: '32px', objectFit: 'contain' }} />
            <span>HorneroDB</span>
          </Link>
        </div>
        <nav className="sidebar-nav">
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}`)}>
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M3 14h18m-9-4v8m-7 0h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            Datos
          </button>
          <button className="sidebar-link active">
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            Configuración
          </button>
        </nav>
      </div>

      {/* Main content */}
      <div className="main-content">
        <div className="main-body">
          <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '1.5rem' }}>
            ⚙️ Configuración
          </h1>

          {/* Tabs */}
          <div className="tabs">
            <button
              className={`tab ${activeSection === 'roles' ? 'active' : ''}`}
              onClick={() => setActiveSection('roles')}
            >
              🔐 Roles de Seguridad
            </button>
            <button
              className={`tab ${activeSection === 'keys' ? 'active' : ''}`}
              onClick={() => setActiveSection('keys')}
            >
              🔑 API Keys
            </button>
          </div>

          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem 0' }}>
              <div className="loading-spinner" />
            </div>
          ) : activeSection === 'roles' ? (
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                <p style={{ color: 'var(--text-secondary)' }}>Gestiona los roles y permisos de acceso</p>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <Button size="sm" variant="secondary" onClick={() => setShowPermissionMatrix(true)}>
                    ⚙️ Editar Permisos
                  </Button>
                  <Button size="sm" onClick={() => setShowCreateRole(true)}>
                    + Nuevo Rol
                  </Button>
                </div>
              </div>

              {roles.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">🔐</div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>Sin roles</h3>
                  <p style={{ color: 'var(--text-secondary)' }}>Creá roles para gestionar permisos</p>
                </div>
              ) : (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem' }}>
                  {roles.map(role => (
                    <div key={role.id} className="card">
                      <div style={{ fontWeight: 700, marginBottom: '0.25rem' }}>{role.name}</div>
                      <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        {role.description || 'Sin descripción'}
                      </div>
                      {role.is_default && (
                        <Badge variant="primary" className="" style={{ marginTop: '0.75rem' }}>Default</Badge>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                <p style={{ color: 'var(--text-secondary)' }}>Genera claves para acceso programático</p>
                <Button size="sm" onClick={() => setShowCreateKey(true)}>
                  + Nueva API Key
                </Button>
              </div>

              {apiKeys.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">🔑</div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>Sin API keys</h3>
                  <p style={{ color: 'var(--text-secondary)' }}>Creá una key para usar la API</p>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                  {apiKeys.map(key => (
                    <div key={key.id} className="card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                      <div>
                        <div style={{ fontWeight: 700 }}>{key.name}</div>
                        <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                          {key.prefix}...{key.key_hash?.slice(-8)}
                        </div>
                      </div>
                      <button
                        onClick={() => deleteAPIKey(key.id)}
                        className="btn btn-ghost btn-sm"
                        style={{ color: 'var(--danger)' }}
                      >
                        🗑️
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Create Role Modal */}
      {showCreateRole && (
        <div className="modal-overlay" onClick={() => setShowCreateRole(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nuevo Rol</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateRole(false)} style={{ borderRadius: '8px' }}>✕</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">Nombre del rol</label>
                <input
                  type="text"
                  className="form-input"
                  value={newRoleName}
                  onChange={e => setNewRoleName(e.target.value)}
                  placeholder="Ej: editor, viewer"
                  autoFocus
                />
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateRole(false)}>Cancelar</Button>
              <Button onClick={handleCreateRole} disabled={!newRoleName.trim()}>Crear</Button>
            </div>
          </div>
        </div>
      )}

      {/* Create API Key Modal */}
      {showCreateKey && (
        <div className="modal-overlay" onClick={() => setShowCreateKey(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nueva API Key</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateKey(false)} style={{ borderRadius: '8px' }}>✕</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">Nombre</label>
                <input
                  type="text"
                  className="form-input"
                  value={newKeyName}
                  onChange={e => setNewKeyName(e.target.value)}
                  placeholder="Ej: Mi App"
                  autoFocus
                />
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateKey(false)}>Cancelar</Button>
              <Button onClick={handleCreateKey} disabled={!newKeyName.trim()}>Crear</Button>
            </div>
          </div>
        </div>
      )}

      {/* Permission Matrix */}
      {showPermissionMatrix && (
        <PermissionMatrix
          workspaceId={workspaceId}
          tables={tables}
          roles={roles}
          onSave={savePermissions}
          onClose={() => setShowPermissionMatrix(false)}
        />
      )}
    </div>
  )
}

export default App
