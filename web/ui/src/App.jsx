import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Link, useNavigate, useParams } from 'react-router-dom'
import axios from 'axios'
import './index.css'

const API_URL = 'http://localhost:8080/api/v1'
const POCKETID_URL = 'http://localhost:1411'

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
      <Routes>
        <Route path="/" element={<Login onLogin={setToken} />} />
        <Route path="/callback" element={<Callback onLogin={setToken} onUser={setUser} />} />
        <Route path="/dashboard" element={token ? <Dashboard onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/dashboard/tables" element={token ? <Tables onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/dashboard/tables/:tableId" element={token ? <TableView onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/dashboard/settings" element={token ? <Settings onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/workspace/:workspaceId" element={token ? <Workspace onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/workspace/:workspaceId/tables" element={token ? <Tables onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/workspace/:workspaceId/tables/:tableId" element={token ? <TableView onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
        <Route path="/workspace/:workspaceId/settings" element={token ? <Settings onLogout={handleLogout} user={user} /> : <Login onLogin={setToken} />} />
      </Routes>
    </BrowserRouter>
  )
}

function Login({ onLogin }) {
  const navigate = useNavigate()
  
  const handleLogin = () => {
    // Redirect to HorneroDB OIDC login endpoint which will redirect to PocketID
    window.location.href = `${API_URL}/auth/oidc/login?redirect=${encodeURIComponent(window.location.origin + '/callback')}`
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-logo">🐦</div>
        <h1 className="login-title">HorneroDB</h1>
        <p className="login-subtitle">Tu base de datos</p>
        <button className="login-btn" onClick={handleLogin}>
          Iniciar sesión con PocketID
        </button>
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
      const hash = window.location.hash
      if (hash.includes('token=')) {
        const tokenMatch = hash.match(/token=([^&]+)/)
        if (tokenMatch) {
          localStorage.setItem('hornero_token', tokenMatch[1])
          axios.defaults.headers.common['Authorization'] = `Bearer ${tokenMatch[1]}`
          onLogin(tokenMatch[1])
          axios.get(`${API_URL}/auth/me`)
            .then(res => {
              onUser(res.data)
              navigate('/dashboard')
            })
            .catch(() => navigate('/'))
          return
        }
      }
      navigate('/')
    }
  }, [])

  return <div className="loading"><div className="spinner"></div></div>
}

function Header({ onLogout, activeTab, setActiveTab, user }) {
  return (
    <header className="header">
      <div className="container">
        <div className="header-content">
          <Link to="/dashboard" className="logo">
            🐦 HorneroDB
          </Link>
          <div className="header-right">
            {user && (
              <span className="user-badge">{user.role || 'user'}</span>
            )}
            <button className={`header-link ${activeTab === 'data' ? '' : ''}`} onClick={() => setActiveTab('data')}>
              Mis Datos
            </button>
            <button className={`header-link ${activeTab === 'settings' ? '' : ''}`} onClick={() => setActiveTab('settings')}>
              Configuración
            </button>
            <button className="header-link" onClick={onLogout}>
              Salir
            </button>
          </div>
        </div>
      </div>
    </header>
  )
}

function Dashboard({ onLogout, user }) {
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')

  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(res.data))
      .catch(() => setWorkspaces([]))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="loading"><div className="spinner"></div></div>

  if (workspaces.length === 0) {
    return (
      <>
        <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
        <main className="dashboard">
          <div className="container">
            <div className="dashboard-header">
              <h1 className="dashboard-title">Mis Datos</h1>
              <p className="dashboard-subtitle">No tienes workspaces todavía</p>
            </div>
            <CreateWorkspace onCreated={() => window.location.reload()} />
          </div>
        </main>
      </>
    )
  }

  const workspace = workspaces[0]

  return (
    <>
      <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
      <main className="dashboard">
        <div className="container">
          {activeTab === 'data' && (
            <>
              <div className="dashboard-header">
                <h1 className="dashboard-title">{workspace.name}</h1>
                <p className="dashboard-subtitle">@{workspace.slug}</p>
              </div>
              <WorkspaceTables workspaceId={workspace.id} onNavigate={navigate} />
            </>
          )}
          {activeTab === 'settings' && (
            <SettingsContent workspaceId={workspace.id} />
          )}
        </div>
      </main>
    </>
  )
}

