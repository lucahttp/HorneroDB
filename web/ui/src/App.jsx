import { useState, useEffect, useRef } from 'react'
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
        .then(res => setUser(res.data.data))
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
              <Route path="/workspace/:workspaceId/tables/:tableId" element={token ? <TableView user={user} onLogout={handleLogout} /> : <Login />} />
              <Route path="/workspace/:workspaceId/settings" element={token ? <Settings user={user} onLogout={handleLogout} /> : <Login />} />
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
          onUser(res.data.data)
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
function TopNavbar({ user, onLogout, workspaceId }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = window.location

  const links = workspaceId ? [
    { id: 'data', label: t('data') || 'Data', icon: <Table2Columns width="1rem" height="1rem" />, path: `/workspace/${workspaceId}` },
    { id: 'settings', label: t('settings') || 'Settings', icon: <SettingsIcon width="1rem" height="1rem" />, path: `/workspace/${workspaceId}/settings` },
  ] : []

  return (
    <header className="top-navbar-global" style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '1rem 2rem',
      borderBottom: 'var(--border-thick) solid var(--border-color)',
      background: 'var(--bg-elevated)',
      position: 'sticky',
      top: 0,
      zIndex: 100,
      gap: '1rem',
      flexWrap: 'wrap'
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '2rem', flexWrap: 'wrap' }}>
        <Link to="/dashboard" style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', textDecoration: 'none', color: 'inherit' }}>
          <img src={horneroLogo} alt="Logo" style={{ width: '32px', height: '32px', objectFit: 'contain' }} />
          <span style={{ fontSize: '1.25rem', fontWeight: 800, letterSpacing: '-0.02em', display: 'block' }}>HorneroDB</span>
        </Link>

        {links.length > 0 && (
          <nav style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            {links.map(link => {
              const isActive = pathname === link.path || pathname.startsWith(link.path + '/settings') && link.id === 'settings'
              return (
                <button
                  key={link.id}
                  onClick={() => navigate(link.path)}
                  className={`btn btn-sm ${isActive ? 'btn-primary' : 'btn-ghost'}`}
                  style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderRadius: 'var(--radius-pill)' }}
                >
                  {link.icon}
                  {link.label}
                </button>
              )
            })}
          </nav>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
        <LanguageSelector />
        <ThemeToggle />
        <div className="avatar" style={{ width: '2rem', height: '2rem', fontSize: '0.8rem', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          {user?.email?.charAt(0).toUpperCase() || 'U'}
        </div>
        <button onClick={onLogout} className="btn btn-ghost btn-sm" style={{ padding: '0.375rem 0.5rem' }} title={t('logout') || 'Logout'}>
          <LogOut width="1rem" height="1rem" />
        </button>
      </div>
    </header>
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
  const importInputRef = useRef(null)

  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(Array.isArray(res.data.data) ? res.data.data : []))
      .catch(() => setWorkspaces([]))
      .finally(() => setLoading(false))
  }, [])

  const handleCreate = async () => {
    if (!newName.trim()) return
    const ownerId = user?.id || user?.user_id
    if (!ownerId) {
      notify(t('error_create_workspace') + ': User ID missing', 'error')
      return
    }

    setCreating(true)
    try {
      await axios.post(`${API_URL}/workspaces`, {
        name: newName,
        slug: newName.toLowerCase().replace(/\s+/g, '-'),
        owner_id: ownerId
      })
      setShowCreate(false)
      setNewName('')
      const res = await axios.get(`${API_URL}/workspaces`)
      setWorkspaces(Array.isArray(res.data.data) ? res.data.data : [])
    } catch (err) {
      console.error('Workspace Create Error:', err)
      // notify is already called by AxiosInterceptor
    } finally {
      setCreating(false)
    }
  }

  const handleImportWorkspace = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    const ownerId = user?.id || user?.user_id
    if (!ownerId) {
      notify(t('error_import_workspace') || 'User ID missing', 'error')
      return
    }

    const reader = new FileReader()
    reader.onload = async (event) => {
      try {
        const jsonDump = JSON.parse(event.target.result)
        setLoading(true)
        await axios.post(`${API_URL}/workspaces/import`, jsonDump)
        
        notify(t('workspace_imported') || 'Workspace imported successfully', 'success')
        const res = await axios.get(`${API_URL}/workspaces`)
        setWorkspaces(Array.isArray(res.data.data) ? res.data.data : [])
      } catch (err) {
        console.error('Import Workspace Error:', err)
        notify(t('error_import_workspace') || 'Error importing workspace', 'error')
      } finally {
        if (importInputRef.current) importInputRef.current.value = ''
        setLoading(false)
      }
    }
    reader.readAsText(file)
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
      console.error('Delete Workspace Error:', err)
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
      {/* Top bar (Global) */}
      <TopNavbar user={user} onLogout={onLogout} />

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
            <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
              <input 
                type="file" 
                ref={importInputRef} 
                style={{ display: 'none' }} 
                accept=".json" 
                onChange={handleImportWorkspace} 
              />
              <Button variant="secondary" onClick={() => importInputRef.current?.click()} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" /></svg>
                {t('import_workspace') || 'Import'}
              </Button>
              <Button onClick={() => setShowCreate(true)}>
                {t('new_workspace_button')}
              </Button>
            </div>
          </div>

          {(workspaces.length === 0) ? (
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
                className="card border-dashed"
                onClick={() => setShowCreate(true)}
                style={{
                  minHeight: '140px',
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  justifyContent: 'center',
                  cursor: 'pointer'
                }}
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
                  onKeyDown={e => {
                    if (e.key === 'Enter' && newName.trim()) {
                      handleCreate();
                    } else if (e.key === 'Escape') {
                      setShowCreate(false);
                      setNewName('');
                    }
                  }}
                  placeholder={t('workspace_placeholder')}
                  autoFocus
                />
              </div>
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => { setShowCreate(false); setNewName(''); }}>{t('cancel')}</Button>
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
        .then(res => setWorkspace(res.data.data))
        .catch(console.error)

      axios.get(`${API_URL}/workspaces/${wsId}/tables`)
        .then(res => setTables(res.data.data))
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
      setTables(res.data.data)
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
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <TopNavbar user={user} onLogout={onLogout} workspaceId={wsId} />

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

            </div>

            {/* Tables section header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
              <h2 style={{ fontSize: '1.125rem', fontWeight: 700 }}>{t('tables')}</h2>
              <Button size="sm" onClick={() => setShowCreateTable(true)}>
                {t('new_table_button')}
              </Button>
            </div>

            {/* Tables grid */}
            {(tables.length === 0 && !showCreateTable) ? (
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

                {/* Create Table Card / Inline Form */}
                <div
                  className="card border-dashed"
                  onClick={() => !showCreateTable && setShowCreateTable(true)}
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    minHeight: '120px',
                    cursor: showCreateTable ? 'default' : 'pointer'
                  }}
                >
                  {!showCreateTable ? (
                    <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                      <div style={{ fontSize: '1.75rem', marginBottom: '0.25rem' }}>+</div>
                      <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>{t('new_table_card_text')}</div>
                    </div>
                  ) : (
                    <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                      <div>
                        <input
                          type="text"
                          className="form-input"
                          value={tableName}
                          onChange={e => setTableName(e.target.value)}
                          onKeyDown={e => {
                            if (e.key === 'Enter' && tableName.trim()) {
                              handleCreateTable(e);
                            } else if (e.key === 'Escape') {
                              setShowCreateTable(false);
                              setTableName('');
                            }
                          }}
                          placeholder={t('table_name_placeholder')}
                          autoFocus
                          style={{ marginBottom: '0.25rem' }}
                        />
                        <p className="form-hint" style={{ fontSize: '0.75rem', textAlign: 'left', margin: 0, paddingLeft: '2px' }}>
                          {t('will_be_created_as')} <code style={{ fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>{tableName.toLowerCase().replace(/\s+/g, '_') || '...'}</code>
                        </p>
                      </div>
                      <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                        <Button variant="secondary" size="sm" onClick={(e) => { e.stopPropagation(); setShowCreateTable(false); setTableName(''); }}>{t('cancel')}</Button>
                        <Button size="sm" onClick={(e) => { e.stopPropagation(); handleCreateTable(e); }} loading={creating} disabled={!tableName.trim()}>{t('create')}</Button>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </motion.div>
        </div>
      </div>
    </div>
  )
}

/* ═══════════════════════════════════════════
   TABLE VIEW
   ═══════════════════════════════════════════ */
function TableView({ user, onLogout }) {
  const { t } = useTranslation()
  const { workspaceId, tableId } = useParams()
  const navigate = useNavigate()

  const [table, setTable] = useState(null)
  const [columns, setColumns] = useState([])
  const [records, setRecords] = useState([])
  const [roles, setRoles] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')
  const [tables, setTables] = useState([])

  // CSV Import Wizard state
  const [csvWizard, setCsvWizard] = useState(null) // null | { step: 1|2|3, raw, headers, rows, mapping, importing, results }
  const csvFileRef = useRef(null)


  useEffect(() => {
    loadData()
  }, [workspaceId, tableId])

  const loadData = async () => {
    try {
      const [tableRes, columnsRes, rolesRes, allTablesRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/roles`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
      ])
      const tableData = tableRes.data.data
      const colData = columnsRes.data.data
      setTable(tableData)
      setColumns(colData)
      setRoles(rolesRes.data.data)
      setTables(allTablesRes.data.data || [])

      // Auto-detect relations to expand
      const relationsToExpand = colData
        .filter(c => c.field_type === 'relation')
        .map(c => c.slug)
        .join(',')

      const expandParam = relationsToExpand ? `?expand=${relationsToExpand}` : ''
      const recordsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/data/${tableData.slug}${expandParam}`)
      setRecords(recordsRes.data.data || [])
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const handleExportSchema = async () => {
    try {
      const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/export`, { responseType: 'blob' })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `hornerodb_workspace_${workspaceId}.json`)
      document.body.appendChild(link)
      link.click()
      link.parentNode.removeChild(link)
    } catch (err) {
      console.error(err)
      notify(t('error_export_schema') || 'Error exporting schema', 'error')
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

  const handleCreateColumn = async (name, type, meta = {}) => {
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`, {
        name,
        slug: name.toLowerCase().replace(/\s+/g, '_'),
        field_type: type,
        meta: meta
      })
      loadData()
      notify(t('column_created'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_create_column'), 'error')
    }
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


  // ── CSV Helpers ────────────────────────────────────────────────────────
  const splitCSVLine = (line) => {
    const result = []
    let cur = ''
    let inQ = false
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (ch === '"') { inQ = !inQ }
      else if (ch === ',' && !inQ) { result.push(cur.trim()); cur = '' }
      else { cur += ch }
    }
    result.push(cur.trim())
    return result
  }

  const parseCSV = (text) => {
    const lines = text.split(/\r?\n/).filter(l => l.trim())
    if (!lines.length) return { headers: [], rows: [] }
    const headers = splitCSVLine(lines[0])
    const rows = lines.slice(1).map(l => {
      const vals = splitCSVLine(l)
      const row = {}
      headers.forEach((h, i) => { row[h] = vals[i] ?? '' })
      return row
    })
    return { headers, rows }
  }

  const handleOpenCSVImport = () => { csvFileRef.current?.click() }

  const handleCSVFileChange = (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      const { headers, rows } = parseCSV(ev.target.result)
      const defaultMapping = {}
      headers.forEach(h => {
        const normalized = h.toLowerCase().replace(/\s+/g, '_')
        const match = columns.find(c => c.slug === normalized || c.slug === h.toLowerCase())
        defaultMapping[h] = match ? match.slug : '__ignore__'
      })
      setCsvWizard({ step: 2, headers, rows, mapping: defaultMapping, importing: false, results: null })
    }
    reader.readAsText(file, 'UTF-8')
    e.target.value = ''
  }

  const handleCSVImport = async () => {
    if (!csvWizard) return
    setCsvWizard(prev => ({ ...prev, step: 3, importing: true, results: [] }))
    const results = []
    for (let i = 0; i < csvWizard.rows.length; i++) {
      const row = csvWizard.rows[i]
      const payload = {}
      csvWizard.headers.forEach(h => {
        const target = csvWizard.mapping[h]
        if (target && target !== '__ignore__') {
          const col = columns.find(c => c.slug === target)
          let val = row[h]
          if (col?.field_type === 'number') {
            if (val === '' || val === undefined || val === null) {
              val = null
            } else {
              const num = Number(val.replace(',', '.'))
              val = isNaN(num) ? null : num
            }
          }
          if (col?.field_type === 'boolean') {
            const v = String(val).toLowerCase().trim()
            val = v === 'true' || v === '1' || v === 'yes' || v === 'si' || v === 'sí'
          }
          payload[target] = val
        }
      })
      try {
        await axios.post(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, payload)
        results.push({ row: i + 1, ok: true })
      } catch (err) {
        results.push({ row: i + 1, ok: false, error: err?.response?.data?.error?.message || err.message })
      }
    }
    setCsvWizard(prev => ({ ...prev, importing: false, results }))
    loadData()
  }

  const handleExportCSV = () => {
    if (!records.length || !columns.length) return
    const headers = columns.map(c => c.slug)
    const rows = records.map(r => headers.map(h => {
      const v = r[h] ?? ''
      const s = String(v)
      return s.includes(',') || s.includes('"') || s.includes('\n') ? `"${s.replace(/"/g, '""')}"` : s
    }).join(','))
    const csv = [headers.join(','), ...rows].join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${table?.slug || 'export'}_${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (loading) {

    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <TopNavbar user={user} onLogout={onLogout} workspaceId={workspaceId} />

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
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem', gap: '0.75rem' }}>
            <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em' }}>{table?.name}</h1>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              {/* Export Schema */}
              <Button size="sm" variant="secondary" onClick={handleExportSchema} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
                {t('export_schema') || 'Export Schema'}
              </Button>
              {/* Import CSV */}
              <input ref={csvFileRef} type="file" accept=".csv,text/csv" style={{ display: 'none' }} onChange={handleCSVFileChange} />
              <Button size="sm" variant="secondary" onClick={handleOpenCSVImport} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l4-4m0 0l4 4m-4-4v12" /></svg>
                Import CSV
              </Button>
              {/* Export CSV */}
              <Button size="sm" variant="secondary" onClick={handleExportCSV} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                Export CSV
              </Button>
            </div>
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
              workspaceId={workspaceId}
              tables={tables}
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

      {/* ── CSV Import Wizard Modal ─────────────────────────────────────── */}
      {csvWizard && (
        <div className="modal-overlay" onClick={() => !csvWizard.importing && setCsvWizard(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '680px', maxHeight: '85vh', overflowY: 'auto' }}>
            <div className="modal-header">
              <h3 className="modal-title">
                {csvWizard.step === 2 ? 'Mapear Columnas CSV' : csvWizard.step === 3 ? 'Importar Datos' : 'Importar CSV'}
              </h3>
              {!csvWizard.importing && (
                <button className="btn btn-ghost btn-sm" onClick={() => setCsvWizard(null)}>
                  <Xmark width="1rem" height="1rem" />
                </button>
              )}
            </div>

            {/* Step 2 – Column Mapping */}
            {csvWizard.step === 2 && (
              <div className="modal-body">
                <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                  Asigná cada columna del CSV a una columna de la tabla, o ignorala.
                </p>
                <div style={{ display: 'grid', gap: '0.75rem' }}>
                  {csvWizard.headers.map(h => (
                    <div key={h} style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', gap: '0.75rem' }}>
                      <div style={{ padding: '0.5rem 0.75rem', background: 'var(--bg-subtle)', borderRadius: '6px', fontFamily: 'var(--font-mono)', fontSize: '0.8125rem', border: '1px solid var(--border)' }}>
                        {h}
                      </div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem' }}>→</span>
                      <select
                        className="form-select"
                        value={csvWizard.mapping[h]}
                        onChange={e => setCsvWizard(prev => ({ ...prev, mapping: { ...prev.mapping, [h]: e.target.value } }))}
                      >
                        <option value="__ignore__">— Ignorar —</option>
                        {columns.map(c => (
                          <option key={c.slug} value={c.slug}>{c.name} ({c.slug})</option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
                <p style={{ marginTop: '1rem', fontSize: '0.8125rem', color: 'var(--text-muted)' }}>
                  {csvWizard.rows.length} filas detectadas
                </p>
              </div>
            )}

            {/* Step 3 – Results */}
            {csvWizard.step === 3 && (
              <div className="modal-body">
                {csvWizard.importing ? (
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '1rem', padding: '2rem 0' }}>
                    <div className="loading-spinner" />
                    <p>Importando {csvWizard.rows.length} registros...</p>
                  </div>
                ) : (
                  <div>
                    <p style={{ marginBottom: '1rem' }}>
                      ✅ {csvWizard.results?.filter(r => r.ok).length} importados &nbsp;·&nbsp;
                      ❌ {csvWizard.results?.filter(r => !r.ok).length} errores
                    </p>
                    {csvWizard.results?.filter(r => !r.ok).map(r => (
                      <div key={r.row} style={{ padding: '0.5rem 0.75rem', background: 'var(--danger-bg, #fee2e2)', borderRadius: '6px', fontSize: '0.8125rem', marginBottom: '0.5rem', color: 'var(--danger, #dc2626)' }}>
                        Fila {r.row}: {r.error}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            <div className="modal-footer">
              {csvWizard.step === 2 && (
                <>
                  <Button variant="secondary" onClick={() => setCsvWizard(null)}>Cancelar</Button>
                  <Button onClick={handleCSVImport} disabled={!csvWizard.rows.length}>
                    Importar {csvWizard.rows.length} registros
                  </Button>
                </>
              )}
              {csvWizard.step === 3 && !csvWizard.importing && (
                <Button onClick={() => setCsvWizard(null)}>Cerrar</Button>
              )}
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
function Settings({ user, onLogout }) {
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
  const [selectedKey, setSelectedKey] = useState(null)
  const [selectedRoleId, setSelectedRoleId] = useState(null)
  const [newRoleName, setNewRoleName] = useState('')
  const [newKeyName, setNewKeyName] = useState('')
  const [newKeyRole, setNewKeyRole] = useState('')
  const [rotatedKeyData, setRotatedKeyData] = useState(null)
  const [createdKeyModal, setCreatedKeyModal] = useState(null) // { name, key } shown after create
  const [newKeyExpiresIn, setNewKeyExpiresIn] = useState(0)
  const [newKeyRateLimit, setNewKeyRateLimit] = useState(0)
  const [newKeyRateLimitPerHour, setNewKeyRateLimitPerHour] = useState(0)

  // API key editing state
  const [editingKeyName, setEditingKeyName] = useState('')
  const [editingKeyRole, setEditingKeyRole] = useState('')
  const [editingKeyRateLimit, setEditingKeyRateLimit] = useState(60)
  const [editingKeyRateLimitPerHour, setEditingKeyRateLimitPerHour] = useState(3600)
  const [editingKeyOrigins, setEditingKeyOrigins] = useState('')
  const [editingKeyReferers, setEditingKeyReferers] = useState('')
  const [savingKey, setSavingKey] = useState(false)

  useEffect(() => {
    loadData()
  }, [workspaceId, activeSection])

  const loadData = async () => {
    setLoading(true)
    try {
      if (!workspace) {
        const wsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}`)
        setWorkspace(wsRes.data.data)
        const st = wsRes.data.data.settings || {}
        setRateLimit(st.rate_limit_per_minute ?? 60)
        setAllowedOrigins((st.allowed_origins || []).join(', '))
      }

      // Always load roles & tables so they are available even on the keys tab
      const [rolesRes, tablesRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${workspaceId}/roles`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
      ])
      const tablesWithCols = await Promise.all(tablesRes.data.data.map(async (t) => {
        try {
          const cr = await axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${t.id}/columns`)
          return { ...t, columns: cr.data.data }
        } catch (e) { return t }
      }))
      setRoles(rolesRes.data.data)
      setTables(tablesWithCols)
      if (rolesRes.data.data.length > 0 && !selectedRoleId) {
        setSelectedRoleId(rolesRes.data.data[0].id)
      }

      if (activeSection === 'keys') {
        const keysRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/keys`)
        setAPIKeys(keysRes.data.data)
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
      setWorkspace(wsRes.data.data)
    } catch (err) {
      console.error(err)
      notify(t('error_save_settings') || 'Error fetching', 'error')
    }
    setSavingGeneral(false)
  }

  const handleCreateRole = async () => {
    if (!newRoleName.trim()) return
    try {
      const resp = await axios.post(`${API_URL}/workspaces/${workspaceId}/roles`, {
        name: newRoleName,
        description: 'Nuevo rol',
        permissions: {}
      })
      setShowCreateRole(false)
      setNewRoleName('')
      setSelectedRoleId(resp.data.data?.id)
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
        expires_in_days: parseInt(newKeyExpiresIn) || 0,
        rate_limit_per_minute: parseInt(newKeyRateLimit) || 0,
        rate_limit_per_hour: parseInt(newKeyRateLimitPerHour) || 0
      })
      // Show key in a persistent modal since it won't be visible again
      setCreatedKeyModal({ name: newKeyName, key: res.data.data?.key || res.data.key })
      setShowCreateKey(false)
      setNewKeyName('')
      setNewKeyRole('')
      setNewKeyExpiresIn(0)
      setNewKeyRateLimit(0)
      setNewKeyRateLimitPerHour(0)
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

  const openKeySettings = (key) => {
    setSelectedKey(key)
    setEditingKeyName(key.name)
    // Handle role_id as it can be a pointer
    const roleIdStr = key.role_id ? (typeof key.role_id === 'string' ? key.role_id : key.role_id.toString()) : ''
    setEditingKeyRole(roleIdStr)
    setEditingKeyRateLimit(key.rate_limit_per_minute ?? 60)
    setEditingKeyRateLimitPerHour(key.rate_limit_per_hour ?? 3600)
    setEditingKeyOrigins((key.allowed_origins || []).join(', '))
    setEditingKeyReferers((key.allowed_referers || []).join(', '))
  }

  const closeKeySettings = () => {
    setSelectedKey(null)
  }

  const saveKeySettings = async () => {
    if (!selectedKey) return
    setSavingKey(true)
    try {
      const originsArray = editingKeyOrigins.split(',').map(o => o.trim()).filter(o => o.length > 0)
      const referersArray = editingKeyReferers.split(',').map(r => r.trim()).filter(r => r.length > 0)

      await axios.put(`${API_URL}/workspaces/${workspaceId}/keys/${selectedKey.id}`, {
        name: editingKeyName,
        role_id: editingKeyRole || null,
        rate_limit_per_minute: editingKeyRateLimit,
        rate_limit_per_hour: editingKeyRateLimitPerHour,
        allowed_origins: originsArray,
        allowed_referers: referersArray
      })
      notify(t('api_key_updated') || 'API key updated', 'success')
      loadData()
      setSelectedKey(null)
    } catch (err) {
      console.error(err)
      notify(t('error_update_api_key') || 'Error updating API key', 'error')
    }
    setSavingKey(false)
  }

  const handleRotateKey = async (keyId, keyName) => {
    if (!confirm(t('confirm_rotate_key', { name: keyName }) || `¿Seguro que quieres regenerar la API key "${keyName}"? La clave anterior dejará de funcionar.`)) return;
    try {
      const res = await axios.post(`${API_URL}/workspaces/${workspaceId}/keys/${keyId}/rotate`)
      setRotatedKeyData(res.data.data)
      loadData()
      notify(t('api_key_rotated') || 'API key regenerated', 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rotate_api_key') || 'Error regenerating API key', 'error')
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
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <TopNavbar user={user} onLogout={onLogout} workspaceId={workspaceId} />

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
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem' }}>
                {roles.length === 0 && !showCreateRole ? (
                  <div className="empty-state" style={{ gridColumn: '1 / -1', border: '1px dashed var(--border)', background: 'var(--bg-subtle)' }}>
                    <div className="empty-icon"><ShieldCheck width="2rem" height="2rem" /></div>
                    <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('no_roles')}</h3>
                    <p style={{ color: 'var(--text-secondary)' }}>{t('create_roles_hint')}</p>
                  </div>
                ) : (
                  roles.map((role) => (
                    <div 
                      key={role.id} 
                      className="card" 
                      onClick={() => setSelectedRoleId(role.id)}
                      style={{ 
                        cursor: 'pointer', 
                        border: selectedRoleId === role.id ? '2px solid var(--primary)' : undefined,
                        padding: selectedRoleId === role.id ? 'calc(1.5rem - 1px)' : undefined 
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                        <div>
                          <div style={{ fontWeight: 700, marginBottom: '0.25rem' }}>{role.name}</div>
                          <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                            {role.description || t('no_description')}
                          </div>
                          {role.is_default && (
                            <Badge variant="primary" className="" style={{ marginTop: '0.75rem' }}>{t('default')}</Badge>
                          )}
                        </div>
                        <Button 
                          size="sm" 
                          variant={selectedRoleId === role.id ? 'primary' : 'secondary'}
                          onClick={(e) => { e.stopPropagation(); setSelectedRoleId(role.id); }}
                        >
                          {selectedRoleId === role.id ? t('selected') || 'Seleccionado' : t('select') || 'Seleccionar'}
                        </Button>
                      </div>
                    </div>
                  ))
                )}

                {/* Create Role Card / Inline Form */}
                <div
                  className="card border-dashed"
                  onClick={() => !showCreateRole && setShowCreateRole(true)}
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    minHeight: roles.length === 0 ? '140px' : 'auto',
                    cursor: showCreateRole ? 'default' : 'pointer'
                  }}
                >
                  {!showCreateRole ? (
                    <div style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                      <div style={{ fontSize: '1.75rem', marginBottom: '0.25rem' }}>+</div>
                      <div style={{ fontSize: '0.875rem', fontWeight: 600 }}>{t('new_role')}</div>
                    </div>
                  ) : (
                    <div style={{ width: '100%', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                      <input
                        type="text"
                        className="form-input"
                        value={newRoleName}
                        onChange={e => setNewRoleName(e.target.value)}
                        onKeyDown={e => {
                          if (e.key === 'Enter' && newRoleName.trim()) {
                            handleCreateRole(e);
                          } else if (e.key === 'Escape') {
                            setShowCreateRole(false);
                            setNewRoleName('');
                          }
                        }}
                        placeholder={t('role_placeholder')}
                        autoFocus
                      />
                      <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                        <Button variant="secondary" size="sm" onClick={(e) => { e.stopPropagation(); setShowCreateRole(false); setNewRoleName(''); }}>{t('cancel')}</Button>
                        <Button size="sm" onClick={(e) => { e.stopPropagation(); handleCreateRole(e); }} disabled={!newRoleName.trim()}>{t('create')}</Button>
                      </div>
                    </div>
                  )}
                </div>
              </div>

              {/* Inline Permission Matrix */}
              <PermissionMatrix
                workspaceId={workspaceId}
                tables={tables}
                roles={roles}
                selectedRoleId={selectedRoleId || (roles.length > 0 ? roles[0].id : '')}
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
                      <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <button
                          onClick={() => handleRotateKey(key.id, key.name)}
                          className="btn btn-ghost btn-sm"
                          title={t('rotate_key') || 'Regenerate API Key'}
                        >
                          <svg width="1rem" height="1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                        </button>
                        <button
                          onClick={() => openKeySettings(key)}
                          className="btn btn-ghost btn-sm"
                          title={t('edit_key_settings') || 'Edit settings'}
                        >
                          <SettingsIcon width="1rem" height="1rem" />
                        </button>
                        <button
                          onClick={() => deleteAPIKey(key.id)}
                          className="btn btn-ghost btn-sm"
                          style={{ color: 'var(--danger)' }}
                        >
                          <Trash width="1rem" height="1rem" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {/* API Key Settings Panel - fuera del ternary */}
              {selectedKey && (
                <div className="card" style={{ marginTop: '1rem', padding: '1.5rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                    <h3 style={{ fontSize: '1.125rem', fontWeight: 700 }}>
                      {t('edit_key_settings') || 'API Key Settings'}
                    </h3>
                    <button onClick={closeKeySettings} className="btn btn-ghost btn-sm">
                      <Xmark width="1rem" height="1rem" />
                    </button>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                    <div className="form-group">
                      <label className="form-label">{t('name')}</label>
                      <input
                        type="text"
                        className="form-input"
                        value={editingKeyName}
                        onChange={e => setEditingKeyName(e.target.value)}
                      />
                    </div>
                    <div className="form-group">
                      <label className="form-label">{t('role_label')}</label>
                      <select
                        className="form-select"
                        value={editingKeyRole}
                        onChange={e => setEditingKeyRole(e.target.value)}
                      >
                        <option value="">{t('select_role') || 'Sin rol'}</option>
                        {roles.map(r => (
                          <option key={r.id} value={r.id}>{r.name}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginTop: '1rem' }}>
                    <div className="form-group">
                      <label className="form-label">{t('rate_limit_per_minute')} & {t('rate_limit_per_hour', 'per hour')}</label>
                      <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <input
                          type="number"
                          className="form-input"
                          value={editingKeyRateLimit}
                          onChange={e => setEditingKeyRateLimit(Number(e.target.value))}
                          min="0"
                          title="Rate limit per minute"
                          placeholder="Per minute"
                        />
                        <input
                          type="number"
                          className="form-input"
                          value={editingKeyRateLimitPerHour}
                          onChange={e => setEditingKeyRateLimitPerHour(Number(e.target.value))}
                          min="0"
                          title="Rate limit per hour"
                          placeholder="Per hour"
                        />
                      </div>
                    </div>
                    <div className="form-group">
                      <label className="form-label">{t('expires_at')}</label>
                      <input
                        type="text"
                        className="form-input"
                        value={selectedKey.expires_at ? new Date(selectedKey.expires_at).toLocaleDateString() : '-'}
                        disabled
                      />
                    </div>
                  </div>

                  <div className="form-group" style={{ marginTop: '1rem' }}>
                    <label className="form-label">{t('allowed_origins')}</label>
                    <input
                      type="text"
                      className="form-input"
                      value={editingKeyOrigins}
                      onChange={e => setEditingKeyOrigins(e.target.value)}
                      placeholder="https://example.com, https://app.example.com"
                    />
                    <small style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                      {t('allowed_origins_hint') || 'Separate with commas'}
                    </small>
                  </div>

                  <div className="form-group" style={{ marginTop: '1rem' }}>
                    <label className="form-label">{t('allowed_referers')}</label>
                    <input
                      type="text"
                      className="form-input"
                      value={editingKeyReferers}
                      onChange={e => setEditingKeyReferers(e.target.value)}
                      placeholder="https://example.com/, https://app.example.com/"
                    />
                    <small style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
                      {t('allowed_referers_hint') || 'Separate with commas'}
                    </small>
                  </div>

                  {/* Usage Metrics */}
                  <div style={{ marginTop: '1.5rem', padding: '1rem', background: 'var(--bg-hover)', borderRadius: '8px' }}>
                    <h4 style={{ fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.75rem' }}>
                      {t('usage_metrics') || 'Usage Metrics'}
                    </h4>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                      <div>
                        <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('created_at')}</div>
                        <div style={{ fontSize: '0.875rem' }}>
                          {selectedKey.created_at ? new Date(selectedKey.created_at).toLocaleString() : '-'}
                        </div>
                      </div>
                      <div>
                        <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{t('last_used')}</div>
                        <div style={{ fontSize: '0.875rem' }}>
                          {selectedKey.last_used_at ? new Date(selectedKey.last_used_at).toLocaleString() : '-'}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div style={{ marginTop: '1.5rem', display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                    <Button variant="secondary" onClick={closeKeySettings}>
                      {t('cancel')}
                    </Button>
                    <Button onClick={saveKeySettings} loading={savingKey}>
                      {t('save_changes')}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
      
      {rotatedKeyData && (
        <div className="modal-overlay" onClick={() => setRotatedKeyData(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '500px' }}>
            <div className="modal-header">
              <h3 className="modal-title">{t('api_key_rotated') || 'New API Key'}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setRotatedKeyData(null)}>
                <Xmark width="1rem" height="1rem" />
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('copy_key_warning') || 'Please copy your new API key now. You will not be able to see it again.'}</label>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <input
                    type="text"
                    className="form-input"
                    value={rotatedKeyData.new_key}
                    readOnly
                  />
                  <Button variant="secondary" onClick={() => {
                    navigator.clipboard.writeText(rotatedKeyData.new_key)
                    notify(t('copied') || 'Copied to clipboard', 'success')
                  }}>
                    <ClipboardCheck width="1rem" height="1rem" />
                  </Button>
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <Button onClick={() => setRotatedKeyData(null)}>{t('close') || 'Close'}</Button>
            </div>
          </div>
        </div>
      )}

      {/* New API Key Created Modal — show raw key once since it won't be visible again */}
      {createdKeyModal && (
        <div className="modal-overlay" onClick={() => setCreatedKeyModal(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '520px' }}>
            <div className="modal-header">
              <h3 className="modal-title">{t('api_key_created_title') || '🔑 API Key creada'}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setCreatedKeyModal(null)}>
                <Xmark width="1rem" height="1rem" />
              </button>
            </div>
            <div className="modal-body">
              <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                {t('copy_key_warning') || 'Copiá la clave ahora. No vas a poder verla de nuevo.'}
              </p>
              <div className="form-group">
                <label className="form-label">{createdKeyModal.name}</label>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <input
                    type="text"
                    className="form-input"
                    value={createdKeyModal.key}
                    readOnly
                    style={{ fontFamily: 'var(--font-mono)', fontSize: '0.8125rem' }}
                  />
                  <Button variant="secondary" onClick={() => {
                    navigator.clipboard.writeText(createdKeyModal.key)
                    notify(t('copied') || 'Copiado al portapapeles', 'success')
                  }}>
                    <ClipboardCheck width="1rem" height="1rem" />
                  </Button>
                </div>
              </div>
              <div style={{ marginTop: '1rem', padding: '0.75rem', background: 'var(--warning-bg, #fef3c7)', borderRadius: '8px', border: '1px solid var(--warning-border, #fbbf24)', fontSize: '0.8125rem', color: 'var(--warning-text, #92400e)' }}>
                ⚠️ {t('key_warning_once') || 'Esta clave solo se muestra una vez. Si la perdés, usá el botón Regenerar en la lista de keys.'}
              </div>
            </div>
            <div className="modal-footer">
              <Button onClick={() => setCreatedKeyModal(null)}>{t('close') || 'Cerrar'}</Button>
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
              <div className="form-group">
                <label className="form-label">Expiration</label>
                <select className="form-select" value={newKeyExpiresIn} onChange={e => setNewKeyExpiresIn(e.target.value)}>
                  <option value={0}>Unlimited</option>
                  <option value={30}>30 Days</option>
                  <option value={60}>60 Days</option>
                  <option value={90}>90 Days</option>
                </select>
              </div>
              <div className="form-group">
                <label className="form-label">Rate Limits (0 for unlimited)</label>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <input type="number" className="form-input" placeholder="Per Min" value={newKeyRateLimit} onChange={e => setNewKeyRateLimit(e.target.value)} />
                  <input type="number" className="form-input" placeholder="Per Hr" value={newKeyRateLimitPerHour} onChange={e => setNewKeyRateLimitPerHour(e.target.value)} />
                </div>
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
