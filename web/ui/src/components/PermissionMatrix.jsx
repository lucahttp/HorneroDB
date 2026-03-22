import { useState, useEffect } from 'react'
import { Button, Badge } from './index.jsx'
import { Lock, Globe } from 'iconoir-react'
import { useTranslation } from 'react-i18next'

const ACCESS_LEVELS = [
  { value: 'all', color: 'success' },
  { value: 'own', color: 'warning' },
  { value: 'none', color: 'error' },
]

const OPERATIONS = ['read', 'create', 'update', 'delete']

export function PermissionMatrix({
  workspaceId,
  tables = [],
  roles = [],
  selectedRoleId,
  onSave,
}) {
  const { t } = useTranslation()
  const [permissions, setPermissions] = useState({})
  const [saving, setSaving] = useState(false)
  const [openColumns, setOpenColumns] = useState(null)

  useEffect(() => {
    setPermissions({})
  }, [selectedRoleId])

  const toggleColumns = (tableSlug, operation) => {
    if (openColumns?.tableSlug === tableSlug && openColumns?.operation === operation) {
      setOpenColumns(null)
    } else {
      setOpenColumns({ tableSlug, operation })
    }
  }

  const currentRole = roles.find(r => r.id === selectedRoleId)
  const rolePermissions = currentRole?.permissions || {}

  const getPermission = (tableSlug, action) => {
    if (permissions[tableSlug]?.[action] !== undefined) {
      return permissions[tableSlug][action]
    }
    const tablePerm = rolePermissions[tableSlug] || rolePermissions['*']
    return tablePerm?.[action] || 'none'
  }

  const getColumns = (tableSlug, operation) => {
    const perm = permissions[tableSlug]?.columns || {}
    if (perm[operation] !== undefined) {
      return perm[operation]
    }
    const tablePerm = rolePermissions[tableSlug] || rolePermissions['*']
    return tablePerm?.columns?.[operation] || []
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
        roleId: selectedRoleId,
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

      {currentRole && (
        <>
          <div className="table-container" style={{ overflow: 'scroll' }}>
            <table className="table" style={{ minWidth: '800px' }}>
              <thead>
                <tr>
                  <th style={{ minWidth: '120px' }}>{t('table')}</th>
                  {OPERATIONS.map(op => (
                    <th key={op} style={{ minWidth: '100px' }}>{t(op)}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td style={{ fontWeight: 700, display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <Globe width="1rem" height="1rem" /> {t('all_tables')}
                  </td>
                  {OPERATIONS.map(action => (
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
                      {action === 'read' && (
                        <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
                          {t('wildcard_hint')}
                        </div>
                      )}
                    </td>
                  ))}
                </tr>

                {tables.map(table => (
                  <tr key={table.id}>
                    <td style={{ fontWeight: 500, verticalAlign: 'top', paddingTop: '0.5rem' }}>{table.name}</td>
                    {OPERATIONS.map(action => {
                      const isColsOpen = openColumns?.tableSlug === table.slug && openColumns?.operation === action
                      const selectedCols = getColumns(table.slug, action) || []
                      const hasColumns = table.columns && table.columns.length > 0

                      return (
                        <td key={action} style={{ position: 'relative', verticalAlign: 'top', paddingTop: '0.5rem', zIndex: isColsOpen ? 50 : 'auto' }}>
                          <select
                            className="form-select"
                            style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem', width: '100%' }}
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

                          {hasColumns && getPermission(table.slug, action) !== 'inherit' && getPermission(table.slug, action) !== 'none' && (
                            <div style={{ marginTop: '0.375rem' }}>
                              <button
                                className="btn btn-sm btn-ghost"
                                style={{ padding: '0.125rem 0.375rem', fontSize: '0.7rem', width: '100%', justifyContent: 'space-between' }}
                                onClick={() => toggleColumns(table.slug, action)}
                              >
                                <span>Columns</span>
                                <span style={{ opacity: 0.6, fontSize: '0.65rem' }}>
                                  {selectedCols.length === 0 ? 'All' : `${selectedCols.length}/${table.columns.length}`}
                                </span>
                              </button>

                              {isColsOpen && (
                                <div style={{
                                  position: 'absolute', zIndex: 50, top: '100%', left: 0, right: 0,
                                  background: 'var(--bg-elevated)', border: '1px solid var(--border)',
                                  borderRadius: '6px', padding: '0.75rem', marginTop: '0.25rem',
                                  boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
                                  display: 'flex', flexDirection: 'column', gap: '0.5rem',
                                  minWidth: '180px'
                                }}>
                                  <div style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-primary)', borderBottom: '1px solid var(--border)', paddingBottom: '0.375rem' }}>
                                    Allowed Columns
                                  </div>
                                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem', maxHeight: '150px', overflowY: 'auto' }}>
                                    {table.columns.map(col => {
                                      const isChecked = selectedCols.includes(col.slug)
                                      return (
                                        <label key={col.slug} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.8rem', cursor: 'pointer', margin: 0 }}>
                                          <input
                                            type="checkbox"
                                            style={{ margin: 0 }}
                                            checked={isChecked}
                                            onChange={(e) => {
                                              let newCols = [...selectedCols]
                                              if (e.target.checked) newCols.push(col.slug)
                                              else newCols = newCols.filter(c => c !== col.slug)

                                              setPermissions(prev => ({
                                                ...prev,
                                                [table.slug]: {
                                                  ...prev[table.slug],
                                                  columns: {
                                                    ...(prev[table.slug]?.columns || {}),
                                                    [action]: newCols
                                                  }
                                                }
                                              }))
                                            }}
                                          />
                                          {col.name}
                                        </label>
                                      )
                                    })}
                                  </div>
                                  <button
                                    className="btn btn-sm btn-ghost"
                                    style={{ marginTop: '0.25rem', fontSize: '0.75rem', padding: '0.25rem', width: '100%', justifyContent: 'center' }}
                                    onClick={() => toggleColumns(table.slug, action)}
                                  >
                                    Done
                                  </button>
                                </div>
                              )}
                            </div>
                          )}
                        </td>
                      )
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

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