function WorkspaceTables({ workspaceId, onNavigate }) {
  const [tables, setTables] = useState([])
  const [showModal, setShowModal] = useState(false)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
      .then(res => setTables(res.data))
      .catch(() => setTables([]))
      .finally(() => setLoading(false))
  }, [workspaceId])

  const createTable = async (name) => {
    const slug = name.toLowerCase().replace(/\s+/g, '_')
    await axios.post(`${API_URL}/workspaces/${workspaceId}/tables`, { name, slug })
    const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
    setTables(res.data)
    setShowModal(false)
  }

  if (loading) return <div className="loading"><div className="spinner"></div></div>

  return (
    <div className="tables-section">
      <div className="flex justify-between items-center" style={{ marginBottom: 16 }}>
        <h3 className="section-title" style={{ margin: 0 }}>Tablas</h3>
        <button className="btn btn-primary btn-sm" onClick={() => setShowModal(true)}>
          + Nueva tabla
        </button>
      </div>

      {tables.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">📋</div>
          <h3 className="empty-state-title">Sin tablas</h3>
          <p className="empty-state-description">Crea tu primera tabla para empezar</p>
        </div>
      ) : (
        tables.map(tbl => (
          <div key={tbl.id} className="table-item" onClick={() => onNavigate(`/workspace/${workspaceId}/tables/${tbl.id}`)}>
            <div>
              <div className="table-item-name">{tbl.name}</div>
              <div className="table-item-slug">{tbl.slug}</div>
            </div>
            <span className="table-item-arrow">→</span>
          </div>
        ))
      )}

      {showModal && (
        <CreateTableModal onClose={() => setShowModal(false)} onCreate={createTable} />
      )}
    </div>
  )
}

function CreateTableModal({ onClose, onCreate }) {
  const [name, setName] = useState('')

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">Nueva Tabla</h3>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Nombre</label>
            <input 
              type="text" 
              className="form-input"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Ej: clientes, productos, ventas"
              autoFocus
            />
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancelar</button>
          <button className="btn btn-primary" onClick={() => onCreate(name)} disabled={!name.trim()}>
            Crear
          </button>
        </div>
      </div>
    </div>
  )
}

function CreateWorkspace({ onCreated }) {
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)

  const create = async () => {
    if (!name.trim()) return
    setLoading(true)
    try {
      await axios.post(`${API_URL}/workspaces`, {
        name,
        slug: name.toLowerCase().replace(/\s+/g, '-'),
        owner_id: '00000000-0000-0000-0000-000000000001'
      })
      onCreated()
    } catch (err) {
      alert('Error al crear workspace')
    }
    setLoading(false)
  }

  if (!show) {
    return (
      <button className="btn btn-primary" onClick={() => setShow(true)}>
        Crear mi primer workspace
      </button>
    )
  }

  return (
    <div className="modal-overlay" onClick={() => setShow(false)}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">Nuevo Workspace</h3>
          <button className="btn btn-ghost btn-sm" onClick={() => setShow(false)}>✕</button>
        </div>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Nombre</label>
            <input 
              type="text" 
              className="form-input"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Ej: Mi Empresa"
              autoFocus
            />
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={() => setShow(false)}>Cancelar</button>
          <button className="btn btn-primary" onClick={create} disabled={!name.trim() || loading}>
            {loading ? 'Creando...' : 'Crear'}
          </button>
        </div>
      </div>
    </div>
  )
}

function Tables({ onLogout }) {
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')

  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(res.data))
      .catch(() => setWorkspaces([]))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="loading"><div className="spinner"></div></div>

  if (workspaces.length === 0) {
    return (
      <>
        <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
        <main className="dashboard">
          <div className="container">
            <div className="dashboard-header">
              <h1 className="dashboard-title">Tablas</h1>
            </div>
            <p className="text-muted">Primero creá un workspace</p>
          </div>
        </main>
      </>
    )
  }

  const workspace = workspaces[0]

  return (
    <>
      <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
      <main className="dashboard">
        <div className="container">
          {activeTab === 'data' && (
            <WorkspaceTables workspaceId={workspace.id} onNavigate={navigate} />
          )}
          {activeTab === 'settings' && (
            <SettingsContent workspaceId={workspace.id} />
          )}
        </div>
      </main>
    </>
  )
}

