import { useState } from 'react'
import { Button, Badge } from './index.jsx'

const ACCESS_LEVELS = [
  { value: 'all', label: 'Todas', color: 'success' },
  { value: 'own', label: 'Propias', color: 'warning' },
  { value: 'none', label: 'Ninguna', color: 'error' },
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
        <p style={{ color: 'var(--text-muted)' }}>Creá un rol para configurar permisos</p>
      </div>
    )
  }

  const hasChanges = Object.keys(permissions).length > 0

  return (
    <div style={{ marginTop: '2rem' }}>
      <h2 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '1rem' }}>
        🔒 Permisos por Tabla
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
              {role.name} {role.is_default && '(Default)'}
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
                  <th>Tabla</th>
                  <th>Crear</th>
                  <th>Leer</th>
                  <th>Actualizar</th>
                  <th>Borrar</th>
                </tr>
              </thead>
              <tbody>
                {/* Global wildcard row */}
                <tr>
                  <td style={{ fontWeight: 700 }}>🌐 Todas las tablas</td>
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
                            {level.label}
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
                          <option value="inherit">Hereda</option>
                          {ACCESS_LEVELS.map(level => (
                            <option key={level.value} value={level.value}>
                              {level.label}
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
              <Badge variant="success">Todas — Acceso completo</Badge>
              <Badge variant="warning">Propias — Solo registros propios</Badge>
              <Badge variant="error">Ninguna — Sin acceso</Badge>
            </div>
            {hasChanges && (
              <Button size="sm" onClick={handleSave} loading={saving}>
                Guardar Cambios
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
  return (
    <select
      className="form-select"
      style={{ fontSize: '0.8rem', padding: '0.375rem 0.5rem' }}
      value={value || 'none'}
      onChange={e => onChange(e.target.value)}
    >
      {options.map(option => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  )
}

export function PermissionBadge({ level }) {
  const config = ACCESS_LEVELS.find(l => l.value === level)
  if (!config) return null

  return (
    <Badge variant={config.color}>
      {config.label}
    </Badge>
  )
}
