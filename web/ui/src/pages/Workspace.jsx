import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { motion } from 'framer-motion'
import axios from 'axios'
import { ClipboardCheck, Table2Columns, EditPencil, Trash } from 'iconoir-react'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button } from '../components/index.jsx'
import { notify } from '../components/Toast.jsx'
import TopNavbar from '../components/TopNavbar.jsx'

export default function Workspace() {
  const { user, logout } = useAuth()
  const { workspaceId } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [workspace, setWorkspace] = useState(null)
  const [tables, setTables] = useState([])
  const [loading, setLoading] = useState(true)
  const [showCreateTable, setShowCreateTable] = useState(false)
  const [newTableName, setNewTableName] = useState('')
  const [creatingTable, setCreatingTable] = useState(false)

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
    if (!newTableName.trim()) return
    setCreatingTable(true)
    try {
      const wsId = workspaceId || workspace?.id
      await axios.post(`${API_URL}/workspaces/${wsId}/tables`, {
        name: newTableName,
        slug: newTableName.toLowerCase().replace(/\s+/g, '_')
      })
      setShowCreateTable(false)
      setNewTableName('')
      const res = await axios.get(`${API_URL}/workspaces/${wsId}/tables`)
      setTables(res.data.data)
    } catch (err) {
      console.error(err)
      notify(t('error_create_table'), 'error')
    } finally {
      setCreatingTable(false)
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
      <TopNavbar workspaceId={wsId} />

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
                          value={newTableName}
                          onChange={e => setNewTableName(e.target.value)}
                          onKeyDown={e => {
                            if (e.key === 'Enter' && newTableName.trim()) {
                              handleCreateTable(e);
                            } else if (e.key === 'Escape') {
                              setShowCreateTable(false);
                              setNewTableName('');
                            }
                          }}
                          placeholder={t('table_name_placeholder')}
                          autoFocus
                          style={{ marginBottom: '0.25rem' }}
                        />
                        <p className="form-hint" style={{ fontSize: '0.75rem', textAlign: 'left', margin: 0, paddingLeft: '2px' }}>
                          {t('will_be_created_as')} <code style={{ fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>{newTableName.toLowerCase().replace(/\s+/g, '_') || '...'}</code>
                        </p>
                      </div>
                      <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                        <Button variant="secondary" size="sm" onClick={(e) => { e.stopPropagation(); setShowCreateTable(false); setNewTableName(''); }}>{t('cancel')}</Button>
                        <Button size="sm" onClick={(e) => { e.stopPropagation(); handleCreateTable(e); }} loading={creatingTable} disabled={!newTableName.trim()}>{t('create')}</Button>
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
