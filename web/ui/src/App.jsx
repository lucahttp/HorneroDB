import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Link, useNavigate, useParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import axios from 'axios'
import { ThemeToggle, Button, Badge } from './components/index.jsx'
import { PermissionMatrix } from './components/PermissionMatrix.jsx'
import { ToastProvider, notify } from './components/Toast.jsx'
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
  )
}

function Login() {
  const handleLogin = () => {
    window.location.href = `${API_URL}/auth/oidc/login?redirect=${encodeURIComponent(window.location.origin + '/callback')}`
  }

  return (
    <div className="login-container">
      <div className="login-left">
        <motion.div 
          className="text-center"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
        >
          <div className="text-8xl mb-6">🐦</div>
          <h1 className="text-white text-4xl font-bold">HorneroDB</h1>
          <p className="text-gray-400 mt-4 text-lg">Tu base de datos personal</p>
        </motion.div>
      </div>
      <div className="login-right">
        <motion.div 
          className="w-full max-w-md"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2 }}
        >
          <h2 className="text-3xl font-bold mb-2">Bienvenido</h2>
          <p className="text-gray-500 mb-8">Iniciá sesión para continuar</p>
          
          <button 
            onClick={handleLogin}
            className="w-full bg-primary text-gray-900 font-semibold py-3 px-6 rounded-xl hover:bg-[#1EB8E6] transition-all duration-200 flex items-center justify-center gap-3"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
            Iniciar sesión con PocketID
          </button>
          
          <p className="text-gray-400 text-sm text-center mt-8">
            Acceso seguro con OIDC
          </p>
        </motion.div>
      </div>
    </div>
  )
}

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
    <div className="min-h-screen flex items-center justify-center">
      <div className="loading-spinner"></div>
    </div>
  )
}

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
          <span>🐦</span>
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
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={link.icon} />
            </svg>
            {link.label}
          </button>
        ))}
      </nav>

      <div className="sidebar-footer">
        <div className="flex items-center gap-3 px-2 py-2">
          <div className="avatar">
            {user?.email?.charAt(0).toUpperCase() || 'U'}
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-sm text-white truncate">{user?.email || 'Usuario'}</div>
            <div className="text-xs text-gray-500">{user?.role || 'user'}</div>
          </div>
        </div>
        <button 
          onClick={onLogout}
          className="w-full mt-3 px-3 py-2 text-sm text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors flex items-center gap-2"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          Cerrar sesión
        </button>
      </div>
    </div>
  )
}

