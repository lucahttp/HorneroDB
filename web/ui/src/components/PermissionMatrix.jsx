import { useState } from 'react'
import { Modal, Button, Badge } from './index.jsx'

const ACCESS_LEVELS = [
  { value: 'all', label: 'Todas', color: 'success' },
  { value: 'own', label: 'Propias', color: 'warning' },
  { value: 'none', label: 'Ninguna', color: 'error' },
]

export function PermissionMatrix({
  workspaceId,
  tables = [],
  roles = [],
  onSave,
  onClose
}) {
  const [selectedRole, setSelectedRole] = useState(roles[0]?.id || '')
  const [permissions, setPermissions] = useState({})
  const [saving, setSaving] = useState(false)

  const currentRole = roles.find(r => r.id === selectedRole)
  const rolePermissions = currentRole?.permissions || {}

  const getPermission = (tableSlug, action) => {
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
      onClose()
    } catch (err) {
      console.error(err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      isOpen={true}
      onClose={onClose}
      title="Permisos de Seguridad"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancelar
          </Button>
          <Button onClick={handleSave} loading={saving}>
            Guardar Cambios
          </Button>
        </>
      }
    >
      <div style={{ minWidth: '600px' }}>
        <div className="form-group">
          <label className="form-label">Selecciona un rol</label>
          <select
            className="form-select"
            value={selectedRole}
            onChange={e => setSelectedRole(e.target.value)}
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
            <div className="table-container" style={{ marginTop: '1rem' }}>
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

            <div style={{ marginTop: '1rem', display: 'flex', gap: '1.5rem', fontSize: '0.8rem' }}>
              <Badge variant="success">Todas — Acceso completo</Badge>
              <Badge variant="warning">Propias — Solo registros propios</Badge>
              <Badge variant="error">Ninguna — Sin acceso</Badge>
            </div>
          </>
        )}
      </div>
    </Modal>
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
