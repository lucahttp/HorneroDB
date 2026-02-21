import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { BrowserRouter, Routes, Route, Link, useNavigate, useParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import axios from 'axios'
import { LanguageSelector } from './components/LanguageSelector.jsx'
import { ThemeToggle, Button, Badge } from './components/index.jsx'
import { PermissionMatrix } from './components/PermissionMatrix.jsx'
import { DataTable } from './components/DataTable.jsx'
import { ToastProvider, notify } from './components/Toast.jsx'
import { ErrorProvider } from './context/ErrorContext.jsx'
import { ErrorModal } from './components/ErrorModal.jsx'
import { AxiosInterceptor } from './components/AxiosInterceptor.jsx'
import { IconProvider } from './components/IconProvider.jsx'
import SettingsUsers from './components/SettingsUsers'
import {
  Lock, Folder, ClipboardCheck, ReportColumns, RulerCombine,
  Settings as SettingsIcon, ShieldCheck, Key, Trash, EditPencil, Xmark,
  LogOut, Table2Columns, EmojiSingLeftNote, Group
} from 'iconoir-react'
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
  const { t } = useTranslation()
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
            {t('your_personal_db')}
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
          <h2 style={{ fontSize: '2rem', fontWeight: 800, marginBottom: '0.5rem', letterSpacing: '-0.02em', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            {t('welcome')} <EmojiSingLeftNote width="1.25em" height="1.25em" />
          </h2>
          <p style={{ color: 'var(--text-secondary)', marginBottom: '2rem', fontSize: '1rem' }}>
            {t('login_subtitle')}
          </p>

          <button
            onClick={handleLogin}
            className="btn btn-primary btn-lg"
            style={{ width: '100%', gap: '0.75rem', fontSize: '1rem', padding: '0.875rem 1.5rem' }}
          >
            <Lock width="1.25rem" height="1.25rem" />
            {t('login_button')}
          </button>

          <p style={{ color: 'var(--text-muted)', fontSize: '0.8125rem', textAlign: 'center', marginTop: '2rem', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.375rem' }}>
            <Lock width="0.875em" height="0.875em" /> {t('secure_access')}
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
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = window.location

  const links = [
    { id: 'data', label: t('data'), icon: <Table2Columns width="1.25rem" height="1.25rem" />, path: `/workspace/${workspaceId}` },
    { id: 'settings', label: t('settings'), icon: <SettingsIcon width="1.25rem" height="1.25rem" />, path: `/workspace/${workspaceId}/settings` },
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
            {link.icon}
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
              {user?.email || t('user_fallback')}
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
          <LogOut width="1rem" height="1rem" />
          {t('logout')}
        </button>
      </div>
    </div>
  )
}

/* ═══════════════════════════════════════════
   DASHBOARD — workspace selector
   ═══════════════════════════════════════════ */
function Dashboard({ user, onLogout }) {
  const { t } = useTranslation()
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
      notify(t('error_create_workspace'), 'error')
    } finally {
      setCreating(false)
    }
  }

  const renameWorkspace = async (id, currentName, e) => {
    e.stopPropagation()
    e.preventDefault()
    const newName = prompt(t('new_name_prompt'), currentName)
    if (!newName || newName.trim() === currentName) return

    try {
      await axios.put(`${API_URL}/workspaces/${id}`, { name: newName })
      setWorkspaces(prev => prev.map(w => w.id === id ? { ...w, name: newName } : w))
      notify(t('workspace_renamed'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rename'), 'error')
    }
  }

  const deleteWorkspace = async (id, name, e) => {
    e.stopPropagation()
    e.preventDefault()
    if (!confirm(t('confirm_delete_workspace', { name }))) return

    try {
      await axios.delete(`${API_URL}/workspaces/${id}`)
      setWorkspaces(prev => prev.filter(w => w.id !== id))
      notify(t('workspace_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_workspace'), 'error')
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
          <LanguageSelector />
          <ThemeToggle />
          <div className="avatar" style={{ width: '2rem', height: '2rem', fontSize: '0.8rem' }}>
            {user?.email?.charAt(0).toUpperCase() || 'U'}
          </div>
          <button onClick={onLogout} className="btn btn-ghost btn-sm" style={{ fontSize: '0.8125rem' }}>
            {t('logout')}
          </button>
        </div>
      </header>

      {/* Content */}
      <div style={{ flex: 1, maxWidth: '960px', width: '100%', margin: '0 auto', padding: '3rem 2rem' }}>
        <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '2rem' }}>
            <div>
              <h1 style={{ fontSize: '2rem', fontWeight: 900, letterSpacing: '-0.03em', marginBottom: '0.25rem' }}>
                {t('your_workspaces')}
              </h1>
              <p style={{ color: 'var(--text-secondary)', fontSize: '1rem' }}>
                {t('select_workspace_subtitle')}
              </p>
            </div>
            <Button onClick={() => setShowCreate(true)}>
              {t('new_workspace_button')}
            </Button>
          </div>

          {workspaces.length === 0 ? (
            <div className="empty-state">
              <div className="empty-icon"><Folder width="2rem" height="2rem" /></div>
              <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>
                {t('no_workspaces')}
              </h3>
              <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>
                {t('create_first_workspace_hint')}
              </p>
              <Button onClick={() => setShowCreate(true)}>
                {t('create_workspace_button')}
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
                      title={t('rename')}
                    >
                      <EditPencil width="1rem" height="1rem" style={{ color: 'var(--text-secondary)' }} />
                    </button>
                    <button
                      onClick={(e) => deleteWorkspace(ws.id, ws.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '10px',
                        opacity: 0.6, hover: { opacity: 1 }, padding: '4px', zIndex: 10
                      }}
                      title={t('delete_workspace')}
                    >
                      <Trash width="1rem" height="1rem" style={{ color: 'var(--danger)' }} />
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
                      <Badge variant="primary">{t('workspace_badge')}</Badge>
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                        {t('open')} →
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
                  <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>{t('new_workspace_button')}</div>
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
              <h3 className="modal-title">{t('new_workspace_title')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreate(false)} style={{ borderRadius: '8px' }}>
                <Xmark width="1.25rem" height="1.25rem" />
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('name')}</label>
                <input
                  type="text"
                  className="form-input"
                  value={newName}
                  onChange={e => setNewName(e.target.value)}
                  placeholder={t('workspace_placeholder')}
                  autoFocus
                />
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreate(false)}>{t('cancel')}</Button>
              <Button onClick={handleCreate} loading={creating} disabled={!newName.trim()}>{t('create')}</Button>
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
  const { t } = useTranslation()
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
      notify(t('error_create_table'), 'error')
    } finally {
      setCreating(false)
    }
  }

  const renameTable = async (id, currentName, e) => {
    e.stopPropagation()
    e.preventDefault()
    const newName = prompt(t('new_name_prompt'), currentName)
    if (!newName || newName.trim() === currentName) return

    try {
      const wsId = workspaceId || workspace?.id
      await axios.put(`${API_URL}/workspaces/${wsId}/tables/${id}`, { name: newName })
      setTables(prev => prev.map(t => t.id === id ? { ...t, name: newName } : t))
      notify(t('table_renamed'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rename'), 'error')
    }
  }

  const deleteTable = async (id, name, e) => {
    e.stopPropagation()
    e.preventDefault()
    if (!confirm(t('confirm_delete_table', { name }))) return

    try {
      const wsId = workspaceId || workspace?.id
      await axios.delete(`${API_URL}/workspaces/${wsId}/tables/${id}`)
      setTables(prev => prev.filter(t => t.id !== id))
      notify(t('table_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_table'), 'error')
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
                  {workspace?.name || t('workspace_badge')}
                </h1>
                <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', fontFamily: 'var(--font-mono)' }}>
                  @{workspace?.slug}
                </p>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <LanguageSelector />
                <ThemeToggle />
              </div>
            </div>

            {/* Tables section header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
              <h2 style={{ fontSize: '1.125rem', fontWeight: 700 }}>{t('tables')}</h2>
              <Button size="sm" onClick={() => setShowCreateTable(true)}>
                {t('new_table_button')}
              </Button>
            </div>

            {/* Tables grid */}
            {tables.length === 0 ? (
              <div className="empty-state">
                <div className="empty-icon"><ClipboardCheck width="2rem" height="2rem" /></div>
                <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('no_tables_yet')}</h3>
                <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>{t('create_first_table_hint')}</p>
                <Button onClick={() => setShowCreateTable(true)}>
                  {t('create_first_table_button')}
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
                    style={{ position: 'relative' }}
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
                          <Table2Columns width="1.25rem" height="1.25rem" style={{ color: 'var(--primary)' }} />
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
                      title={t('rename')}
                    >
                      <EditPencil width="1rem" height="1rem" style={{ color: 'var(--text-secondary)' }} />
                    </button>
                    <button
                      onClick={(e) => deleteTable(table.id, table.name, e)}
                      className="btn btn-ghost btn-sm"
                      style={{
                        position: 'absolute', top: '10px', right: '10px',
                        padding: '4px', opacity: 0.6
                      }}
                      title={t('delete_table')}
                    >
                      <Trash width="1rem" height="1rem" style={{ color: 'var(--danger)' }} />
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
                    <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>{t('new_table_card_text')}</div>
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
              <h3 className="modal-title">{t('new_table_title')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateTable(false)} style={{ borderRadius: '8px' }}>
                <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('table_name_label')}</label>
                <input
                  type="text"
                  className="form-input"
                  value={tableName}
                  onChange={e => setTableName(e.target.value)}
                  placeholder={t('table_name_placeholder')}
                  autoFocus
                />
                <p className="form-hint">
                  {t('will_be_created_as')} <code style={{ fontFamily: 'var(--font-mono)', background: 'var(--bg-surface)', padding: '0.125rem 0.375rem', borderRadius: '4px', fontSize: '0.8125rem' }}>{tableName.toLowerCase().replace(/\s+/g, '_') || '...'}</code>
                </p>
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateTable(false)}>{t('cancel')}</Button>
              <Button onClick={handleCreateTable} loading={creating} disabled={!tableName.trim()}>{t('create_table_button')}</Button>
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
  const { t } = useTranslation()
  const { workspaceId, tableId } = useParams()
  const navigate = useNavigate()

  const [table, setTable] = useState(null)
  const [columns, setColumns] = useState([])
  const [records, setRecords] = useState([])
  const [roles, setRoles] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')

  const [showConfig, setShowConfig] = useState(false)
  const [configRole, setConfigRole] = useState('')
  const [rolePermissions, setRolePermissions] = useState(null)

  useEffect(() => {
    loadData()
  }, [workspaceId, tableId])

  const loadData = async () => {
    try {
      const [tableRes, columnsRes, rolesRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/roles`)
      ])
      setTable(tableRes.data)
      setColumns(columnsRes.data)
      setRoles(rolesRes.data)

      const recordsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/data/${tableRes.data.slug}`)
      setRecords(recordsRes.data.data || [])
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const openConfigContext = () => {
    setConfigRole('')
    setRolePermissions(null)
    setShowConfig(true)
  }

  const handleRoleSelect = (roleId) => {
    setConfigRole(roleId)
    const role = roles.find(r => r.id === roleId)
    if (role && role.permissions && role.permissions[table.slug]) {
      setRolePermissions(role.permissions[table.slug])
    } else {
      setRolePermissions({ create: 'none', read: 'none', update: 'none', delete: 'none', columns: [] })
    }
  }

  const handleToggleColumnConfig = (colSlug) => {
    setRolePermissions(prev => {
      const currentCols = prev?.columns || []
      const isSelected = currentCols.includes(colSlug) || currentCols.includes('*')

      let nextCols
      if (currentCols.includes('*')) {
        nextCols = columns.map(c => c.slug).filter(s => s !== colSlug)
      } else if (isSelected) {
        nextCols = currentCols.filter(c => c !== colSlug)
      } else {
        nextCols = [...currentCols, colSlug]
      }
      return { ...prev, columns: nextCols }
    })
  }

  const handleSaveConfig = async () => {
    if (!configRole) return
    const role = roles.find(r => r.id === configRole)
    if (!role) return

    try {
      const updatedPermissions = {
        ...(role.permissions || {}),
        [table.slug]: rolePermissions
      }

      await axios.put(`${API_URL}/workspaces/${workspaceId}/roles/${role.id}`, {
        permissions: updatedPermissions
      })
      notify(t('permissions_saved') || 'Config saved', 'success')
      setShowConfig(false)
      loadData()
    } catch (err) {
      console.error(err)
      notify(t('error_save_permissions') || 'Error', 'error')
    }
  }

  const handleCreateRecord = async (data) => {
    await axios.post(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, data)
    loadData()
  }

  const deleteRecord = async (id) => {
    if (!confirm(t('confirm_delete_record'))) return
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${id}`)
      setRecords(prev => prev.filter(r => r.id !== id))
      notify(t('record_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_record'), 'error')
    }
  }

  const handleUpdateRecord = async (recordId, data) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${recordId}`, data)
      loadData()
    } catch (err) {
      console.error(err)
      notify(t('error_update_record'), 'error')
    }
  }

  const handleBulkDelete = async (ids) => {
    try {
      await Promise.all(
        ids.map(id => axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${id}`))
      )
      notify(t('records_deleted'), 'success')
      loadData()
    } catch (err) {
      console.error(err)
      notify(t('error_delete_records'), 'error')
    }
  }

  const handleCreateColumn = async (name, type) => {
    await axios.post(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`, {
      name,
      slug: name.toLowerCase().replace(/\s+/g, '_'),
      field_type: type
    })
    loadData()
  }

  const handleDeleteColumn = async (columnId) => {
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns/${columnId}`)
      loadData()
      notify(t('column_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_column'), 'error')
    }
  }

  const handleRenameColumn = async (columnId, newName) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns/${columnId}`, {
        name: newName
      })
      loadData()
      notify(t('column_renamed'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rename_column'), 'error')
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
            {t('data')}
          </button>
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}/settings`)}>
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            {t('settings')}
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
            ← {t('back')}
          </button>

          {/* Table header */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
            <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em' }}>{table?.name}</h1>
            <Button size="sm" variant="secondary" onClick={openConfigContext} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
              <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /></svg>
              {t('settings') || 'Config'}
            </Button>
          </div>

          {/* Tabs */}
          <div className="tabs">
            <button
              className={`tab ${activeTab === 'data' ? 'active' : ''}`}
              onClick={() => setActiveTab('data')}
            >
              📊 {t('data')}
            </button>
            <button
              className={`tab ${activeTab === 'columns' ? 'active' : ''}`}
              onClick={() => setActiveTab('columns')}
            >
              📐 {t('columns')} ({columns.length})
            </button>
          </div>

          {/* Data Tab */}
          {activeTab === 'data' && (
            <DataTable
              columns={columns}
              records={records}
              onCreateRecord={handleCreateRecord}
              onDeleteRecord={deleteRecord}
              onUpdateRecord={handleUpdateRecord}
              onBulkDelete={handleBulkDelete}
              onCreateColumn={handleCreateColumn}
              onDeleteColumn={handleDeleteColumn}
              onRenameColumn={handleRenameColumn}
            />
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

      {showConfig && (
        <div className="modal-overlay" onClick={() => setShowConfig(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">{t('table')} {t('settings') || 'Settings'}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowConfig(false)}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('role_label') || 'Role'}</label>
                <select className="form-select" value={configRole} onChange={e => handleRoleSelect(e.target.value)}>
                  <option value="">{t('select_role') || 'Select...'}</option>
                  {roles.map(r => <option key={r.id} value={r.id}>{r.name} {r.is_default ? `(${t('default') || 'Default'})` : ''}</option>)}
                </select>
              </div>

              {configRole && rolePermissions && (
                <div className="form-group" style={{ marginTop: '1rem' }}>
                  <label className="form-label">{t('visible_columns') || 'Visible Columns'}</label>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', background: 'var(--bg-surface)', padding: '1rem', borderRadius: '8px', maxHeight: '200px', overflowY: 'auto' }}>
                    {columns.map(col => {
                      const isChecked = rolePermissions.columns?.includes('*') || rolePermissions.columns?.includes(col.slug)
                      return (
                        <label key={col.id} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', fontSize: '0.875rem' }}>
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => handleToggleColumnConfig(col.slug)}
                          />
                          {col.name} <code style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{col.slug}</code>
                        </label>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowConfig(false)}>{t('cancel') || 'Cancel'}</Button>
              <Button onClick={handleSaveConfig} disabled={!configRole}>{t('save_changes') || 'Save'}</Button>
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
  const { t } = useTranslation()
  const { workspaceId } = useParams()
  const navigate = useNavigate()
  const [activeSection, setActiveSection] = useState('general')
  const [workspace, setWorkspace] = useState(null)
  const [rateLimit, setRateLimit] = useState(60)
  const [allowedOrigins, setAllowedOrigins] = useState('')
  const [savingGeneral, setSavingGeneral] = useState(false)
  const [roles, setRoles] = useState([])
  const [tables, setTables] = useState([])
  const [apiKeys, setAPIKeys] = useState([])
  const [loading, setLoading] = useState(true)

  const [showCreateRole, setShowCreateRole] = useState(false)
  const [showCreateKey, setShowCreateKey] = useState(false)
  const [newRoleName, setNewRoleName] = useState('')
  const [newKeyName, setNewKeyName] = useState('')
  const [newKeyRole, setNewKeyRole] = useState('')

  useEffect(() => {
    loadData()
  }, [workspaceId, activeSection])

  const loadData = async () => {
    setLoading(true)
    try {
      if (!workspace) {
        const wsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}`)
        setWorkspace(wsRes.data)
        const st = wsRes.data.settings || {}
        setRateLimit(st.rate_limit_per_minute ?? 60)
        setAllowedOrigins((st.allowed_origins || []).join(', '))
      }

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

  const handleSaveGeneral = async () => {
    setSavingGeneral(true)
    try {
      const originsArray = allowedOrigins.split(',')
        .map(o => o.trim())
        .filter(o => o.length > 0)

      const newSettings = {
        ...(workspace?.settings || {}),
        rate_limit_per_minute: Number(rateLimit),
        allowed_origins: originsArray
      }

      await axios.put(`${API_URL}/workspaces/${workspaceId}`, {
        settings: newSettings
      })
      notify(t('settings_saved') || 'Settings saved', 'success')
      const wsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}`)
      setWorkspace(wsRes.data)
    } catch (err) {
      console.error(err)
      notify(t('error_save_settings') || 'Error fetching', 'error')
    }
    setSavingGeneral(false)
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
      notify(t('error_create_role'), 'error')
    }
  }

  const handleCreateKey = async () => {
    if (!newKeyName.trim()) return
    try {
      const res = await axios.post(`${API_URL}/workspaces/${workspaceId}/keys`, {
        name: newKeyName,
        role_id: newKeyRole,
        expires_in_days: 365
      })
      notify(t('api_key_created', { key: res.data.key }), 'success')
      setShowCreateKey(false)
      setNewKeyName('')
      setNewKeyRole('')
      loadData()
    } catch (err) {
      console.error(err)
      notify(t('error_create_api_key'), 'error')
    }
  }

  const deleteAPIKey = async (id) => {
    if (confirm(t('confirm_delete_api_key'))) {
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
      notify(t('permissions_saved'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_save_permissions'), 'error')
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
            {t('data')}
          </button>
          <button className="sidebar-link active">
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            {t('settings')}
          </button>
        </nav>
      </div>

      {/* Main content */}
      <div className="main-content">
        <div className="main-body">
          <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '1.5rem', display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <SettingsIcon width="2rem" height="2rem" /> {t('settings')}
          </h1>

          {/* Tabs */}
          <div className="tabs">
            <button
              className={`tab ${activeSection === 'general' ? 'active' : ''}`}
              onClick={() => setActiveSection('general')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <SettingsIcon width="1rem" height="1rem" /> {t('general') || 'General'}
            </button>
            <button
              className={`tab ${activeSection === 'users' ? 'active' : ''}`}
              onClick={() => setActiveSection('users')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <Group width="1rem" height="1rem" /> {t('users')}
            </button>
            <button
              className={`tab ${activeSection === 'roles' ? 'active' : ''}`}
              onClick={() => setActiveSection('roles')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <ShieldCheck width="1rem" height="1rem" /> {t('security_roles')}
            </button>
            <button
              className={`tab ${activeSection === 'keys' ? 'active' : ''}`}
              onClick={() => setActiveSection('keys')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <Key width="1rem" height="1rem" /> {t('api_keys')}
            </button>
          </div>

          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '3rem 0' }}>
              <div className="loading-spinner" />
            </div>
          ) : activeSection === 'general' ? (
            <div className="card" style={{ maxWidth: '600px' }}>
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '1rem' }}>{t('workspace_security') || 'Workspace Security'}</h2>

              <div className="form-group">
                <label className="form-label">{t('rate_limit') || 'Rate Limit (Requests per minute per user)'}</label>
                <input
                  type="number"
                  className="form-input"
                  value={rateLimit}
                  onChange={e => setRateLimit(e.target.value)}
                  min="0"
                />
                <p className="form-hint" style={{ marginTop: '0.25rem' }}>{t('rate_limit_hint') || 'Set to 0 to disable. Useful for protecting public endpoints.'}</p>
              </div>

              <div className="form-group" style={{ marginTop: '1.5rem' }}>
                <label className="form-label">{t('allowed_origins') || 'Allowed Origins (CORS)'}</label>
                <textarea
                  className="form-input"
                  value={allowedOrigins}
                  onChange={e => setAllowedOrigins(e.target.value)}
                  placeholder="https://example.com, https://app.example.com"
                  rows={3}
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
                <p className="form-hint" style={{ marginTop: '0.25rem' }}>{t('allowed_origins_hint') || 'Comma separated domains. Use * to allow all (not recommended context). Empty allows any Origin without enforcing.'}</p>
              </div>

              <Button onClick={handleSaveGeneral} loading={savingGeneral} style={{ marginTop: '1rem' }}>
                {t('save_changes') || 'Save Changes'}
              </Button>
            </div>
          ) : activeSection === 'users' ? (
            <SettingsUsers workspaceId={workspaceId} roles={roles} notify={notify} />
          ) : activeSection === 'roles' ? (
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                <p style={{ color: 'var(--text-secondary)' }}>{t('manage_roles_desc')}</p>
                <Button size="sm" onClick={() => setShowCreateRole(true)}>
                  {t('new_role')}
                </Button>
              </div>

              {roles.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon"><ShieldCheck width="2rem" height="2rem" /></div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('no_roles')}</h3>
                  <p style={{ color: 'var(--text-secondary)' }}>{t('create_roles_hint')}</p>
                </div>
              ) : (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem' }}>
                  {roles.map(role => (
                    <div key={role.id} className="card">
                      <div style={{ fontWeight: 700, marginBottom: '0.25rem' }}>{role.name}</div>
                      <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        {role.description || t('no_description')}
                      </div>
                      {role.is_default && (
                        <Badge variant="primary" className="" style={{ marginTop: '0.75rem' }}>{t('default')}</Badge>
                      )}
                    </div>
                  ))}
                </div>
              )}

              {/* Inline Permission Matrix */}
              <PermissionMatrix
                workspaceId={workspaceId}
                tables={tables}
                roles={roles}
                onSave={savePermissions}
              />
            </div>
          ) : (
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                <p style={{ color: 'var(--text-secondary)' }}>{t('generate_keys_desc')}</p>
                <Button size="sm" onClick={() => setShowCreateKey(true)}>
                  {t('new_api_key')}
                </Button>
              </div>

              {apiKeys.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon"><Key width="2rem" height="2rem" /></div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('no_api_keys')}</h3>
                  <p style={{ color: 'var(--text-secondary)' }}>{t('create_key_hint')}</p>
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
                        <Trash width="1rem" height="1rem" />
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
              <h3 className="modal-title">{t('new_role')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateRole(false)} style={{ borderRadius: '8px' }}>
                <Xmark width="1.25rem" height="1.25rem" />
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('role_name')}</label>
                <input
                  type="text"
                  className="form-input"
                  value={newRoleName}
                  onChange={e => setNewRoleName(e.target.value)}
                  placeholder={t('role_placeholder')}
                  autoFocus
                />
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateRole(false)}>{t('cancel')}</Button>
              <Button onClick={handleCreateRole} disabled={!newRoleName.trim()}>{t('create')}</Button>
            </div>
          </div>
        </div>
      )}

      {/* Create API Key Modal */}
      {showCreateKey && (
        <div className="modal-overlay" onClick={() => setShowCreateKey(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">{t('new_api_key')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateKey(false)} style={{ borderRadius: '8px' }}>
                <Xmark width="1.25rem" height="1.25rem" />
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('name')}</label>
                <input
                  type="text"
                  className="form-input"
                  value={newKeyName}
                  onChange={e => setNewKeyName(e.target.value)}
                  placeholder={t('api_key_placeholder')}
                  autoFocus
                />
              </div>
              <div className="form-group">
                <label className="form-label">{t('role_label')}</label>
                <select
                  className="form-select"
                  value={newKeyRole}
                  onChange={e => setNewKeyRole(e.target.value)}
                >
                  <option value="">{t('select_role') || 'Sin rol (Solo lectura pública)'}</option>
                  {roles.map(r => (
                    <option key={r.id} value={r.id}>{r.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => setShowCreateKey(false)}>{t('cancel')}</Button>
              <Button onClick={handleCreateKey} disabled={!newKeyName.trim()}>{t('create')}</Button>
            </div>
          </div>
        </div>
      )}


    </div>
  )
}

export default App