function Dashboard({ user, onLogout }) {
  const [workspaces, setWorkspaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(res.data))
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
        owner_id: user?.id || '00000000-0000-0000-0000-000000000001'
      })
      setShowCreate(false)
      setNewName('')
      const res = await axios.get(`${API_URL}/workspaces`)
      setWorkspaces(res.data)
    } catch (err) {
      console.error(err)
      notify('Error al crear workspace', 'error')
    } finally {
      setCreating(false)
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="loading-spinner"></div>
      </div>
    )
  }

  if (workspaces.length === 0) {
    return (
      <div className="min-h-screen flex">
        <Sidebar user={user} onLogout={onLogout} workspaceId="new" />
        <div className="main-content flex items-center justify-center">
          <motion.div 
            className="text-center"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
          >
            <div className="text-6xl mb-4">📁</div>
            <h2 className="text-2xl font-bold mb-2">Sin workspaces</h2>
            <p className="text-gray-500 mb-6">Creá tu primer workspace para comenzar</p>
            <Button onClick={() => setShowCreate(true)}>
              + Crear workspace
            </Button>
          </motion.div>
        </div>

        {showCreate && (
          <div className="modal-overlay" onClick={() => setShowCreate(false)}>
            <div className="modal" onClick={e => e.stopPropagation()}>
              <div className="modal-header">
                <h3 className="modal-title">Nuevo Workspace</h3>
                <button className="btn btn-ghost btn-sm" onClick={() => setShowCreate(false)}>
                  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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

  return <Workspace user={user} onLogout={onLogout} workspaceProp={workspaces[0]} />
}

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

  const wsId = workspaceId || workspace?.id

  if (loading || !wsId) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="loading-spinner"></div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex">
      <Sidebar user={user} onLogout={onLogout} workspaceId={wsId} />
      
      <div className="main-content">
        <div className="main-body">
          <motion.div 
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
          >
            <div className="flex items-center justify-between mb-8">
              <div>
                <h1 className="text-2xl font-bold">{workspace?.name || 'Workspace'}</h1>
                <p className="text-gray-500">@{workspace?.slug}</p>
              </div>
              <div className="flex items-center gap-3">
                <ThemeToggle />
              </div>
            </div>

            <div className="flex items-center justify-between mb-6">
              <h2 className="text-lg font-semibold">Tablas</h2>
              <Button size="sm" onClick={() => setShowCreateTable(true)}>
                + Nueva tabla
              </Button>
            </div>

            {tables.length === 0 ? (
              <div className="empty-state">
                <div className="empty-icon">📋</div>
                <h3 className="text-lg font-medium mb-2">Sin tablas todavía</h3>
                <p className="text-gray-500 mb-4">Creá tu primera tabla para organizar datos</p>
                <Button onClick={() => setShowCreateTable(true)}>
                  + Crear primera tabla
                </Button>
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
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
                      <div className="flex items-center gap-3 mb-2">
                        <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
                          <svg className="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
                          </svg>
                        </div>
                        <div className="font-semibold">{table.name}</div>
                      </div>
                      <div className="text-sm text-gray-500">@{table.slug}</div>
                    </div>
                  </motion.div>
                ))}
                
                <div 
                  className="card border-dashed cursor-pointer flex items-center justify-center min-h-[120px]"
                  onClick={() => setShowCreateTable(true)}
                >
                  <div className="text-center text-gray-400">
                    <div className="text-2xl mb-1">+</div>
                    <div className="text-sm">Nueva tabla</div>
                  </div>
                </div>
              </div>
            )}
          </motion.div>
        </div>
      </div>

      {showCreateTable && (
        <div className="modal-overlay" onClick={() => setShowCreateTable(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nueva Tabla</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateTable(false)}>
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
                  Se creará como: <code className="font-mono">{tableName.toLowerCase().replace(/\s+/g, '_') || '...'}</code>
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
      <div className="min-h-screen flex items-center justify-center">
        <div className="loading-spinner"></div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex">
      <div className="sidebar">
        <div className="sidebar-header">
          <Link to={`/workspace/${workspaceId}`} className="sidebar-logo">
            <span>🐦</span>
            <span>HorneroDB</span>
          </Link>
        </div>
        <nav className="sidebar-nav">
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}`)}>
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M3 14h18m-9-4v8m-7 0h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            Datos
          </button>
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}/settings`)}>
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            Configuración
          </button>
        </nav>
      </div>
      
      <div className="main-content">
        <div className="main-body">
          <button 
            onClick={() => navigate(`/workspace/${workspaceId}`)}
            className="btn btn-ghost mb-4"
          >
            ← Volver
          </button>

          <div className="flex items-center justify-between mb-6">
            <h1 className="text-2xl font-bold">{table?.name}</h1>
            <div className="flex gap-2">
              <Button size="sm" variant="secondary" onClick={() => setShowCreateColumn(true)}>
                + Columna
              </Button>
              <Button size="sm" onClick={() => setShowCreateRecord(true)}>
                + Registro
              </Button>
            </div>
          </div>

          <div className="tabs">
            <button 
              className={`tab ${activeTab === 'data' ? 'active' : ''}`}
              onClick={() => setActiveTab('data')}
            >
              Datos
            </button>
            <button 
              className={`tab ${activeTab === 'columns' ? 'active' : ''}`}
              onClick={() => setActiveTab('columns')}
            >
              Columnas ({columns.length})
            </button>
          </div>

          {activeTab === 'data' && (
            <div className="table-container">
              {records.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">📝</div>
                  <h3 className="text-lg font-medium mb-2">Sin datos</h3>
                  <p className="text-gray-500 mb-4">Agregá el primer registro</p>
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
                        <td><code className="text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">{String(record.id)?.slice(0, 8)}</code></td>
                        {columns.map(col => (
                          <td key={col.id}>{String(record[col.slug] || '-')}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}

          {activeTab === 'columns' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
              {columns.map(col => (
                <div key={col.id} className="card">
                  <div className="font-semibold">{col.name}</div>
                  <div className="text-sm text-gray-500">@{col.slug}</div>
                  <Badge variant="primary" className="mt-2">{col.field_type}</Badge>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {showCreateRecord && (
        <div className="modal-overlay" onClick={() => setShowCreateRecord(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nuevo Registro</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateRecord(false)}>✕</button>
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

      {showCreateColumn && (
        <div className="modal-overlay" onClick={() => setShowCreateColumn(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nueva Columna</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateColumn(false)}>✕</button>
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

function Settings() {
  const { workspaceId } = useParams()
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
    <div className="min-h-screen flex">
      <div className="sidebar">
        <div className="sidebar-header">
          <Link to={`/workspace/${workspaceId}`} className="sidebar-logo">
            <span>🐦</span>
            <span>HorneroDB</span>
          </Link>
        </div>
        <nav className="sidebar-nav">
          <button className="sidebar-link" onClick={() => navigate(`/workspace/${workspaceId}`)}>
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M3 14h18m-9-4v8m-7 0h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
            Datos
          </button>
          <button className="sidebar-link active">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            Configuración
          </button>
        </nav>
      </div>

      <div className="main-content">
        <div className="main-body">
          <h1 className="text-2xl font-bold mb-6">Configuración</h1>

          <div className="tabs">
            <button 
              className={`tab ${activeSection === 'roles' ? 'active' : ''}`}
              onClick={() => setActiveSection('roles')}
            >
              Roles de Seguridad
            </button>
            <button 
              className={`tab ${activeSection === 'keys' ? 'active' : ''}`}
              onClick={() => setActiveSection('keys')}
            >
              API Keys
            </button>
          </div>

          {loading ? (
            <div className="flex justify-center py-12">
              <div className="loading-spinner"></div>
            </div>
          ) : activeSection === 'roles' ? (
            <div>
              <div className="flex items-center justify-between mb-6">
                <p className="text-gray-500">Gestiona los roles y permisos de acceso</p>
                <div className="flex gap-2">
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
                  <h3 className="text-lg font-medium mb-2">Sin roles</h3>
                  <p className="text-gray-500">Creá roles para gestionar permisos</p>
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                  {roles.map(role => (
                    <div key={role.id} className="card">
                      <div className="font-semibold">{role.name}</div>
                      <div className="text-sm text-gray-500">{role.description || 'Sin descripción'}</div>
                      {role.is_default && (
                        <Badge variant="primary" className="mt-2">Default</Badge>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div>
              <div className="flex items-center justify-between mb-6">
                <p className="text-gray-500">Genera claves para acceso programático</p>
                <Button size="sm" onClick={() => setShowCreateKey(true)}>
                  + Nueva API Key
                </Button>
              </div>

              {apiKeys.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon">🔑</div>
                  <h3 className="text-lg font-medium mb-2">Sin API keys</h3>
                  <p className="text-gray-500">Creá una key para usar la API</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {apiKeys.map(key => (
                    <div key={key.id} className="flex items-center justify-between p-4 bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl">
                      <div>
                        <div className="font-medium">{key.name}</div>
                        <div className="text-sm text-gray-500 font-mono">{key.prefix}...{key.key_hash?.slice(-8)}</div>
                      </div>
                      <button 
                        onClick={() => deleteAPIKey(key.id)}
                        className="btn btn-ghost btn-sm text-red-500"
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

      {showCreateRole && (
        <div className="modal-overlay" onClick={() => setShowCreateRole(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nuevo Rol</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateRole(false)}>✕</button>
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

      {showCreateKey && (
        <div className="modal-overlay" onClick={() => setShowCreateKey(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Nueva API Key</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateKey(false)}>✕</button>
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
