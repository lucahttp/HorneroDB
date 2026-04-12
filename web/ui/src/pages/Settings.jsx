import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import {
  Settings as SettingsIcon, ShieldCheck, Key, Trash, Xmark,
  Group, ClipboardCheck
} from 'iconoir-react'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button, Badge } from '../components/index.jsx'
import { PermissionMatrix } from '../components/PermissionMatrix.jsx'
import SettingsUsers from '../components/SettingsUsers'
import { notify } from '../components/Toast.jsx'
import TopNavbar from '../components/TopNavbar.jsx'

export default function Settings() {
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  const { workspaceId } = useParams()
  const navigate = useNavigate()

  const [workspace, setWorkspace] = useState(null)
  const [roles, setRoles] = useState([])
  const [users, setUsers] = useState([])
  const [apiKeys, setApiKeys] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeSection, setActiveSection] = useState('general')
  const [rateLimit, setRateLimit] = useState(60)
  const [allowedOrigins, setAllowedOrigins] = useState('')
  const [savingGeneral, setSavingGeneral] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [tables, setTables] = useState([])

  const [showCreateRole, setShowCreateRole] = useState(false)
  const [showCreateKey, setShowCreateKey] = useState(false)
  const [selectedKey, setSelectedKey] = useState(null)
  const [selectedRoleId, setSelectedRoleId] = useState(null)
  const [newRoleName, setNewRoleName] = useState('')
  const [newKeyName, setNewKeyName] = useState('')
  const [newKeyRole, setNewKeyRole] = useState('')
  const [rotatedKeyData, setRotatedKeyData] = useState(null)
  const [createdKeyModal, setCreatedKeyModal] = useState(null)
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
        setApiKeys(keysRes.data.data)
      }
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  useEffect(() => {
    loadData()
  }, [workspaceId, activeSection])

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
      notify(t('settings_saved'), 'success')
      const wsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}`)
      setWorkspace(wsRes.data.data)
    } catch (err) {
      console.error(err)
      notify(t('error_save_settings'), 'error')
    }
    setSavingGeneral(false)
  }

  const handleExportWorkspace = async () => {
    setExporting(true)
    try {
      const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/export`)
      const dataStr = "data:text/json;charset=utf-8," + encodeURIComponent(JSON.stringify(res.data.data, null, 2))
      const downloadAnchorNode = document.createElement('a')
      downloadAnchorNode.setAttribute("href", dataStr)
      downloadAnchorNode.setAttribute("download", `workspace_${workspace?.slug || workspaceId}_export.json`)
      document.body.appendChild(downloadAnchorNode)
      downloadAnchorNode.click()
      downloadAnchorNode.remove()
      notify(t('workspace_exported'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_export_workspace'), 'error')
    }
    setExporting(false)
  }

  const handleCreateRole = async () => {
    if (!newRoleName.trim()) return
    try {
      const resp = await axios.post(`${API_URL}/workspaces/${workspaceId}/roles`, {
        name: newRoleName,
        description: t('new_role'),
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
      notify(t('api_key_updated'), 'success')
      loadData()
      setSelectedKey(null)
    } catch (err) {
      console.error(err)
      notify(t('error_update_api_key'), 'error')
    }
    setSavingKey(false)
  }

  const handleRotateKey = async (keyId, keyName) => {
    if (!confirm(t('confirm_rotate_key', { name: keyName }))) return;
    try {
      const res = await axios.post(`${API_URL}/workspaces/${workspaceId}/keys/${keyId}/rotate`)
      setRotatedKeyData(res.data.data)
      loadData()
      notify(t('api_key_rotated'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rotate_api_key'), 'error')
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
      <TopNavbar workspaceId={workspaceId} />

      <div className="main-content">
        <div className="main-body">
          <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em', marginBottom: '1.5rem', display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <SettingsIcon width="2rem" height="2rem" /> {t('settings')}
          </h1>

          <div className="tabs">
            <button
              className={`tab ${activeSection === 'general' ? 'active' : ''}`}
              onClick={() => setActiveSection('general')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <SettingsIcon width="1rem" height="1rem" /> {t('general')}
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
              <h2 style={{ fontSize: '1.25rem', fontWeight: 700, marginBottom: '1rem' }}>{t('workspace_security')}</h2>

              <div className="form-group">
                <label className="form-label">{t('rate_limit')}</label>
                <input
                  type="number"
                  className="form-input"
                  value={rateLimit}
                  onChange={e => setRateLimit(e.target.value)}
                  min="0"
                />
                <p className="form-hint" style={{ marginTop: '0.25rem' }}>{t('rate_limit_hint')}</p>
              </div>

              <div className="form-group" style={{ marginTop: '1.5rem' }}>
                <label className="form-label">{t('allowed_origins')}</label>
                <textarea
                  className="form-input"
                  value={allowedOrigins}
                  onChange={e => setAllowedOrigins(e.target.value)}
                  placeholder="https://example.com, https://app.example.com"
                  rows={3}
                  style={{ fontFamily: 'var(--font-mono)' }}
                />
                <p className="form-hint" style={{ marginTop: '0.25rem' }}>{t('allowed_origins_hint')}</p>
              </div>

              <Button onClick={handleSaveGeneral} loading={savingGeneral} style={{ marginTop: '1rem' }}>
                {t('save_changes')}
              </Button>

              <div style={{ marginTop: '2rem', paddingTop: '1.5rem', borderTop: '1px solid var(--border-light)' }}>
                <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('export_workspace')}</h3>
                <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem', fontSize: '0.875rem' }}>
                  {t('export_workspace_hint')}
                </p>
                <Button variant="secondary" onClick={handleExportWorkspace} loading={exporting} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
                  {t('export_button')}
                </Button>
              </div>

              <div style={{ marginTop: '2.5rem', paddingTop: '1.5rem', borderTop: 'var(--border-thick) solid var(--border-subtle)' }}>
                <h3 style={{ fontSize: '1.125rem', fontWeight: 800, marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <svg width="1.2rem" height="1.2rem" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 12c0 1.657-4.03 3-9 3s-9-1.343-9-3 4.03-3 9-3 9 1.343 9 3z"></path><path d="M3 12v6c0 1.657 4.03 3 9 3s9-1.343 9-3v-6"></path><path d="M21 7c0 1.657-4.03 3-9 3s-9-1.343-9-3 4.03-3 9-3 9 1.343 9 3z"></path></svg>
                  {t('language_stats')}
                </h3>
                <div className="table-container" style={{ borderRadius: 'var(--radius-lg)', overflow: 'hidden', border: '1px solid var(--border-light)' }}>
                  <table style={{ width: '100%', fontSize: '0.85rem', borderCollapse: 'collapse' }}>
                    <thead style={{ background: 'var(--bg-subtle)', borderBottom: '1px solid var(--border-light)' }}>
                      <tr>
                        <th style={{ padding: '0.75rem', textAlign: 'left', fontWeight: 700 }}>{t('rank')}</th>
                        <th style={{ padding: '0.75rem', textAlign: 'left', fontWeight: 700 }}>{t('language')}</th>
                        <th style={{ padding: '0.75rem', textAlign: 'right', fontWeight: 700 }}>{t('internet_users')}</th>
                        <th style={{ padding: '0.75rem', textAlign: 'right', fontWeight: 700 }}>%</th>
                      </tr>
                    </thead>
                    <tbody style={{ color: 'var(--text-secondary)' }}>
                      {[
                        { r: 1, l: 'English', u: '1,186,451,052', p: '25.9%' },
                        { r: 2, l: 'Chinese', u: '888,453,068', p: '19.4%' },
                        { r: 3, l: 'Spanish', u: '363,684,593', p: '7.9%' },
                        { r: 4, l: 'Arabic', u: '237,418,349', p: '5.2%' },
                        { r: 5, l: 'Indonesian', u: '198,029,815', p: '4.3%' },
                        { r: 6, l: 'Portuguese', u: '171,750,818', p: '3.7%' },
                        { r: 7, l: 'French', u: '144,695,288', p: '3.3%' },
                        { r: 8, l: 'Japanese', u: '118,626,672', p: '2.6%' },
                        { r: 9, l: 'Russian', u: '116,353,942', p: '2.5%' },
                        { r: 10, l: 'German', u: '92,525,427', p: '2.0%' }
                      ].map((item, i) => (
                        <tr key={i} style={{ borderBottom: i === 9 ? 'none' : '1px solid var(--border-subtle)', background: i % 2 === 0 ? 'transparent' : 'var(--bg-elevated-subtle)' }}>
                          <td style={{ padding: '0.65rem 0.75rem' }}>{item.r}</td>
                          <td style={{ padding: '0.65rem 0.75rem', fontWeight: 600, color: 'var(--text)' }}>{item.l}</td>
                          <td style={{ padding: '0.65rem 0.75rem', textAlign: 'right' }}>{item.u}</td>
                          <td style={{ padding: '0.65rem 0.75rem', textAlign: 'right' }}>{item.p}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <p style={{ marginTop: '0.75rem', fontSize: '0.75rem', color: 'var(--text-muted)', fontStyle: 'italic' }}>
                  {t('source')}: <a href="https://en.wikipedia.org/wiki/Languages_used_on_the_Internet#Internet_users_by_language" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--primary)', textDecoration: 'none' }}>Wikipedia</a>
                </p>
              </div>
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
                            <Badge variant="primary" style={{ marginTop: '0.75rem' }}>{t('default')}</Badge>
                          )}
                        </div>
                        <Button
                          size="sm"
                          variant={selectedRoleId === role.id ? 'primary' : 'secondary'}
                          onClick={(e) => { e.stopPropagation(); setSelectedRoleId(role.id); }}
                        >
                          {selectedRoleId === role.id ? t('selected') : t('select')}
                        </Button>
                      </div>
                    </div>
                  ))
                )}

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
                          title={t('rotate_key')}
                        >
                          <svg width="1rem" height="1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                        </button>
                        <button
                          onClick={() => openKeySettings(key)}
                          className="btn btn-ghost btn-sm"
                          title={t('edit_key_settings')}
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

              {selectedKey && (
                <div className="card" style={{ marginTop: '1rem', padding: '1.5rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                    <h3 style={{ fontSize: '1.125rem', fontWeight: 700 }}>
                      {t('edit_key_settings')}
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
                        <option value="">{t('select_role')}</option>
                        {roles.map(r => (
                          <option key={r.id} value={r.id}>{r.name}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginTop: '1rem' }}>
                    <div className="form-group">
                      <label className="form-label">{t('rate_limit_per_minute')} & {t('rate_limit_per_hour')}</label>
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
              <h3 className="modal-title">{t('api_key_rotated')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setRotatedKeyData(null)}>
                <Xmark width="1rem" height="1rem" />
              </button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label className="form-label">{t('copy_key_warning')}</label>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <input
                    type="text"
                    className="form-input"
                    value={rotatedKeyData.new_key}
                    readOnly
                  />
                  <Button variant="secondary" onClick={() => {
                    navigator.clipboard.writeText(rotatedKeyData.new_key)
                    notify(t('copied'), 'success')
                  }}>
                    <ClipboardCheck width="1rem" height="1rem" />
                  </Button>
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <Button onClick={() => setRotatedKeyData(null)}>{t('close')}</Button>
            </div>
          </div>
        </div>
      )}

      {createdKeyModal && (
        <div className="modal-overlay" onClick={() => setCreatedKeyModal(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '520px' }}>
            <div className="modal-header">
              <h3 className="modal-title">{t('api_key_created_title')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setCreatedKeyModal(null)}>
                <Xmark width="1rem" height="1rem" />
              </button>
            </div>
            <div className="modal-body">
              <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                {t('copy_key_warning')}
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
                    notify(t('copied'), 'success')
                  }}>
                    <ClipboardCheck width="1rem" height="1rem" />
                  </Button>
                </div>
              </div>
            </div>
            <div className="modal-footer">
              <Button onClick={() => setCreatedKeyModal(null)}>{t('close')}</Button>
            </div>
          </div>
        </div>
      )}

      {showCreateKey && (
        <div className="modal-overlay" onClick={() => setShowCreateKey(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">{t('new_api_key')}</h3>
              <button className="btn btn-ghost btn-sm" onClick={() => setShowCreateKey(false)}>
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
                  <option value="">{t('select_role')}</option>
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
