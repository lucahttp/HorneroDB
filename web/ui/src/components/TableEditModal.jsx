import { useState, useEffect } from 'react'
import axios from 'axios'
import { Xmark, Plus, Trash, Table2Columns } from 'iconoir-react'
import { Button } from './index.jsx'
import { notify } from './Toast.jsx'
import { API_URL } from '../constants/index.js'
import { FIELD_TYPE_OPTIONS, getFieldConfig } from '../fieldTypeConfig.jsx'

/**
 * Modal to edit a table: rename it and manage its columns.
 * Props:
 *   table — table object with { id, name, slug, columns }
 *   workspaceId
 *   onClose()
 *   onTableUpdated(updatedTable) — called after rename
 *   onColumnAdded(newCol) / onColumnDeleted(colId)
 */
export function TableEditModal({ table, workspaceId, onClose, onTableUpdated, onColumnAdded, onColumnDeleted }) {
  const [tab, setTab] = useState('columns') // 'general' | 'columns'
  const [name, setName] = useState(table.name)
  const [saving, setSaving] = useState(false)

  // New column form
  const [newColName, setNewColName] = useState('')
  const [newColType, setNewColType] = useState('text')
  const [newColTargetTable, setNewColTargetTable] = useState('')
  const [addingCol, setAddingCol] = useState(false)
  const [showAddForm, setShowAddForm] = useState(false)

  // local columns state (start from prop)
  const [columns, setColumns] = useState(table.columns || [])

  // Tables list for relation picker (fetched lazily)
  const [allTables, setAllTables] = useState([])
  useEffect(() => {
    if (newColType === 'relation' && allTables.length === 0) {
      axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
        .then(r => setAllTables(r.data.data || []))
        .catch(() => {})
    }
  }, [newColType, allTables.length, workspaceId])

  const handleRename = async () => {
    if (!name.trim() || name === table.name) return
    setSaving(true)
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/tables/${table.id}`, { name })
      onTableUpdated({ ...table, name })
      notify('Tabla renombrada', 'success')
    } catch {
      notify('Error al renombrar la tabla', 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleAddColumn = async () => {
    if (!newColName.trim()) return
    setAddingCol(true)
    try {
      const meta = newColType === 'relation' ? { target_table: newColTargetTable } : {}
      const res = await axios.post(
        `${API_URL}/workspaces/${workspaceId}/tables/${table.id}/columns`,
        { name: newColName.trim(), field_type: newColType, meta }
      )
      const col = res.data.data
      setColumns(prev => [...prev, col])
      onColumnAdded?.(col)
      setNewColName('')
      setNewColType('text')
      setNewColTargetTable('')
      setShowAddForm(false)
      notify('Columna creada', 'success')
    } catch {
      notify('Error al crear la columna', 'error')
    } finally {
      setAddingCol(false)
    }
  }

  const handleDeleteColumn = async (col) => {
    if (!confirm(`¿Eliminar la columna "${col.name}"? Esta acción no se puede deshacer.`)) return
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/tables/${table.id}/columns/${col.id}`)
      setColumns(prev => prev.filter(c => c.id !== col.id))
      onColumnDeleted?.(col.id)
      notify('Columna eliminada', 'success')
    } catch {
      notify('Error al eliminar la columna', 'error')
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '520px' }}>
        {/* Header */}
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{ width: '2rem', height: '2rem', borderRadius: '8px', background: 'var(--primary)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Table2Columns width="1.1rem" height="1.1rem" style={{ color: '#fff' }} />
            </div>
            <div>
              <div className="modal-title">{table.name}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>@{table.slug}</div>
            </div>
          </div>
          <button className="btn btn-ghost btn-sm" onClick={onClose}><Xmark width="1.25rem" /></button>
        </div>

        {/* Tabs */}
        <div className="tabs" style={{ marginBottom: 0, paddingLeft: '1.5rem' }}>
          <button className={`tab${tab === 'columns' ? ' active' : ''}`} onClick={() => setTab('columns')}>Columnas</button>
          <button className={`tab${tab === 'general' ? ' active' : ''}`} onClick={() => setTab('general')}>General</button>
        </div>

        {/* Body */}
        <div className="modal-body">
          {tab === 'general' && (
            <div>
              <div className="form-group">
                <label className="form-label">Nombre de la tabla</label>
                <input
                  className="form-input"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleRename()}
                />
              </div>
              <Button onClick={handleRename} loading={saving} disabled={!name.trim() || name === table.name}>
                Guardar
              </Button>
            </div>
          )}

          {tab === 'columns' && (
            <div>
              {/* Existing columns */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', marginBottom: '1rem' }}>
                {columns.length === 0 && (
                  <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', textAlign: 'center', padding: '1rem 0' }}>
                    No hay columnas. Agrega una abajo.
                  </p>
                )}
                {columns.map(col => {
                  const cfg = getFieldConfig(col.field_type)
                  return (
                    <div key={col.id} style={{
                      display: 'flex', alignItems: 'center', gap: '0.75rem',
                      padding: '0.625rem 0.875rem',
                      border: '1.5px solid var(--border-light)',
                      borderRadius: 'var(--radius-sm)',
                      background: 'var(--bg-surface)',
                    }}>
                      <span style={{ width: '1.4rem', height: '1.4rem', borderRadius: '5px', background: cfg.color, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '0.7rem', fontWeight: 700, flexShrink: 0 }}>
                        {typeof cfg.icon === 'string' ? cfg.icon : '⟲'}
                      </span>
                      <span style={{ fontWeight: 600, flex: 1, fontSize: '0.875rem' }}>{col.name}</span>
                      <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>{col.field_type}</span>
                      <button className="btn btn-ghost btn-sm" style={{ padding: '4px' }} onClick={() => handleDeleteColumn(col)}>
                        <Trash width="0.9rem" height="0.9rem" style={{ color: 'var(--danger)' }} />
                      </button>
                    </div>
                  )
                })}
              </div>

              {/* Add column form */}
              {showAddForm ? (
                <div style={{ padding: '1rem', border: '2px dashed var(--border-color)', borderRadius: 'var(--radius-md)', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label" style={{ fontSize: '0.7rem' }}>Nombre</label>
                    <input
                      className="form-input"
                      autoFocus
                      placeholder="nombre_columna"
                      value={newColName}
                      onChange={e => setNewColName(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter') handleAddColumn(); if (e.key === 'Escape') setShowAddForm(false) }}
                    />
                  </div>
                  <div className="form-group" style={{ marginBottom: 0 }}>
                    <label className="form-label" style={{ fontSize: '0.7rem' }}>Tipo</label>
                    <select
                      className="form-select"
                      value={newColType}
                      onChange={e => setNewColType(e.target.value)}
                    >
                      {FIELD_TYPE_OPTIONS.map(ft => (
                        <option key={ft.value} value={ft.value}>{ft.label}</option>
                      ))}
                    </select>
                  </div>
                  {newColType === 'relation' && (
                    <div className="form-group" style={{ marginBottom: 0 }}>
                      <label className="form-label" style={{ fontSize: '0.7rem' }}>Tabla destino</label>
                      <select className="form-select" value={newColTargetTable} onChange={e => setNewColTargetTable(e.target.value)}>
                        <option value="">Seleccionar tabla...</option>
                        {allTables.map(t => <option key={t.id} value={t.slug}>{t.name}</option>)}
                      </select>
                    </div>
                  )}
                  <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                    <Button size="sm" onClick={handleAddColumn} loading={addingCol} disabled={!newColName.trim()}>Crear</Button>
                    <Button variant="secondary" size="sm" onClick={() => setShowAddForm(false)}>Cancelar</Button>
                  </div>
                </div>
              ) : (
                <button
                  className="btn btn-secondary btn-sm"
                  style={{ width: '100%', justifyContent: 'center', gap: '0.5rem' }}
                  onClick={() => setShowAddForm(true)}
                >
                  <Plus width="1rem" height="1rem" />
                  Agregar columna
                </button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
