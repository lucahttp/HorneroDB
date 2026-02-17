import { useState } from 'react'
import { Modal, Button } from './index.jsx'

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
      <div className="min-w-[600px]">
        <div className="mb-4">
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
            <div className="overflow-x-auto mt-4">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left font-medium text-gray-600 dark:text-gray-400 px-3 py-2">Tabla</th>
                    <th className="px-3 py-2">Crear</th>
                    <th className="px-3 py-2">Leer</th>
                    <th className="px-3 py-2">Actualizar</th>
                    <th className="px-3 py-2">Borrar</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td className="font-semibold px-3 py-2">🌐 Todas las tablas</td>
                    {['create', 'read', 'update', 'delete'].map(action => (
                      <td key={action} className="px-3 py-2">
                        <select
                          className="w-full px-2 py-1 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-xs"
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
                      <td className="px-3 py-2">{table.name}</td>
                      {['create', 'read', 'update', 'delete'].map(action => (
                        <td key={action} className="px-3 py-2">
                          <select
                            className="w-full px-2 py-1 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-xs"
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

            <div className="mt-4 flex gap-6 text-xs text-gray-500">
              <span>🟢 <b>Todas</b> - Acceso completo</span>
              <span>🟡 <b>Propias</b> - Solo registros propios</span>
              <span>🔴 <b>Ninguna</b> - Sin acceso</span>
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
      className="px-2 py-1 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-700 rounded text-xs"
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
    <span className={`badge badge-${config.color}`}>
      {config.label}
    </span>
  )
}
