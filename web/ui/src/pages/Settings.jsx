import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import {
  Settings as SettingsIcon, ShieldCheck, Key, Trash, Xmark,
  Group, ClipboardCheck, Link as LinkIcon, Activity, Refresh
} from 'iconoir-react'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button, Badge } from '../components/index.jsx'
import { PermissionMatrix } from '../components/PermissionMatrix.jsx'
import SettingsUsers from '../components/SettingsUsers'
import { notify } from '../components/toast.js'
import TopNavbar from '../components/TopNavbar.jsx'
import { getSystemPermissions, hasSystemAccess } from '../utils/permissions.js'

/** Event types a webhook can subscribe to (backend `change_type`). */
const WEBHOOK_EVENT_TYPES = ['created', 'updated', 'deleted']

export default function Settings() {
  const { user } = useAuth()
  const { t } = useTranslation()
  const { workspaceId } = useParams()

  const [workspace, setWorkspace] = useState(null)
  const [roles, setRoles] = useState([])
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

  // ── Webhooks ─────────────────────────────────────────
  const [webhooks, setWebhooks] = useState([])
  const [webhooksForbidden, setWebhooksForbidden] = useState(false)
  const [showCreateWebhook, setShowCreateWebhook] = useState(false)
  const [newWebhookUrl, setNewWebhookUrl] = useState('')
  const [newWebhookTable, setNewWebhookTable] = useState('')
  const [newWebhookEvent, setNewWebhookEvent] = useState('created')
  const [newWebhookClientState, setNewWebhookClientState] = useState('')
  const [savingWebhook, setSavingWebhook] = useState(false)

  // ── Webhook delivery history ─────────────────────────
  const [webhookEvents, setWebhookEvents] = useState([])
  const [webhookEventsLoading, setWebhookEventsLoading] = useState(false)
  const [webhookEventsError, setWebhookEventsError] = useState(false)
  const [webhookEventFilter, setWebhookEventFilter] = useState('')
  const [retryingEventId, setRetryingEventId] = useState(null)

  // ── Audit log ─────────────────────────────────────────
  const [auditEntries, setAuditEntries] = useState([])
  const [auditMeta, setAuditMeta] = useState({ count: 0, has_more: false })
  const [auditPage, setAuditPage] = useState(0)
  const [auditLoading, setAuditLoading] = useState(false)
  const [auditError, setAuditError] = useState(false)
  const [auditAction, setAuditAction] = useState('')
  const [auditResourceType, setAuditResourceType] = useState('')

  const fetchWorkspace = useCallback(async () => {
    const wsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}`)
    const data = wsRes.data.data
    const st = data.settings || {}
    return {
      workspace: data,
      rateLimit: st.rate_limit_per_minute ?? 60,
      allowedOrigins: (st.allowed_origins || []).join(', '),
    }
  }, [workspaceId])

  const fetchRolesAndTables = useCallback(async () => {
    const [rolesRes, tablesRes] = await Promise.all([
      // These list endpoints are paginated: ask for enough rows to be sure
      // the role of the current user is part of the answer.
      axios.get(`${API_URL}/workspaces/${workspaceId}/roles`, { params: { limit: 200 } }),
      axios.get(`${API_URL}/workspaces/${workspaceId}/tables`, { params: { limit: 200 } })
    ])
    const tablesWithCols = await Promise.all(tablesRes.data.data.map(async (t) => {
      try {
        const cr = await axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${t.id}/columns`, { params: { limit: 200 } })
        return { ...t, columns: cr.data.data }
      } catch {
        return t
      }
    }))
    return { roles: rolesRes.data.data || [], tables: tablesWithCols }
  }, [workspaceId])

  const fetchApiKeys = useCallback(async () => {
    const keysRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/keys`)
    return keysRes.data.data || []
  }, [workspaceId])

  /** Loads the subscriptions of the workspace. */
  const fetchWebhooks = useCallback(async () => {
    const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/webhooks`)
    return res.data.data || []
  }, [workspaceId])

  /** Loads the delivery history of the workspace webhooks. */
  const fetchWebhookEvents = useCallback(async () => {
    setWebhookEventsLoading(true)
    try {
      const params = { limit: 25 }
      if (webhookEventFilter) params.status = webhookEventFilter
      const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/webhooks/events`, { params })
      setWebhookEvents(res.data.data || [])
      setWebhookEventsError(false)
    } catch (err) {
      console.error(err)
      setWebhookEvents([])
      setWebhookEventsError(true)
    } finally {
      setWebhookEventsLoading(false)
    }
  }, [workspaceId, webhookEventFilter])

  /** Loads the audit log of the workspace. */
  const fetchAudit = useCallback(async () => {
    setAuditLoading(true)
    try {
      const params = { limit: 50, offset: auditPage * 50 }
      if (auditAction) params.action = auditAction
      if (auditResourceType) params.resource_type = auditResourceType
      const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/audit`, { params })
      setAuditEntries(res.data.data || [])
      setAuditMeta(res.data.meta || { count: 0, has_more: false })
      setAuditError(false)
    } catch (err) {
      console.error(err)
      setAuditEntries([])
      setAuditError(true)
    } finally {
      setAuditLoading(false)
    }
  }, [workspaceId, auditPage, auditAction, auditResourceType])

  useEffect(() => {
    let cancelled = false

    const load = async () => {
      const ws = await fetchWorkspace()
      const rt = await fetchRolesAndTables()
      const keys = activeSection === 'keys' ? await fetchApiKeys() : null
      return { ws, rt, keys }
    }

    load()
      .then(({ ws, rt, keys }) => {
        if (cancelled) return
        setWorkspace(ws.workspace)
        setRateLimit(ws.rateLimit)
        setAllowedOrigins(ws.allowedOrigins)
        setRoles(rt.roles)
        setTables(rt.tables)
        setSelectedRoleId(prev => prev || (rt.roles.length > 0 ? rt.roles[0].id : null))
        if (keys) setApiKeys(keys)
      })
      .catch(err => {
        if (cancelled) return
        console.error(err)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => { cancelled = true }
  }, [workspaceId, activeSection, fetchWorkspace, fetchRolesAndTables, fetchApiKeys])

  // Webhooks live in their own request so a 403 (missing `webhooks`
  // permission) does not break the rest of the settings page.
  useEffect(() => {
    if (activeSection !== 'webhooks') return undefined

    let cancelled = false

    fetchWebhooks()
      .then(list => {
        if (cancelled) return
        setWebhooks(list)
        setWebhooksForbidden(false)
      })
      .catch(err => {
        if (cancelled) return
        console.error(err)
        setWebhooks([])
        setWebhooksForbidden(err.response?.status === 403)
      })

    return () => { cancelled = true }
  }, [activeSection, fetchWebhooks])

  // The delivery history lives in its own request: a missing `webhooks`
  // permission must not break the subscription list.
  useEffect(() => {
    if (activeSection !== 'webhooks') return undefined
    fetchWebhookEvents()
    return undefined
  }, [activeSection, fetchWebhookEvents])

  // The audit log is only loaded when its tab is open.
  useEffect(() => {
    if (activeSection !== 'audit') return undefined
    fetchAudit()
    return undefined
  }, [activeSection, fetchAudit])

  const handleSectionChange = (section) => {
    setLoading(true)
    setActiveSection(section)
  }

  /** Reloads the workspace, the roles and the tables (used after a mutation). */
  const reload = useCallback(async () => {
    try {
      const ws = await fetchWorkspace()
      const rt = await fetchRolesAndTables()
      setWorkspace(ws.workspace)
      setRateLimit(ws.rateLimit)
      setAllowedOrigins(ws.allowedOrigins)
      setRoles(rt.roles)
      setTables(rt.tables)
      setSelectedRoleId(prev => prev || (rt.roles.length > 0 ? rt.roles[0].id : null))
      if (activeSection === 'keys') {
        setApiKeys(await fetchApiKeys())
      }
    } catch (err) {
      console.error(err)
    }
  }, [fetchWorkspace, fetchRolesAndTables, fetchApiKeys, activeSection])

  const reloadWebhooks = useCallback(async () => {
    try {
      const list = await fetchWebhooks()
      setWebhooks(list)
    } catch (err) {
      console.error(err)
    }
  }, [fetchWebhooks])

  const handleCreateWebhook = async () => {
    if (!newWebhookUrl.trim() || !newWebhookTable) return
    setSavingWebhook(true)
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/webhooks`, {
        resource: newWebhookTable,
        change_type: newWebhookEvent,
        notification_url: newWebhookUrl.trim(),
        client_state: newWebhookClientState.trim() || undefined
      })
      setShowCreateWebhook(false)
      setNewWebhookUrl('')
      setNewWebhookTable('')
      setNewWebhookEvent('created')
      setNewWebhookClientState('')
      notify(t('webhook_created'), 'success')
      reloadWebhooks()
    } catch (err) {
      console.error(err)
      notify(t('error_create_webhook'), 'error')
    }
    setSavingWebhook(false)
  }

  const deleteWebhook = async (id) => {
    if (!confirm(t('confirm_delete_webhook'))) return
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/webhooks/${id}`)
      notify(t('webhook_deleted'), 'success')
      reloadWebhooks()
    } catch (err) {
      console.error(err)
      notify(t('error_delete_webhook'), 'error')
    }
  }

  const webhookUrlValid = /^https:\/\/\S+$/i.test(newWebhookUrl.trim())

  /**
   * Asks the API to deliver a failed event again and reloads the history.
   */
  const handleRetryWebhookEvent = async (eventId) => {
    setRetryingEventId(eventId)
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/webhooks/events/${eventId}/retry`)
      notify(t('webhook_retry_success'), 'success')
      await fetchWebhookEvents()
    } catch (err) {
      console.error(err)
      notify(t('webhook_retry_error'), 'error')
    } finally {
      setRetryingEventId(null)
    }
  }

  /** Human readable state of a webhook delivery. */
  const webhookEventStatus = (status) => {
    const key = `webhook_status_${String(status || '').toLowerCase()}`
    return t(key, { defaultValue: status || '—' })
  }

  /** Badge variant that matches the state of a webhook delivery. */
  const webhookEventVariant = (status) => {
    switch (String(status || '').toLowerCase()) {
      case 'failed': return 'error'
      case 'completed':
      case 'delivered':
      case 'success': return 'success'
      case 'pending':
      case 'retrying': return 'warning'
      default: return 'gray'
    }
  }

  /** Formats an ISO timestamp for the audit log and the delivery history. */
  const formatTimestamp = (value) => {
    if (!value) return '—'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return String(value)
    return date.toLocaleString()
  }

  /** Distinct actions present in the loaded audit page (filter options). */
  const auditActions = useMemo(
    () => [...new Set(auditEntries.map(e => e.action).filter(Boolean))].sort(),
    [auditEntries]
  )

  /** Distinct resource types present in the loaded audit page. */
  const auditResourceTypes = useMemo(
    () => [...new Set(auditEntries.map(e => e.resource_type).filter(Boolean))].sort(),
    [auditEntries]
  )

  // ── System permissions of the current role (CLS) ─────
  const currentRole = roles.find(r => r.name === user?.role)
  const systemPermissions = getSystemPermissions(currentRole?.permissions, user?.role)
  const canManageWebhooks = hasSystemAccess(systemPermissions, 'webhooks', 'manage')

  /** Name of the table a webhook is subscribed to. */
  const webhookTableName = (resource) => {
    const table = tables.find(t => t.id === resource)
    return table ? table.name : resource
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
      reload()
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
      reload()
    } catch (err) {
      console.error(err)
      notify(t('error_create_api_key'), 'error')
    }
  }

  const deleteAPIKey = async (id) => {
    if (confirm(t('confirm_delete_api_key'))) {
      try {
        await axios.delete(`${API_URL}/workspaces/${workspaceId}/keys/${id}`)
        reload()
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
      reload()
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
      reload()
      notify(t('api_key_rotated'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rotate_api_key'), 'error')
    }
  }

  const savePermissions = async ({ roleId, permissions }) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/roles/${roleId}`, { permissions })
      reload()
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
              onClick={() => handleSectionChange('general')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <SettingsIcon width="1rem" height="1rem" /> {t('general')}
            </button>
            <button
              className={`tab ${activeSection === 'users' ? 'active' : ''}`}
              onClick={() => handleSectionChange('users')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <Group width="1rem" height="1rem" /> {t('users')}
            </button>
            <button
              className={`tab ${activeSection === 'roles' ? 'active' : ''}`}
              onClick={() => handleSectionChange('roles')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <ShieldCheck width="1rem" height="1rem" /> {t('security_roles')}
            </button>
            <button
              className={`tab ${activeSection === 'keys' ? 'active' : ''}`}
              onClick={() => handleSectionChange('keys')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <Key width="1rem" height="1rem" /> {t('api_keys')}
            </button>
            <button
              className={`tab ${activeSection === 'webhooks' ? 'active' : ''}`}
              onClick={() => handleSectionChange('webhooks')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <LinkIcon width="1rem" height="1rem" /> {t('webhooks')}
            </button>
            <button
              className={`tab ${activeSection === 'audit' ? 'active' : ''}`}
              onClick={() => handleSectionChange('audit')}
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
            >
              <Activity width="1rem" height="1rem" /> {t('audit_tab')}
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
                tables={tables}
                roles={roles}
                selectedRoleId={selectedRoleId || (roles.length > 0 ? roles[0].id : '')}
                onSave={savePermissions}
              />
            </div>
          ) : activeSection === 'audit' ? (
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem', gap: '1rem', flexWrap: 'wrap' }}>
                <p style={{ color: 'var(--text-secondary)' }}>{t('audit_desc')}</p>
                <button
                  className="btn btn-secondary btn-sm"
                  onClick={fetchAudit}
                  aria-label={t('refresh')}
                  title={t('refresh')}
                >
                  <Refresh width="1rem" height="1rem" />
                </button>
              </div>

              {/* Filters */}
              <div style={{ display: 'flex', gap: '0.75rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
                <div className="form-group" style={{ marginBottom: 0, minWidth: '12rem' }}>
                  <label className="form-label" htmlFor="audit-filter-action">{t('audit_filter_action')}</label>
                  <select
                    id="audit-filter-action"
                    className="form-select"
                    value={auditAction}
                    onChange={e => { setAuditAction(e.target.value); setAuditPage(0) }}
                  >
                    <option value="">{t('audit_all_actions')}</option>
                    {auditActions.map(action => (
                      <option key={action} value={action}>{action}</option>
                    ))}
                  </select>
                </div>
                <div className="form-group" style={{ marginBottom: 0, minWidth: '12rem' }}>
                  <label className="form-label" htmlFor="audit-filter-resource">{t('audit_filter_resource')}</label>
                  <select
                    id="audit-filter-resource"
                    className="form-select"
                    value={auditResourceType}
                    onChange={e => { setAuditResourceType(e.target.value); setAuditPage(0) }}
                  >
                    <option value="">{t('audit_all_resources')}</option>
                    {auditResourceTypes.map(type => (
                      <option key={type} value={type}>{type}</option>
                    ))}
                  </select>
                </div>
              </div>

              {auditError ? (
                <div className="empty-state">
                  <div className="empty-icon"><Activity width="2rem" height="2rem" /></div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('audit_load_error')}</h3>
                </div>
              ) : auditLoading && auditEntries.length === 0 ? (
                <div style={{ display: 'flex', justifyContent: 'center', padding: '2rem 0' }}>
                  <div className="loading-spinner" />
                </div>
              ) : auditEntries.length === 0 ? (
                <div className="empty-state">
                  <div className="empty-icon"><Activity width="2rem" height="2rem" /></div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('audit_no_entries')}</h3>
                </div>
              ) : (
                <>
                  <div className="table-container" style={{ overflowX: 'auto' }}>
                    <table className="table">
                      <thead>
                        <tr>
                          <th>{t('audit_date')}</th>
                          <th>{t('audit_actor')}</th>
                          <th>{t('audit_action')}</th>
                          <th>{t('audit_resource')}</th>
                          <th>{t('audit_auth_source')}</th>
                          <th>{t('audit_ip')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {auditEntries.map(entry => (
                          <tr key={entry.id}>
                            <td style={{ whiteSpace: 'nowrap' }}>{formatTimestamp(entry.created_at)}</td>
                            <td>{entry.actor_email || entry.actor_id || t('audit_unknown_actor')}</td>
                            <td><Badge variant="primary">{entry.action}</Badge></td>
                            <td>
                              {entry.resource_type || '—'}
                              {entry.resource_id ? (
                                <span style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>
                                  {' '}· {String(entry.resource_id).slice(0, 8)}
                                </span>
                              ) : null}
                            </td>
                            <td>{entry.auth_source || '—'}</td>
                            <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>{entry.ip_address || '—'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  <div className="pagination-bar">
                    <span className="pagination-info">
                      {t('audit_page', { page: auditPage + 1 })} · {auditMeta.count ?? auditEntries.length}
                    </span>
                    <div className="pagination-controls">
                      <button
                        className="btn btn-secondary btn-sm"
                        onClick={() => setAuditPage(p => Math.max(0, p - 1))}
                        disabled={auditPage <= 0 || auditLoading}
                      >
                        {t('previous')}
                      </button>
                      <button
                        className="btn btn-secondary btn-sm"
                        onClick={() => setAuditPage(p => p + 1)}
                        disabled={!auditMeta.has_more || auditLoading}
                      >
                        {t('next')}
                      </button>
                    </div>
                  </div>
                </>
              )}
            </div>
          ) : activeSection === 'webhooks' ? (
            <div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem', gap: '1rem' }}>
                <p style={{ color: 'var(--text-secondary)' }}>{t('webhooks_desc')}</p>
                {!webhooksForbidden && canManageWebhooks && (
                  <Button size="sm" onClick={() => setShowCreateWebhook(v => !v)}>
                    {t('new_webhook')}
                  </Button>
                )}
              </div>

              {webhooksForbidden ? (
                <div className="empty-state">
                  <div className="empty-icon"><LinkIcon width="2rem" height="2rem" /></div>
                  <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('no_webhooks_permission')}</h3>
                  <p style={{ color: 'var(--text-secondary)' }}>{t('no_webhooks_permission_hint')}</p>
                </div>
              ) : (
                <>
                  {showCreateWebhook && (
                    <div className="card" style={{ marginBottom: '1.5rem', padding: '1.5rem' }}>
                      <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '1rem' }}>{t('new_webhook')}</h3>

                      <div className="form-group">
                        <label className="form-label">{t('webhook_url')}</label>
                        <input
                          type="url"
                          className="form-input"
                          value={newWebhookUrl}
                          onChange={e => setNewWebhookUrl(e.target.value)}
                          placeholder="https://example.com/hooks/hornero"
                        />
                        {newWebhookUrl.trim() && !webhookUrlValid && (
                          <p className="form-hint" style={{ color: 'var(--danger)' }}>{t('webhook_url_invalid')}</p>
                        )}
                      </div>

                      <div className="form-group" style={{ marginTop: '1rem' }}>
                        <label className="form-label">{t('webhook_table')}</label>
                        <select
                          className="form-select"
                          value={newWebhookTable}
                          onChange={e => setNewWebhookTable(e.target.value)}
                        >
                          <option value="">{t('select_table_placeholder')}</option>
                          {tables.map(tbl => (
                            <option key={tbl.id} value={tbl.id}>{tbl.name}</option>
                          ))}
                        </select>
                      </div>

                      <div className="form-group" style={{ marginTop: '1rem' }}>
                        <label className="form-label">{t('webhook_event')}</label>
                        <select
                          className="form-select"
                          value={newWebhookEvent}
                          onChange={e => setNewWebhookEvent(e.target.value)}
                        >
                          {WEBHOOK_EVENT_TYPES.map(type => (
                            <option key={type} value={type}>{t(`webhook_event_${type}`)}</option>
                          ))}
                        </select>
                      </div>

                      <div className="form-group" style={{ marginTop: '1rem' }}>
                        <label className="form-label">{t('webhook_client_state')}</label>
                        <input
                          type="text"
                          className="form-input"
                          value={newWebhookClientState}
                          onChange={e => setNewWebhookClientState(e.target.value)}
                          placeholder={t('webhook_client_state_placeholder')}
                        />
                        <p className="form-hint" style={{ marginTop: '0.25rem' }}>{t('webhook_client_state_hint')}</p>
                      </div>

                      <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
                        <Button variant="secondary" onClick={() => setShowCreateWebhook(false)}>{t('cancel')}</Button>
                        <Button
                          onClick={handleCreateWebhook}
                          loading={savingWebhook}
                          disabled={!newWebhookUrl.trim() || !newWebhookTable || !webhookUrlValid}
                        >
                          {t('create')}
                        </Button>
                      </div>
                    </div>
                  )}

                  {webhooks.length === 0 ? (
                    <div className="empty-state">
                      <div className="empty-icon"><LinkIcon width="2rem" height="2rem" /></div>
                      <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('no_webhooks')}</h3>
                      <p style={{ color: 'var(--text-secondary)' }}>{t('create_webhook_hint')}</p>
                    </div>
                  ) : (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                      {webhooks.map(hook => (
                        <div key={hook.id} className="card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
                          <div style={{ minWidth: 0 }}>
                            <div style={{ fontWeight: 700 }}>{webhookTableName(hook.resource)}</div>
                            <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                              {hook.notification_url}
                            </div>
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                            <Badge variant="primary">
                              {t(`webhook_event_${hook.change_type}`, { defaultValue: hook.change_type })}
                            </Badge>
                            <button
                              onClick={() => deleteWebhook(hook.id)}
                              className="btn btn-ghost btn-sm"
                              style={{ color: 'var(--danger)' }}
                              title={t('delete_webhook')}
                              aria-label={t('delete_webhook')}
                              disabled={!canManageWebhooks}
                            >
                              <Trash width="1rem" height="1rem" />
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* ── Delivery history ── */}
                  <div style={{ marginTop: '2.5rem', paddingTop: '1.5rem', borderTop: '1px solid var(--border-light)' }}>
                    <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: '1rem', gap: '1rem', flexWrap: 'wrap' }}>
                      <div>
                        <h3 style={{ fontSize: '1.125rem', fontWeight: 700 }}>{t('webhook_delivery_history')}</h3>
                        <p style={{ fontSize: '0.8125rem', color: 'var(--text-muted)' }}>{t('webhook_delivery_history_desc')}</p>
                      </div>
                      <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                        <select
                          className="form-select"
                          style={{ height: 'auto', padding: '0.375rem 0.5rem' }}
                          value={webhookEventFilter}
                          onChange={e => setWebhookEventFilter(e.target.value)}
                          aria-label={t('webhook_delivery_status')}
                        >
                          <option value="">{t('webhook_filter_all')}</option>
                          <option value="failed">{t('webhook_filter_failed')}</option>
                          <option value="pending">{t('webhook_filter_pending')}</option>
                          <option value="completed">{t('webhook_filter_completed')}</option>
                        </select>
                        <button
                          className="btn btn-secondary btn-sm"
                          onClick={fetchWebhookEvents}
                          title={t('refresh')}
                          aria-label={t('refresh')}
                        >
                          <Refresh width="1rem" height="1rem" />
                        </button>
                      </div>
                    </div>

                    {webhookEventsError ? (
                      <div className="empty-state">
                        <div className="empty-icon"><LinkIcon width="2rem" height="2rem" /></div>
                        <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('webhook_events_load_error')}</h3>
                      </div>
                    ) : webhookEventsLoading && webhookEvents.length === 0 ? (
                      <div style={{ display: 'flex', justifyContent: 'center', padding: '1.5rem 0' }}>
                        <div className="loading-spinner" />
                      </div>
                    ) : webhookEvents.length === 0 ? (
                      <div className="empty-state">
                        <div className="empty-icon"><LinkIcon width="2rem" height="2rem" /></div>
                        <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{t('webhook_no_events')}</h3>
                      </div>
                    ) : (
                      <div className="table-container" style={{ overflowX: 'auto' }}>
                        <table className="table">
                          <thead>
                            <tr>
                              <th>{t('webhook_delivery_status')}</th>
                              <th>{t('webhook_attempts')}</th>
                              <th>{t('webhook_http_code')}</th>
                              <th>{t('webhook_last_error')}</th>
                              <th>{t('webhook_last_attempt')}</th>
                              <th style={{ width: '110px' }}></th>
                            </tr>
                          </thead>
                          <tbody>
                            {webhookEvents.map(event => {
                              const isFailed = String(event.status || '').toLowerCase() === 'failed'
                              return (
                                <tr key={event.id}>
                                  <td>
                                    <Badge variant={webhookEventVariant(event.status)}>
                                      {webhookEventStatus(event.status)}
                                    </Badge>
                                  </td>
                                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>
                                    {event.attempts ?? 0} / {event.max_attempts ?? '—'}
                                  </td>
                                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: '0.75rem' }}>
                                    {event.last_status_code ?? '—'}
                                  </td>
                                  <td style={{ fontSize: '0.8125rem', maxWidth: '18rem', wordBreak: 'break-word' }}>
                                    {event.last_error || '—'}
                                  </td>
                                  <td style={{ whiteSpace: 'nowrap', fontSize: '0.8125rem' }}>
                                    {formatTimestamp(event.last_attempt_at || event.created_at)}
                                  </td>
                                  <td>
                                    {isFailed && (
                                      <button
                                        className="btn btn-secondary btn-sm"
                                        onClick={() => handleRetryWebhookEvent(event.id)}
                                        disabled={retryingEventId === event.id}
                                        title={t('webhook_retry')}
                                        aria-label={t('webhook_retry')}
                                      >
                                        {retryingEventId === event.id ? t('webhook_retrying') : t('webhook_retry')}
                                      </button>
                                    )}
                                  </td>
                                </tr>
                              )
                            })}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                </>
              )}
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
                          aria-label={t('rotate_key')}
                        >
                          <svg width="1rem" height="1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                        </button>
                        <button
                          onClick={() => openKeySettings(key)}
                          className="btn btn-ghost btn-sm"
                          title={t('edit_key_settings')}
                          aria-label={t('edit_key_settings')}
                        >
                          <SettingsIcon width="1rem" height="1rem" />
                        </button>
                        <button
                          onClick={() => deleteAPIKey(key.id)}
                          className="btn btn-ghost btn-sm"
                          style={{ color: 'var(--danger)' }}
                          title={t('confirm_delete_api_key')}
                          aria-label={t('confirm_delete_api_key')}
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
