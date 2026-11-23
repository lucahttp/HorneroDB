import { useState, useEffect } from 'react'
import { Button, Badge } from './index.jsx'
import { Lock, Globe, ShieldCheck } from 'iconoir-react'
import { useTranslation } from 'react-i18next'
import { SYSTEM_ACCESS_KEY, SYSTEM_AREAS, SYSTEM_ACCESS_LEVELS } from '../utils/permissions.js'

const ACCESS_LEVELS = [
  { value: 'all', color: 'success' },
  { value: 'own', color: 'warning' },
  { value: 'none', color: 'error' },
]

const OPERATIONS = ['read', 'create', 'update', 'delete']

/** i18n key of every system area shown in the `__system__` block. */
const SYSTEM_AREA_LABEL_KEYS = {
  webhooks: 'sys_area_webhooks',
  roles: 'sys_area_roles',
  api_keys: 'sys_area_api_keys',
  tables: 'sys_area_tables',
  settings: 'sys_area_settings',
  mcp: 'sys_area_mcp',
}

/** Parse the permission document of a role (the API may return an object or a JSON string). */
function parseRolePermissions(raw) {
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

export function PermissionMatrix({
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
  const rolePermissions = parseRolePermissions(currentRole?.permissions)
  const isAdminRole = currentRole?.name === 'admin'

  const getPermission = (tableSlug, action) => {
    if (permissions[tableSlug]?.[action] !== undefined) {
      return permissions[tableSlug][action]
    }
    const tablePerm = rolePermissions[tableSlug] || rolePermissions['*']
    return tablePerm?.[action] || 'none'
  }

  /**
   * Column restriction of an operation.
   *
   * Mirrors `resolveColumns` of the backend: the table rule wins over the
   * `*` wildcard rule, and the fallback only happens when the table rule does
   * not define a list for that operation.
   *
   * @returns {string[]|undefined} `undefined` means "no restriction"
   * (every column is allowed), an empty array means "no column is allowed".
   */
  const getColumns = (tableSlug, operation) => {
    const local = permissions[tableSlug]?.columns
    if (local && Object.prototype.hasOwnProperty.call(local, operation)) {
      const value = local[operation]
      return value === null ? undefined : value
    }

    const fromTable = rolePermissions[tableSlug]?.columns?.[operation]
    if (fromTable !== undefined && fromTable !== null) {
      return Array.isArray(fromTable) ? fromTable : undefined
    }
    const fromWildcard = rolePermissions['*']?.columns?.[operation]
    if (fromWildcard !== undefined && fromWildcard !== null) {
      return Array.isArray(fromWildcard) ? fromWildcard : undefined
    }
    return undefined
  }

  /** Level of a system area (`none` / `view` / `manage` / `all`). */
  const getSystemPermission = (area) => {
    const local = permissions[SYSTEM_ACCESS_KEY]?.[area]
    if (local !== undefined) return local
    const value = rolePermissions[SYSTEM_ACCESS_KEY]?.[area]
    return SYSTEM_ACCESS_LEVELS.includes(value) ? value : 'none'
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

  const handleSystemPermissionChange = (area, value) => {
    setPermissions(prev => ({
      ...prev,
      [SYSTEM_ACCESS_KEY]: {
        ...(prev[SYSTEM_ACCESS_KEY] || {}),
        [area]: value
      }
    }))
  }

  /**
   * Store a column restriction.
   * @param {string[]|null} value `null` = all columns (no restriction),
   *   `[]` = no column, otherwise the explicit list of allowed columns.
   */
  const setColumnsRestriction = (tableSlug, operation, value) => {
    setPermissions(prev => ({
      ...prev,
      [tableSlug]: {
        ...(prev[tableSlug] || {}),
        columns: {
          ...(prev[tableSlug]?.columns || {}),
          [operation]: value
        }
      }
    }))
  }

  const toggleColumn = (table, operation, slug, checked) => {
    const current = getColumns(table.slug, operation)
    // "No restriction" behaves like "every column selected".
    const base = current === undefined ? table.columns.map(c => c.slug) : [...current]
    let next
    if (checked) {
      if (!base.includes(slug)) next = [...base, slug]
      else next = base
    } else {
      next = base.filter(c => c !== slug)
    }
    // Selecting every column is the same as not restricting them at all.
    const value = next.length === table.columns.length ? null : next
    setColumnsRestriction(table.slug, operation, value)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const finalPermissions = {}

      /**
       * Builds the COMPLETE rule of a table.
       *
       * The backend treats a rule that exists but does not mention an
       * operation as "deny" (`resolveOperationLevel`), so sending a partial
       * rule would silently remove the access inherited from the `*` wildcard.
       * Every operation is therefore sent explicitly: the value the user
       * changed, the stored value, or `inherit` (which falls back to the
       * wildcard rule).
       */
      const buildRule = (slug, local, fallbackLevel) => {
        const existing = rolePermissions[slug] || {}
        const rule = {}

        OPERATIONS.forEach(operation => {
          if (local[operation] !== undefined) {
            rule[operation] = local[operation]
          } else if (existing[operation] !== undefined) {
            rule[operation] = existing[operation]
          } else {
            rule[operation] = fallbackLevel
          }
        })

        const localColumns = local.columns
        if (localColumns) {
          const columns = {}
          Object.entries(localColumns).forEach(([operation, value]) => {
            columns[operation] = value
          })
          if (Object.keys(columns).length > 0) {
            rule.columns = columns
          }
        }

        return rule
      }

      tables.forEach(table => {
        const tablePerm = permissions[table.slug]
        if (tablePerm) {
          // A table without its own rule inherits everything from `*`.
          finalPermissions[table.slug] = buildRule(table.slug, tablePerm, 'inherit')
        }
      })

      if (permissions['*']) {
        // The wildcard rule cannot inherit from anything else.
        finalPermissions['*'] = buildRule('*', permissions['*'], 'none')
      }

      if (permissions[SYSTEM_ACCESS_KEY]) {
        finalPermissions[SYSTEM_ACCESS_KEY] = permissions[SYSTEM_ACCESS_KEY]
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

  /** Label of the "columns" button, reflecting the three possible states. */
  const columnsSummary = (table, operation) => {
    const selectedCols = getColumns(table.slug, operation)
    if (selectedCols === undefined) return t('columns_all_unrestricted')
    if (selectedCols.length === 0) return t('columns_none')
    return `${selectedCols.length}/${table.columns.length}`
  }

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <Lock width="1.25rem" height="1.25rem" /> {t('table_permissions')}
      </h2>

      {currentRole && (
        <>
          <div className="table-container" style={{ overflow: 'scroll' }}>
            <table className="table table-permissions" style={{ minWidth: '800px' }}>
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
                    <Globe width="1rem" height="1rem" />
                    <span className="hidden sm:inline">{t('all_tables')}</span>
                    <span className="sm:hidden">{t('all')}</span>
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
                      const selectedCols = getColumns(table.slug, action) ?? []
                      const hasColumns = table.columns && table.columns.length > 0
                      const currentLevel = getPermission(table.slug, action)

                      return (
                        <td key={action} style={{ position: 'relative', verticalAlign: 'top', paddingTop: '0.5rem', zIndex: isColsOpen ? 50 : 'auto' }}>
                          <select
                            className="form-select"
                            style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem', width: '100%' }}
                            value={currentLevel}
                            onChange={e => handlePermissionChange(table.slug, action, e.target.value)}
                          >
                            <option value="inherit">{t('inherit_from_wildcard')}</option>
                            {ACCESS_LEVELS.map(level => (
                              <option key={level.value} value={level.value}>
                                {t(`access_level_${level.value}`)}
                              </option>
                            ))}
                          </select>

                          {currentLevel === 'inherit' && (
                            <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', marginTop: '0.25rem' }}>
                              {t('inherit_hint')}
                            </div>
                          )}

                          {hasColumns && currentLevel !== 'none' && (
                            <div style={{ marginTop: '0.375rem' }}>
                              <button
                                className="btn btn-sm btn-ghost"
                                style={{ padding: '0.125rem 0.375rem', fontSize: '0.7rem', width: '100%', justifyContent: 'space-between' }}
                                onClick={() => toggleColumns(table.slug, action)}
                              >
                                <span>{t('columns')}</span>
                                <span style={{ opacity: 0.6, fontSize: '0.65rem' }}>
                                  {columnsSummary(table, action)}
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
                                    {t('allowed_columns')}
                                  </div>
                                  <div style={{ display: 'flex', gap: '0.375rem', flexWrap: 'wrap' }}>
                                    <button
                                      type="button"
                                      className="btn btn-sm btn-secondary"
                                      style={{ fontSize: '0.7rem', padding: '0.2rem 0.5rem', flex: 1 }}
                                      onClick={() => setColumnsRestriction(table.slug, action, null)}
                                    >
                                      {t('columns_select_all')}
                                    </button>
                                    <button
                                      type="button"
                                      className="btn btn-sm btn-secondary"
                                      style={{ fontSize: '0.7rem', padding: '0.2rem 0.5rem', flex: 1 }}
                                      onClick={() => setColumnsRestriction(table.slug, action, [])}
                                    >
                                      {t('columns_select_none')}
                                    </button>
                                  </div>
                                  <div style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem', maxHeight: '150px', overflowY: 'auto' }}>
                                    {table.columns.map(col => {
                                      const isChecked = getColumns(table.slug, action) === undefined
                                        ? true
                                        : selectedCols.includes(col.slug)
                                      return (
                                        <label key={col.slug} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.8rem', cursor: 'pointer', margin: 0 }}>
                                          <input
                                            type="checkbox"
                                            style={{ margin: 0 }}
                                            checked={isChecked}
                                            onChange={(e) => toggleColumn(table, action, col.slug, e.target.checked)}
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
                                    {t('done')}
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

          {/* ── System (workspace wide) permissions ───────────────────── */}
          <div style={{ marginTop: '2rem' }}>
            <h3 style={{ fontSize: '1rem', fontWeight: 700, marginBottom: '0.25rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <ShieldCheck width="1.1rem" height="1.1rem" /> {t('system_permissions')}
            </h3>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.8125rem', marginBottom: '0.75rem' }}>
              {t('system_permissions_desc')}
            </p>

            {isAdminRole && (
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.75rem' }}>
                {t('system_admin_note')}
              </div>
            )}

            <div className="table-container" style={{ overflow: 'scroll' }}>
              <table className="table table-permissions" style={{ minWidth: '420px' }}>
                <thead>
                  <tr>
                    <th style={{ minWidth: '160px' }}>{t('system_area')}</th>
                    <th style={{ minWidth: '140px' }}>{t('access_level')}</th>
                  </tr>
                </thead>
                <tbody>
                  {SYSTEM_AREAS.map(area => (
                    <tr key={area}>
                      <td style={{ fontWeight: 500, verticalAlign: 'middle' }}>
                        {t(SYSTEM_AREA_LABEL_KEYS[area])}
                      </td>
                      <td style={{ verticalAlign: 'middle' }}>
                        <select
                          className="form-select"
                          style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem', width: '100%' }}
                          value={getSystemPermission(area)}
                          onChange={e => handleSystemPermissionChange(area, e.target.value)}
                        >
                          {SYSTEM_ACCESS_LEVELS.map(level => (
                            <option key={level} value={level}>
                              {t(`system_access_${level}`)}
                            </option>
                          ))}
                        </select>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
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
