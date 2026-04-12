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
  const [isNamingModalOpen, setIsNamingModalOpen] = useState(false)
  const [importFile, setImportFile] = useState(null)
  const [canCreateWorkspaces, setCanCreateWorkspaces] = useState(false)

  const importInputRef = useRef(null)
  
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)
  const [templates, setTemplates] = useState([])
  const [selectedTemplate, setSelectedTemplate] = useState(null)

  // Fetch workspaces and user permissions
  useEffect(() => {
    // Fetch workspaces
    axios.get(`${API_URL}/workspaces`)
      .then(res => setWorkspaces(Array.isArray(res.data.data) ? res.data.data : []))
      .catch((err) => {
        setWorkspaces([])
        notify(t('error_fetching_workspaces'), 'error')
      })
      .finally(() => setLoading(false))

    // Fetch current user permissions
    axios.get(`${API_URL}/auth/me`)
      .then(res => {
        if (res.data.data) {
          setCanCreateWorkspaces(res.data.data.can_create_workspaces || false)
        }
      })
      .catch(() => {
        setCanCreateWorkspaces(false)
      })

    // Fetch templates
    axios.get(`${API_URL}/templates`)
      .then(res => {
        setTemplates(Array.isArray(res.data.data) ? res.data.data : [])
      })
      .catch((err) => {
        console.error('Error fetching templates:', err)
        setTemplates([])
      })
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
      if (selectedTemplate) {
        // Fetch template data and import it under the new name
        const tplRes = await axios.get(`${API_URL}/templates/${selectedTemplate.filename}`)
        const dump = tplRes.data.data
        // Override workspace name/slug with user input
        dump.workspace.name = newName
        dump.workspace.slug = newName.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
        dump.workspace.owner_id = ownerId
        await axios.post(`${API_URL}/workspaces/import`, dump)
      } else {
        await axios.post(`${API_URL}/workspaces`, {
          name: newName,
          slug: newName.toLowerCase().replace(/\s+/g, '-'),
          owner_id: ownerId
        })
      }
      setShowCreate(false)
      setNewName('')
      setSelectedTemplate(null)
      const res = await axios.get(`${API_URL}/workspaces`)
      setWorkspaces(Array.isArray(res.data.data) ? res.data.data : [])
    } catch (err) {
      console.error('Workspace Create Error:', err)
      notify(t('error_create_workspace'), 'error')
    } finally {
      setCreating(false)
    }
  }

  const handleImportWorkspace = async (e) => {
    const file = e.target.files?.[0]
    if (!file) return

    const ownerId = user?.id || user?.user_id
    if (!ownerId) {
      notify(t('error_import_workspace'), 'error')
      return
    }

    const reader = new FileReader()
    reader.onload = async (event) => {
      try {
        const jsonDump = JSON.parse(event.target.result)
        setLoading(true)
        await axios.post(`${API_URL}/workspaces/import`, jsonDump)
        
        notify(t('workspace_imported'), 'success')
        const res = await axios.get(`${API_URL}/workspaces`)
        setWorkspaces(Array.isArray(res.data.data) ? res.data.data : [])
      } catch (err) {
        console.error('Import Workspace Error:', err)
        notify(t('error_import_workspace'), 'error')
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
              <Button 
                variant="secondary" 
                onClick={() => importInputRef.current?.click()} 
                disabled={!canCreateWorkspaces}
                title={!canCreateWorkspaces ? t('admin_only_feature') : ''}
                data-tour="import-workspace"
                style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.5rem', opacity: canCreateWorkspaces ? 1 : 0.5 }}
              >
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" /></svg>
                <span className="hidden sm:inline">{t('import_workspace')}</span>
              </Button>
              <Button 
                onClick={() => setShowCreate(true)} 
                disabled={!canCreateWorkspaces}
                title={!canCreateWorkspaces ? t('admin_only_feature') : ''}
                data-tour="create-workspace"
                style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.5rem', opacity: canCreateWorkspaces ? 1 : 0.5 }}
              >
                <span style={{ fontSize: '1.25rem', lineHeight: 1 }}>+</span>
                <span className="hidden sm:inline">{t('new_workspace_button')}</span>
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
              <Button 
                onClick={() => setShowCreate(true)}
                disabled={!canCreateWorkspaces}
                title={!canCreateWorkspaces ? t('admin_only_feature') : ''}
                style={{ opacity: canCreateWorkspaces ? 1 : 0.5 }}
              >
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
                    className="card cursor-pointer workspace-card"
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
                      <div style={{ minWidth: 0, flex: 1, paddingRight: '2.5rem' }}>
                        <div 
                          style={{ fontWeight: 800, fontSize: '1.0625rem', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}
                          title={ws.name}
                        >
                          {ws.name}
                        </div>
                        <div 
                          style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}
                          title={`@${ws.slug}`}
                        >
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
        <div className="modal-overlay" onClick={() => { setShowCreate(false); setSelectedTemplate(null); setNewName(''); }}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '560px' }}>
            <div className="modal-header">
              <h3 className="modal-title">{t('new_workspace_title')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => { setShowCreate(false); setSelectedTemplate(null); setNewName(''); }} style={{ borderRadius: '8px' }}>
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

              {templates.length > 0 && (
                <div style={{ marginTop: '1.25rem' }}>
                  <label className="form-label" style={{ marginBottom: '0.75rem', display: 'block' }}>
                    {t('template_label')}
                  </label>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(150px, 1fr))', gap: '0.625rem' }}>
                    <div
                      onClick={() => setSelectedTemplate(null)}
                      style={{
                        padding: '0.75rem',
                        borderRadius: '10px',
                        border: `2px solid ${!selectedTemplate ? 'var(--primary)' : 'var(--border)'}`,
                        background: !selectedTemplate ? 'var(--primary-light)' : 'var(--bg-subtle)',
                        cursor: 'pointer',
                        textAlign: 'center',
                        fontSize: '0.8125rem',
                        transition: 'all 0.15s ease'
                      }}
                    >
                      <div style={{ fontSize: '1.5rem', marginBottom: '0.25rem' }}>⬜</div>
                      <div style={{ fontWeight: 600 }}>{t('blank_workspace')}</div>
                    </div>
                    {templates.map(tpl => (
                      <div
                        key={tpl.filename}
                        onClick={() => setSelectedTemplate(tpl)}
                        style={{
                          padding: '0.75rem',
                          borderRadius: '10px',
                          border: `2px solid ${selectedTemplate?.filename === tpl.filename ? 'var(--primary)' : 'var(--border)'}`,
                          background: selectedTemplate?.filename === tpl.filename ? 'var(--primary-light)' : 'var(--bg-subtle)',
                          cursor: 'pointer',
                          textAlign: 'center',
                          fontSize: '0.8125rem',
                          transition: 'all 0.15s ease'
                        }}
                      >
                        <div style={{ fontSize: '1.5rem', marginBottom: '0.25rem' }}>{tpl.icon}</div>
                        <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>{tpl.name}</div>
                        <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem', lineHeight: 1.3 }}>{tpl.description}</div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
            <div className="modal-footer">
              <Button variant="secondary" onClick={() => { setShowCreate(false); setSelectedTemplate(null); setNewName(''); }}>{t('cancel')}</Button>
              <Button onClick={handleCreate} loading={creating} disabled={!newName.trim()}>{t('create')}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
