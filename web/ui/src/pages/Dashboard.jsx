import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { motion } from 'framer-motion'
import axios from 'axios'
import { Folder, EditPencil, Trash, Xmark } from 'iconoir-react'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button, Badge } from '../components/index.jsx'
import { notify } from '../components/Toast.jsx'
import TopNavbar from '../components/TopNavbar.jsx'

export default function Dashboard() {
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [workspaces, setWorkspaces] = useState([])
  const [loading, setLoading] = useState(true)
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [newWorkspaceName, setNewWorkspaceName] = useState('')
  const [isNamingModalOpen, setIsNamingModalOpen] = useState(false) // This state is not used in the provided snippet, but keeping it as per diff
  const [importFile, setImportFile] = useState(null)

  const importInputRef = useRef(null)
  
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)
  useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(Array.isArray(res.data.data) ? res.data.data : []))
      .catch((err) => {
        setWorkspaces([])
        notify(t('error_fetching_workspaces') || 'Error fetching workspaces', 'error')
      })
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
      <TopNavbar />

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
