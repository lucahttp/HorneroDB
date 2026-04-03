import { useState, useEffect } from 'react'
import axios from 'axios'
import { Xmark, Table2Columns } from 'iconoir-react'
import { API_URL } from '../constants/index.js'
import { getFieldConfig } from '../fieldTypeConfig.jsx'

/**
 * Preview modal — shows the first 10 rows of a table.
 * Props:
 *   table — { id, name, slug, columns }
 *   workspaceId
 *   onClose()
 */
export function TablePreviewModal({ table, workspaceId, onClose }) {
  const [records, setRecords] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    axios
      .get(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, { params: { limit: 10 } })
      .then(r => setRecords(r.data.data || []))
      .catch(() => setError('No se pudieron cargar los datos'))
      .finally(() => setLoading(false))
  }, [workspaceId, table.slug])

  const columns = table.columns || []

  // Format a cell value for display
  const renderCell = (record, col) => {
    const val = record[col.slug]
    if (val == null || val === '') return <span style={{ color: 'var(--text-muted)', fontStyle: 'italic' }}>—</span>
    const cfg = getFieldConfig(col.field_type)
    if (cfg.inputType === 'checkbox') return val === true || val === 'true' ? '✓' : '✗'
    if (col.field_type === 'json') return <code style={{ fontSize: '0.7rem', color: 'var(--text-muted)' }}>{JSON.stringify(val).slice(0, 30)}</code>
    return String(val).slice(0, 40)
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal"
        onClick={e => e.stopPropagation()}
        style={{ maxWidth: '90vw', width: '860px', maxHeight: '85vh' }}
      >
        {/* Header */}
        <div className="modal-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{ width: '2rem', height: '2rem', borderRadius: '8px', background: 'var(--accent)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Table2Columns width="1.1rem" height="1.1rem" style={{ color: '#fff' }} />
            </div>
            <div>
              <div className="modal-title">Preview — {table.name}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                Primeras 10 filas · @{table.slug}
              </div>
            </div>
          </div>
          <button className="btn btn-ghost btn-sm" onClick={onClose}><Xmark width="1.25rem" /></button>
        </div>

        {/* Body */}
        <div className="modal-body" style={{ padding: 0, overflowX: 'auto' }}>
          {loading ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '3rem' }}>
              <div className="loading-spinner" />
            </div>
          ) : error ? (
            <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--danger)' }}>{error}</div>
          ) : records.length === 0 ? (
            <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>
              Esta tabla no tiene registros aún.
            </div>
          ) : columns.length === 0 ? (
            <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>
              Esta tabla no tiene columnas definidas.
            </div>
          ) : (
            <table className="table" style={{ fontSize: '0.8125rem' }}>
              <thead>
                <tr>
                  {columns.map(col => {
                    const cfg = getFieldConfig(col.field_type)
                    return (
                      <th key={col.id}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                          <span style={{ width: '1.1rem', height: '1.1rem', borderRadius: '4px', background: cfg.color, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '0.6rem', fontWeight: 700, flexShrink: 0 }}>
                            {typeof cfg.icon === 'string' ? cfg.icon : '⟲'}
                          </span>
                          {col.name}
                        </div>
                      </th>
                    )
                  })}
                </tr>
              </thead>
              <tbody>
                {records.map((rec, i) => (
                  <tr key={rec.id || i}>
                    {columns.map(col => (
                      <td key={col.id} style={{ maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {renderCell(rec, col)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {!loading && !error && records.length > 0 && (
          <div className="modal-footer" style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
            Mostrando {records.length} de los primeros registros
          </div>
        )}
      </div>
    </div>
  )
}
