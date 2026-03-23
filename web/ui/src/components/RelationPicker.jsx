import { useState, useEffect } from 'react'
import axios from 'axios'
import { Xmark, NavArrowLeft, NavArrowRight, Search } from 'iconoir-react'
import { Button } from './index.jsx'

const API_URL = import.meta.env.SERVER_PUBLIC_URL + '/api/v1' || 'http://localhost:8080/api/v1';

export function RelationPicker({
  workspaceId,
  targetTableSlug,
  displayColumn,
  onSelect,
  onClose,
  initialValue
}) {
  const [records, setRecords] = useState([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [error, setError] = useState(null)

  const loadRecords = async () => {
    if (!targetTableSlug) return
    setLoading(true)
    setError(null)
    try {
      const resp = await axios.get(`${API_URL}/workspaces/${workspaceId}/data/${targetTableSlug}`, {
        params: {
          search: search || undefined,
          limit: 50
        }
      })
      setRecords(resp.data.data || [])
    } catch (err) {
      console.error(err)
      setError('Error al cargar registros')
    }
    setLoading(false)
  }

  useEffect(() => {
    if (targetTableSlug) {
      loadRecords()
    } else {
      setLoading(false)
      setError('Configuración de relación incompleta (falta tabla destino)')
    }
  }, [targetTableSlug, search])

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '500px' }}>
        <div className="modal-header" style={{ background: 'var(--bg-elevated)', borderBottom: '1.5px solid var(--border-light)' }}>
          <h3 className="modal-title">Seleccionar Registro</h3>
          <button className="btn btn-ghost btn-sm" onClick={onClose}>
            <Xmark width="1.25rem" height="1.25rem" />
          </button>
        </div>

        <div className="modal-body" style={{ padding: '1rem' }}>
          <div className="form-group" style={{ marginBottom: '1rem', position: 'relative' }}>
            <Search
              width="1rem"
              height="1rem"
              style={{ position: 'absolute', left: '0.75rem', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }}
            />
            <input
              type="text"
              className="form-input"
              style={{ paddingLeft: '2.5rem' }}
              placeholder={`Buscar en ${targetTableSlug}...`}
              value={search}
              onChange={e => setSearch(e.target.value)}
              autoFocus
            />
          </div>

          <div style={{ maxHeight: '300px', overflowY: 'auto', border: '1.5px solid var(--border-light)', borderRadius: 'var(--radius-md)' }}>
            {loading ? (
              <div style={{ padding: '2rem', textAlign: 'center' }}>
                <div className="loading-spinner" style={{ margin: '0 auto', width: '2rem', height: '2rem' }} />
              </div>
            ) : error ? (
              <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--danger)' }}>{error}</div>
            ) : records.length === 0 ? (
              <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>No se encontraron registros</div>
            ) : (
              <table className="table" style={{ border: 'none' }}>
                <tbody>
                  {records.map(rec => (
                    <tr
                      key={rec.id}
                      onClick={() => onSelect(rec.id, rec[displayColumn])}
                      style={{ cursor: 'pointer' }}
                      className={initialValue === rec.id ? 'row-selected' : ''}
                    >
                      <td style={{ padding: '0.75rem 1rem' }}>
                        <div style={{ fontWeight: 600 }}>{rec[displayColumn] || rec.id.slice(0, 8)}</div>
                        <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>{rec.id}</div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        <div className="modal-footer" style={{ borderTop: 'none', background: 'transparent' }}>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button variant="danger" onClick={() => onSelect(null, null)}>Limpiar Selección</Button>
        </div>
      </div>
    </div>
  )
}