function TableView({ onLogout }) {
  const params = useParams()
  const navigate = useNavigate()
  const workspaceId = params.workspaceId
  const tableId = params.tableId
  
  const [table, setTable] = useState(null)
  const [columns, setColumns] = useState([])
  const [records, setRecords] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')
  const [showModal, setShowModal] = useState(false)

  useEffect(() => {
    if (tableId && workspaceId) {
      loadData()
    } else if (tableId) {
      loadDataSimple()
    }
  }, [tableId, workspaceId])

  const loadData = async () => {
    setLoading(true)
    try {
      const [tableRes, columnsRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`)
      ])
      setTable(tableRes.data)
      setColumns(columnsRes.data)
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const loadDataSimple = async () => {
    // Get workspace from URL params or default
    setLoading(true)
    const wsRes = await axios.get(`${API_URL}/workspaces`)
    if (wsRes.data.length === 0) {
      setLoading(false)
      return
    }
    const wsId = wsRes.data[0].id
    
    try {
      const [tableRes, columnsRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${wsId}/tables/${tableId}`),
        axios.get(`${API_URL}/workspaces/${wsId}/tables/${tableId}/columns`)
      ])
      setTable(tableRes.data)
      setColumns(columnsRes.data)
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const loadRecords = async () => {
    if (!table) return
    const wsId = workspaceId || (await axios.get(`${API_URL}/workspaces`)).data[0]?.id
    if (!wsId) return
    try {
      const res = await axios.get(`${API_URL}/workspaces/${wsId}/data/${table.slug}`)
      setRecords(res.data.data || [])
    } catch (err) {
      console.error(err)
    }
  }

  useEffect(() => {
    if (table && activeTab === 'data') {
      loadRecords()
    }
  }, [table, activeTab])

  const createRecord = async (data) => {
    const wsId = workspaceId || (await axios.get(`${API_URL}/workspaces`)).data[0]?.id
    await axios.post(`${API_URL}/workspaces/${wsId}/data/${table.slug}`, data)
    loadRecords()
    setShowModal(false)
  }

  const createColumn = async (name, fieldType) => {
    const wsId = workspaceId || (await axios.get(`${API_URL}/workspaces`)).data[0]?.id
    const slug = name.toLowerCase().replace(/\s+/g, '_')
    await axios.post(`${API_URL}/workspaces/${wsId}/tables/${tableId}/columns`, { name, slug, field_type: fieldType })
    loadData()
  }

  if (loading) return <div className="loading"><div className="spinner"></div></div>

  return (
    <>
      <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
      <main className="dashboard">
        <div className="container">
          <div className="flex items-center gap-4 mb-4">
            <button className="btn btn-ghost" onClick={() => navigate(-1)}>←</button>
            <h2>{table?.name || 'Tabla'}</h2>
          </div>

          <div className="tabs">
            <button className={`tab ${activeTab === 'data' ? 'active' : ''}`} onClick={() => setActiveTab('data')}>
              Datos
            </button>
            <button className={`tab ${activeTab === 'columns' ? 'active' : ''}`} onClick={() => setActiveTab('columns')}>
              Columnas
            </button>
          </div>

          {activeTab === 'data' && (
            <>
              <div className="flex justify-between items-center mb-4">
                <span className="text-muted text-sm">{records.length} registros</span>
                <button className="btn btn-primary btn-sm" onClick={() => setShowModal(true)}>
                  + Nuevo registro
                </button>
              </div>
              {records.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-state-icon">📝</div>
                  <h3 className="empty-state-title">Sin datos</h3>
                </div>
              ) : (
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>ID</th>
                      {columns.map(col => <th key={col.id}>{col.name}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((r, i) => (
                      <tr key={i}>
                        <td><code className="text-sm text-muted">{r.id?.slice(0,8)}</code></td>
                        {columns.map(col => <td key={col.id}>{String(r[col.slug] || '-')}</td>)}
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </>
          )}

          {activeTab === 'columns' && (
            <>
              <div className="flex justify-between items-center mb-4">
                <span className="text-muted text-sm">{columns.length} columnas</span>
                <button className="btn btn-primary btn-sm" onClick={() => setShowModal('column')}>
                  + Nueva columna
                </button>
              </div>
              {columns.map(col => (
                <div key={col.id} className="setting-item">
                  <div className="setting-item-info">
                    <h4>{col.name}</h4>
                    <p>{col.slug}</p>
                  </div>
                  <span className="badge badge-success">{col.field_type}</span>
                </div>
              ))}
            </>
          )}

          {showModal === 'column' && (
            <CreateColumnModal onClose={() => setShowModal(false)} onCreate={createColumn} />
          )}

          {showModal === true && (
            <CreateRecordModal columns={columns} onClose={() => setShowModal(false)} onCreate={createRecord} />
          )}
        </div>
      </main>
    </>
  )
}

function CreateColumnModal({ onClose, onCreate }) {
  const [name, setName] = useState('')
  const [type, setType] = useState('text')

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">Nueva Columna</h3>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          <div className="form-group">
            <label className="form-label">Nombre</label>
            <input type="text" className="form-input" value={name} onChange={e => setName(e.target.value)} />
          </div>
          <div className="form-group">
            <label className="form-label">Tipo</label>
            <select className="form-select" value={type} onChange={e => setType(e.target.value)}>
              <option value="text">Texto</option>
              <option value="long_text">Texto largo</option>
              <option value="number">Número</option>
              <option value="boolean">Booleano</option>
              <option value="date">Fecha</option>
              <option value="datetime">Fecha y hora</option>
              <option value="email">Email</option>
            </select>
          </div>
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancelar</button>
          <button className="btn btn-primary" onClick={() => onCreate(name, type)} disabled={!name.trim()}>Crear</button>
        </div>
      </div>
    </div>
  )
}

function CreateRecordModal({ columns, onClose, onCreate }) {
  const [data, setData] = useState({})

  const handleSubmit = () => {
    onCreate(data)
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3 className="modal-title">Nuevo Registro</h3>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>✕</button>
        </div>
        <div className="modal-body">
          {columns.map(col => (
            <div className="form-group" key={col.id}>
              <label className="form-label">{col.name}</label>
              <input 
                type={col.field_type === 'number' ? 'number' : 'text'} 
                className="form-input"
                value={data[col.slug] || ''}
                onChange={e => setData({ ...data, [col.slug]: e.target.value })}
              />
            </div>
          ))}
        </div>
        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancelar</button>
          <button className="btn btn-primary" onClick={handleSubmit}>Crear</button>
        </div>
      </div>
    </div>
  )
}

function Settings({ onLogout }) {
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')

  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(res.data))
      .catch(() => setWorkspaces([]))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="loading"><div className="spinner"></div></div>

  if (workspaces.length === 0) {
    return (
      <>
        <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
        <main className="dashboard">
          <div className="container">
            <p className="text-muted">Primero creá un workspace</p>
          </div>
        </main>
      </>
    )
  }

  return (
    <>
      <Header onLogout={onLogout} activeTab={activeTab} setActiveTab={setActiveTab} user={user} />
      <main className="dashboard">
        <div className="container">
          <SettingsContent workspaceId={workspaces[0].id} />
        </div>
      </main>
    </>
  )
}

function SettingsContent({ workspaceId }) {
  const [activeSection, setActiveSection] = useState('roles')
  const [roles, setRoles] = useState([])
  const [apiKeys, setAPIKeys] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
  }, [workspaceId, activeSection])

  const loadData = async () => {
    setLoading(true)
    try {
      if (activeSection === 'roles') {
        const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/roles`)
        setRoles(res.data)
      } else if (activeSection === 'keys') {
        const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/keys`)
        setAPIKeys(res.data)
      }
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  const createRole = async (name, description) => {
    await axios.post(`${API_URL}/workspaces/${workspaceId}/roles`, {
      name,
      description,
      permissions: '{}'
    })
    loadData()
  }

  const createAPIKey = async (name) => {
    const res = await axios.post(`${API_URL}/workspaces/${workspaceId}/keys`, {
      name,
      expires_in_days: 365
    })
    alert(`API Key creada: ${res.data.key}\n¡Guardala bien, no se puede recuperar!`)
    loadData()
  }

  const deleteAPIKey = async (id) => {
    if (!confirm('¿Eliminar esta API key?')) return
    await axios.delete(`${API_URL}/workspaces/${workspaceId}/keys/${id}`)
    loadData()
  }

  return (
    <div>
      <div className="dashboard-header">
        <h1 className="dashboard-title">Configuración</h1>
      </div>

      <div className="tabs">
        <button className={`tab ${activeSection === 'roles' ? 'active' : ''}`} onClick={() => setActiveSection('roles')}>
          Roles de Seguridad
        </button>
        <button className={`tab ${activeSection === 'keys' ? 'active' : ''}`} onClick={() => setActiveSection('keys')}>
          API Keys
        </button>
      </div>

      {activeSection === 'roles' && (
        <div className="settings-section">
          <div className="flex justify-between items-center mb-4">
            <p className="text-muted">Define roles de seguridad como en Dataverse</p>
            <button className="btn btn-primary btn-sm" onClick={() => createRole('Nuevo Rol', 'Descripción')}>
              + Nuevo Rol
            </button>
          </div>
          {roles.length === 0 ? (
            <div className="empty-state">
              <div className="empty-state-icon">🔐</div>
              <h3 className="empty-state-title">Sin roles</h3>
              <p className="empty-state-description">Los roles definen qué pueden hacer los usuarios</p>
            </div>
          ) : (
            roles.map(role => (
              <div key={role.id} className="setting-item">
                <div className="setting-item-info">
                  <h4>{role.name}</h4>
                  <p>{role.description || 'Sin descripción'}</p>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {activeSection === 'keys' && (
        <div className="settings-section">
          <div className="flex justify-between items-center mb-4">
            <p className="text-muted">Genera API keys para acceder programáticamente</p>
            <button className="btn btn-primary btn-sm" onClick={() => createAPIKey('Mi Key')}>
              + Nueva API Key
            </button>
          </div>
          {apiKeys.length === 0 ? (
            <div className="empty-state">
              <div className="empty-state-icon">🔑</div>
              <h3 className="empty-state-title">Sin API keys</h3>
              <p className="empty-state-description">Crea una key para usar la API</p>
            </div>
          ) : (
            apiKeys.map(key => (
              <div key={key.id} className="setting-item">
                <div className="setting-item-info">
                  <h4>{key.name}</h4>
                  <p>{key.masked_key}</p>
                </div>
                <button className="btn btn-ghost btn-sm" onClick={() => deleteAPIKey(key.id)}>Eliminar</button>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  )
}

function Workspace({ onLogout }) {
  const { workspaceId } = useParams()
  const navigate = useNavigate()
  const [workspace, setWorkspace] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (workspaceId) {
      axios.get(`${API_URL}/workspaces/${workspaceId}`)
        .then(res => setWorkspace(res.data))
        .catch(console.error)
        .finally(() => setLoading(false))
    }
  }, [workspaceId])

  if (loading) return <div className="loading"><div className="spinner"></div></div>

  return (
    <>
      <Header onLogout={onLogout} activeTab="data" setActiveTab={() => {}} />
      <main className="dashboard">
        <div className="container">
          <div className="dashboard-header">
            <h1 className="dashboard-title">{workspace?.name}</h1>
            <p className="dashboard-subtitle">@{workspace?.slug}</p>
          </div>
          <WorkspaceTables workspaceId={workspaceId} onNavigate={(id) => navigate(`/workspace/${workspaceId}/tables/${id}`)} />
        </div>
      </main>
    </>
  )
}

export default App
