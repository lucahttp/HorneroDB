import { useState } from 'react'
import { Button, Badge } from './index.jsx'
import { Lock, Globe } from 'iconoir-react'
import { useTranslation } from 'react-i18next'

const ACCESS_LEVELS = [
  { value: 'all', color: 'success' },
  { value: 'own', color: 'warning' },
  { value: 'none', color: 'error' },
]

/**
 * Inline permission matrix — renders directly in the page (no modal).
 * Shows CRUD permissions per table for a selected role.
 */
export function PermissionMatrix({
  workspaceId,
  tables = [],
  roles = [],
  onSave,
}) {
  const { t } = useTranslation()
  const [selectedRole, setSelectedRole] = useState(roles[0]?.id || '')
  const [permissions, setPermissions] = useState({})
  const [saving, setSaving] = useState(false)

  const currentRole = roles.find(r => r.id === selectedRole)
  const rolePermissions = currentRole?.permissions || {}

  const getPermission = (tableSlug, action) => {
    // Local edits override role defaults
    if (permissions[tableSlug]?.[action] !== undefined) {
      return permissions[tableSlug][action]
    }
    const tablePerm = rolePermissions[tableSlug] || rolePermissions['*']
    return tablePerm?.[action] || 'none'
  }

  const handlePermissionChange = (tableSlug, action, value) => {
    setPermissions(prev => ({
      ...prev,
      [tableSlug]: {
        ...(prev[tableSlug] || {}),
        [action]: value
      }
    }))
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const finalPermissions = {}

      tables.forEach(table => {
        const tablePerm = permissions[table.slug]
        if (tablePerm) {
          finalPermissions[table.slug] = tablePerm
        }
      })

      if (permissions['*']) {
        finalPermissions['*'] = permissions['*']
      }

      await onSave({
        roleId: selectedRole,
        permissions: finalPermissions
      })
      setPermissions({})
    } catch (err) {
      console.error(err)
    } finally {
      setSaving(false)
    }
  }

  if (roles.length === 0) {
    return (
      <div className="empty-state" style={{ padding: '2rem' }}>
        <p style={{ color: 'var(--text-muted)' }}>{t('create_role_hint')}</p>
      </div>
    )
  }

  const hasChanges = Object.keys(permissions).length > 0

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <Lock width="1.25rem" height="1.25rem" /> {t('table_permissions')}
      </h2>

      {/* Role selector */}
      <div className="form-group" style={{ maxWidth: '320px', marginBottom: '1rem' }}>
        <label className="form-label">Rol</label>
        <select
          className="form-select"
          value={selectedRole}
          onChange={e => { setSelectedRole(e.target.value); setPermissions({}) }}
        >
          {roles.map(role => (
            <option key={role.id} value={role.id}>
              {role.name} {role.is_default && `(${t('default')})`}
            </option>
          ))}
        </select>
      </div>

      {currentRole && (
        <>
          <div className="table-container">
            <table className="table">
              <thead>
                <tr>
                  <th>{t('table')}</th>
                  <th>{t('create')}</th>
                  <th>{t('read')}</th>
                  <th>{t('update')}</th>
                  <th>{t('delete')}</th>
                </tr>
              </thead>
              <tbody>
                {/* Global wildcard row */}
                <tr>
                  <td style={{ fontWeight: 700, display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <Globe width="1rem" height="1rem" /> {t('all_tables')}
                  </td>
                  {['create', 'read', 'update', 'delete'].map(action => (
                    <td key={action}>
                      <select
                        className="form-select"
                        style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem' }}
                        value={getPermission('*', action)}
                        onChange={e => handlePermissionChange('*', action, e.target.value)}
                      >
                        {ACCESS_LEVELS.map(level => (
                          <option key={level.value} value={level.value}>
                            {t(`access_level_${level.value}`)}
                          </option>
                        ))}
                      </select>
                    </td>
                  ))}
                </tr>

                {tables.map(table => (
                  <tr key={table.id}>
                    <td>{table.name}</td>
                    {['create', 'read', 'update', 'delete'].map(action => (
                      <td key={action}>
                        <select
                          className="form-select"
                          style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem' }}
                          value={getPermission(table.slug, action)}
                          onChange={e => handlePermissionChange(table.slug, action, e.target.value)}
                        >
                          <option value="inherit">{t('inherit')}</option>
                          {ACCESS_LEVELS.map(level => (
                            <option key={level.value} value={level.value}>
                              {t(`access_level_${level.value}`)}
                            </option>
                          ))}
                        </select>
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Legend + save */}
          <div style={{
            marginTop: '1rem',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}>
            <div style={{ display: 'flex', gap: '1.5rem', fontSize: '0.8rem' }}>
              <Badge variant="success">{t('legend_all')}</Badge>
              <Badge variant="warning">{t('legend_own')}</Badge>
              <Badge variant="error">{t('legend_none')}</Badge>
            </div>
            {hasChanges && (
              <Button size="sm" onClick={handleSave} loading={saving}>
                {t('save_changes')}
              </Button>
            )}
          </div>
        </>
      )}
    </div>
  )
}

export function PermissionSelector({
  value,
  onChange,
  options = ACCESS_LEVELS
}) {
  const { t } = useTranslation()
  return (
    <select
      className="form-select"
      style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem' }}
      value={value || 'none'}
      onChange={e => onChange(e.target.value)}
    >
      {options.map(option => (
        <option key={option.value} value={option.value}>
          {t(`access_level_${option.value}`)}
        </option>
      ))}
    </select>
  )
}

export function PermissionBadge({ level }) {
  const { t } = useTranslation()
  const config = ACCESS_LEVELS.find(l => l.value === level)
  if (!config) return null

  return (
    <Badge variant={config.color}>
      {t(`access_level_${config.value}`)}
    </Badge>
  )
}
