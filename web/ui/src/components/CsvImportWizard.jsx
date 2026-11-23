import { useState, useRef, forwardRef, useImperativeHandle } from 'react'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { Xmark } from 'iconoir-react'
import { API_URL } from '../constants'
import { Button } from './index.jsx'

/**
 * Amount of rows sent per bulk request. The API accepts a whole batch in one
 * call, so a large file does not turn into hundreds of round-trips.
 */
const BULK_BATCH_SIZE = 200

export const CsvImportWizard = forwardRef(({ workspaceId, table, columns, onImportComplete }, ref) => {
  const { t } = useTranslation()
  const [csvWizard, setCsvWizard] = useState(null)
  const csvFileRef = useRef(null)

  useImperativeHandle(ref, () => ({
    open: () => csvFileRef.current?.click()
  }))

  /**
   * RFC 4180 style CSV parser.
   *
   * It understands escaped quotes (`""` inside a quoted field) and line
   * breaks inside quoted fields, which a naive `split('\n')` cannot handle.
   */
  const parseCSV = (text) => {
    const rows = []
    let row = []
    let field = ''
    let inQuotes = false
    let i = 0
    const len = text.length

    // Drop the UTF-8 byte order mark if the file has one.
    if (text.charCodeAt(0) === 0xfeff) i = 1

    while (i < len) {
      const ch = text[i]

      if (inQuotes) {
        if (ch === '"') {
          // `""` is an escaped quote, a single `"` closes the field.
          if (text[i + 1] === '"') {
            field += '"'
            i += 2
            continue
          }
          inQuotes = false
          i += 1
          continue
        }
        field += ch
        i += 1
        continue
      }

      if (ch === '"') {
        inQuotes = true
        i += 1
        continue
      }

      if (ch === ',') {
        row.push(field)
        field = ''
        i += 1
        continue
      }

      if (ch === '\r' || ch === '\n') {
        row.push(field)
        field = ''
        rows.push(row)
        row = []
        i += 1
        // Treat `\r\n` as a single line break.
        if (ch === '\r' && text[i] === '\n') i += 1
        continue
      }

      field += ch
      i += 1
    }

    // The last field of the file, when the file does not end with a break.
    if (field.length > 0 || row.length > 0) {
      row.push(field)
      rows.push(row)
    }

    const populated = rows.filter(r => r.some(cell => cell.trim() !== ''))
    if (!populated.length) return { headers: [], rows: [] }

    const headers = populated[0].map(h => h.trim())
    const dataRows = populated.slice(1).map(r => {
      const record = {}
      headers.forEach((h, idx) => { record[h] = (r[idx] ?? '').trim() })
      return record
    })

    return { headers, rows: dataRows }
  }

  const handleCSVFileChange = (e) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      const { headers, rows } = parseCSV(ev.target.result)
      const defaultMapping = {}
      headers.forEach(h => {
        const normalized = h.toLowerCase().replace(/\s+/g, '_')
        const match = columns.find(c => c.slug === normalized || c.slug === h.toLowerCase())
        defaultMapping[h] = match ? match.slug : '__ignore__'
      })
      setCsvWizard({
        step: 2,
        headers,
        rows,
        mapping: defaultMapping,
        importing: false,
        results: null,
        batch: 0,
        totalBatches: 0,
        imported: 0,
      })
    }
    reader.readAsText(file, 'UTF-8')
    e.target.value = ''
  }

  const handleCSVImport = async () => {
    if (!csvWizard) return
    setCsvWizard(prev => ({
      ...prev,
      step: 3,
      importing: true,
      results: [],
      batch: 0,
      totalBatches: 0,
      imported: 0,
    }))

    // Build every payload up-front so a mapping/typing problem is detected
    // before the first request goes out.
    const payloads = csvWizard.rows.map((row, index) => {
      const payload = {}
      csvWizard.headers.forEach(h => {
        const target = csvWizard.mapping[h]
        if (target && target !== '__ignore__') {
          const col = columns.find(c => c.slug === target)
          let val = row[h]
          if (col?.field_type === 'number') {
            if (val === '' || val === undefined || val === null) {
              val = null
            } else {
              const num = Number(val.replace(',', '.'))
              val = isNaN(num) ? null : num
            }
          }
          if (col?.field_type === 'boolean') {
            const v = String(val).toLowerCase().trim()
            val = v === 'true' || v === '1' || v === 'yes' || v === 'si' || v === 'sí'
          }
          payload[target] = val
        }
      })
      return { index, payload }
    })

    const totalBatches = Math.max(1, Math.ceil(payloads.length / BULK_BATCH_SIZE))
    const results = []
    let imported = 0

    setCsvWizard(prev => ({ ...prev, totalBatches }))

    for (let b = 0; b < totalBatches; b++) {
      const batch = payloads.slice(b * BULK_BATCH_SIZE, (b + 1) * BULK_BATCH_SIZE)
      if (!batch.length) continue

      // Show the progress of the batch that is being sent right now.
      setCsvWizard(prev => ({ ...prev, batch: b + 1, imported }))

      try {
        await axios.post(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/bulk`, {
          records: batch.map(item => item.payload),
        })
        batch.forEach(item => results.push({ row: item.index + 1, ok: true }))
        imported += batch.length
      } catch (err) {
        const message = err?.response?.data?.error?.message || err.message
        batch.forEach(item => results.push({ row: item.index + 1, ok: false, error: message }))
      }
    }

    setCsvWizard(prev => ({ ...prev, importing: false, results, imported }))
    if (onImportComplete) onImportComplete()
  }

  return (
    <>
      <input ref={csvFileRef} type="file" accept=".csv,text/csv" style={{ display: 'none' }} onChange={handleCSVFileChange} />
      {csvWizard && (
        <div className="modal-overlay" onClick={() => !csvWizard.importing && setCsvWizard(null)}>
          <div className="modal" onClick={e => e.stopPropagation()} style={{ maxWidth: '680px', maxHeight: '85vh', overflowY: 'auto' }}>
            <div className="modal-header">
              <h3 className="modal-title">
                {csvWizard.step === 2 ? t('csv_map_columns') : csvWizard.step === 3 ? t('csv_import_data') : t('csv_import')}
              </h3>
              {!csvWizard.importing && (
                <button className="btn btn-ghost btn-sm" onClick={() => setCsvWizard(null)} aria-label={t('aria_close_modal')}>
                  <Xmark width="1rem" height="1rem" />
                </button>
              )}
            </div>

            {csvWizard.step === 2 && (
              <div className="modal-body">
                <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                  {t('csv_map_hint')}
                </p>
                <div style={{ display: 'grid', gap: '0.75rem' }}>
                  {csvWizard.headers.map(h => (
                    <div key={h} style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', alignItems: 'center', gap: '0.75rem' }}>
                      <div style={{ padding: '0.5rem 0.75rem', background: 'var(--bg-subtle)', borderRadius: '6px', fontFamily: 'var(--font-mono)', fontSize: '0.8125rem', border: '1px solid var(--border)' }}>
                        {h}
                      </div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem' }}>→</span>
                      <select
                        className="form-select"
                        value={csvWizard.mapping[h]}
                        onChange={e => setCsvWizard(prev => ({ ...prev, mapping: { ...prev.mapping, [h]: e.target.value } }))}
                      >
                        <option value="__ignore__">{t('csv_ignore')}</option>
                        {columns.map(c => (
                          <option key={c.slug} value={c.slug}>{c.name} ({c.slug})</option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
                <p style={{ marginTop: '1rem', fontSize: '0.8125rem', color: 'var(--text-muted)' }}>
                  {t('csv_rows_detected', { count: csvWizard.rows.length })}
                </p>
              </div>
            )}

            {csvWizard.step === 3 && (
              <div className="modal-body">
                {csvWizard.importing ? (
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '1rem', padding: '2rem 0' }}>
                    <div className="loading-spinner" />
                    <p>{t('csv_importing', { count: csvWizard.rows.length })}</p>
                    {csvWizard.totalBatches > 0 && (
                      <>
                        <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                          {t('csv_batch_progress', { current: csvWizard.batch, total: csvWizard.totalBatches })}
                        </p>
                        <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                          {t('csv_rows_progress', { done: csvWizard.imported, total: csvWizard.rows.length })}
                        </p>
                        <p style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                          {t('csv_bulk_batch_size', { size: BULK_BATCH_SIZE })}
                        </p>
                      </>
                    )}
                  </div>
                ) : (
                  <div>
                    <p style={{ marginBottom: '1rem' }}>
                      ✅ {t('csv_imported', { count: csvWizard.results?.filter(r => r.ok).length })} &nbsp;·&nbsp;
                      ❌ {t('csv_errors', { count: csvWizard.results?.filter(r => !r.ok).length })}
                    </p>
                    {csvWizard.results?.filter(r => !r.ok).map(r => (
                      <div key={r.row} style={{ padding: '0.5rem 0.75rem', background: 'var(--danger-bg, #fee2e2)', borderRadius: '6px', fontSize: '0.8125rem', marginBottom: '0.5rem', color: 'var(--danger, #dc2626)' }}>
                        {t('csv_row_error', { row: r.row })}: {r.error}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            <div className="modal-footer">
              {csvWizard.step === 2 && (
                <>
                  <Button variant="secondary" onClick={() => setCsvWizard(null)}>{t('cancel')}</Button>
                  <Button onClick={handleCSVImport} disabled={!csvWizard.rows.length}>
                    {t('csv_import_records', { count: csvWizard.rows.length })}
                  </Button>
                </>
              )}
              {csvWizard.step === 3 && !csvWizard.importing && (
                <Button onClick={() => setCsvWizard(null)}>{t('close')}</Button>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
})
