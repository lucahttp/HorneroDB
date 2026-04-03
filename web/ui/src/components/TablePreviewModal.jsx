import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { Xmark, Table2Columns } from 'iconoir-react'
import { API_URL } from '../constants/index.js'
import { getFieldConfig } from '../fieldTypeConfig.jsx'

export function TablePreviewModal({ table, workspaceId, onClose }) {
  const { t } = useTranslation()
  const [records, setRecords] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    axios
      .get(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, { params: { limit: 10 } })
      .then(r => setRecords(r.data.data || []))
      .catch(() => setError(t('preview_error')))
      .finally(() => setLoading(false))
  }, [workspaceId, table.slug, t])

  const columns = table.columns || []

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
              <div className="modal-title">{t('preview_title')} — {table.name}</div>
              <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                {t('preview_subtitle')} · @{table.slug}
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
              {t('preview_no_records')}
            </div>
          ) : columns.length === 0 ? (
            <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>
              {t('preview_no_columns')}
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
            {t('preview_showing', { count: records.length })}
          </div>
        )}
      </div>
    </div>
  )
}
