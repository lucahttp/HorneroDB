import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { Xmark, NavArrowLeft, NavArrowRight, Search } from 'iconoir-react'
import { Button } from './index.jsx'
import { API_URL } from '../constants/index.js'

export function RelationPicker({
  workspaceId,
  targetTableSlug,
  displayColumn,
  onSelect,
  onClose,
  initialValue
}) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')

  /**
   * Result of the last request. `key` identifies the request the payload
   * belongs to, so responses of a previous search are discarded instead of
   * being rendered as the current result.
   */
  const [result, setResult] = useState({ key: null, records: [], errorKey: null })

  const requestKey = `${targetTableSlug || ''}|${search}`

  useEffect(() => {
    if (!targetTableSlug) return undefined

    let cancelled = false

    axios.get(`${API_URL}/workspaces/${workspaceId}/data/${targetTableSlug}`, {
      params: {
        search: search || undefined,
        limit: 50
      }
    })
      .then(resp => {
        if (cancelled) return
        setResult({ key: requestKey, records: resp.data.data || [], errorKey: null })
      })
      .catch(() => {
        if (cancelled) return
        setResult({ key: requestKey, records: [], errorKey: 'error_loading_records' })
      })

    return () => { cancelled = true }
  }, [workspaceId, targetTableSlug, search, requestKey])

  const loading = !!targetTableSlug && result.key !== requestKey
  const records = result.key === requestKey ? result.records : []
  const error = !targetTableSlug
    ? t('relation_config_incomplete')
    : (result.key === requestKey && result.errorKey ? t(result.errorKey) : null)

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '500px' }}>
        <div className="modal-header" style={{ background: 'var(--bg-elevated)', borderBottom: '1.5px solid var(--border-light)' }}>
          <h3 className="modal-title">{t('select_record')}</h3>
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
              placeholder={t('search_in', { table: targetTableSlug })}
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
              <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>{t('no_records_found')}</div>
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
          <Button variant="secondary" onClick={onClose}>{t('cancel')}</Button>
          <Button variant="danger" onClick={() => onSelect(null, null)}>{t('clear_selection')}</Button>
        </div>
      </div>
    </div>
  )
}
