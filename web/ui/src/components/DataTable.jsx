import { useState, useRef, useEffect, useCallback } from 'react'
import { ColumnHeaderMenu } from './ColumnHeaderMenu.jsx'
import { getFieldConfig, FIELD_TYPE_OPTIONS } from '../fieldTypeConfig.jsx'
import { Trash } from 'iconoir-react'
import { useTranslation } from 'react-i18next'

/**
 * Spreadsheet-style data table with:
 * - PocketBase-style type icons in column headers
 * - Per-type inputs (date picker, checkbox, textarea, etc.)
 * - Checkbox row selection + select-all + bulk actions toolbar
 * - Inline cell editing on click (Enter saves, Escape cancels)
 * - Inline "ghost row" at bottom for creating new records
 * - "+" button in header for inline column creation
 * - Column header dropdown menus for edit/delete
 */
export function DataTable({
  columns,
  records,
  onCreateRecord,
  onDeleteRecord,
  onUpdateRecord,
  onBulkDelete,
  onCreateColumn,
  onDeleteColumn,
  onRenameColumn,
}) {
  const { t } = useTranslation()
  const [newRow, setNewRow] = useState({})
  const [creatingRow, setCreatingRow] = useState(false)
  const [showAddColumn, setShowAddColumn] = useState(false)
  const [newColName, setNewColName] = useState('')
  const [newColType, setNewColType] = useState('text')
  const newColInputRef = useRef(null)

  // Selection state
  const [selectedRows, setSelectedRows] = useState(new Set())

  // Inline editing state
  const [editingCell, setEditingCell] = useState(null)
  const [editValue, setEditValue] = useState('')
  const editInputRef = useRef(null)

  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus()
      if (editInputRef.current.select) editInputRef.current.select()
    }
  }, [editingCell])

  useEffect(() => {
    setSelectedRows(new Set())
  }, [records])

  // ── Selection ───────────────────────────────────────

  const toggleRow = useCallback((id) => {
    setSelectedRows(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const toggleAll = useCallback(() => {
    if (selectedRows.size === records.length) {
      setSelectedRows(new Set())
    } else {
      setSelectedRows(new Set(records.map(r => r.id)))
    }
  }, [records, selectedRows.size])

  const handleBulkDelete = () => {
    const count = selectedRows.size
    if (!count) return
    if (confirm(t('confirm_delete_records', { count }))) {
      onBulkDelete([...selectedRows])
    }
  }

  // ── Inline editing ──────────────────────────────────

  const startEditing = (rowId, colSlug, fieldType, currentValue) => {
    const cfg = getFieldConfig(fieldType)
    if (cfg.inputType === 'checkbox') {
      // Toggle immediately for booleans
      const newVal = currentValue === true || currentValue === 'true' ? false : true
      onUpdateRecord(rowId, { [colSlug]: newVal })
      return
    }
    setEditingCell({ rowId, colSlug, fieldType })
    setEditValue(currentValue == null || currentValue === '-' ? '' : String(currentValue))
  }

  const saveEdit = async () => {
    if (!editingCell) return
    const { rowId, colSlug } = editingCell
    const original = records.find(r => r.id === rowId)
    const originalValue = String(original?.[colSlug] || '')

    if (editValue !== originalValue) {
      try {
        await onUpdateRecord(rowId, { [colSlug]: editValue })
      } catch (err) { /* parent handles */ }
    }
    setEditingCell(null)
  }

  const cancelEdit = () => {
    setEditingCell(null)
    setEditValue('')
  }

  const handleEditKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); saveEdit() }
    if (e.key === 'Escape') { e.preventDefault(); cancelEdit() }
    if (e.key === 'Tab') { e.preventDefault(); saveEdit() }
  }

  // ── Ghost row ───────────────────────────────────────

  const handleCreateRow = async () => {
    const hasData = Object.values(newRow).some(v => v !== '' && v !== undefined)
    if (!hasData || creatingRow) return
    setCreatingRow(true)
    try {
      await onCreateRecord(newRow)
      setNewRow({})
    } catch (err) { /* parent */ }
    finally { setCreatingRow(false) }
  }

  const handleRowKeyDown = (e, colIndex) => {
    if (e.key === 'Enter') { e.preventDefault(); handleCreateRow() }
    if (e.key === 'Tab' && !e.shiftKey && colIndex === columns.length - 1) {
      e.preventDefault(); handleCreateRow()
    }
  }

  // ── Column creation ─────────────────────────────────

  const handleCreateColumn = async () => {
    if (!newColName.trim()) return
    try {
      await onCreateColumn(newColName.trim(), newColType)
      setNewColName(''); setNewColType('text'); setShowAddColumn(false)
    } catch (err) { /* parent */ }
  }

  const handleDeleteColumn = (id, name) => {
    if (confirm(t('confirm_delete_column', { name }))) {
      onDeleteColumn(id)
    }
  }

  // ── Render helpers ──────────────────────────────────

  /** Render appropriate input for editing a cell */
  const renderEditInput = (fieldType) => {
    const cfg = getFieldConfig(fieldType)

    if (cfg.inputType === 'textarea') {
      return (
        <textarea
          ref={editInputRef}
          className="inline-cell-input inline-cell-editing"
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') { e.preventDefault(); cancelEdit() }
            if (e.key === 'Tab') { e.preventDefault(); saveEdit() }
          }}
          onBlur={saveEdit}
          rows={2}
          style={{ resize: 'vertical', minHeight: '2rem' }}
        />
      )
    }

    return (
      <input
        ref={editInputRef}
        type={cfg.inputType}
        step={cfg.step}
        className="inline-cell-input inline-cell-editing"
        value={editValue}
        onChange={(e) => setEditValue(e.target.value)}
        onKeyDown={handleEditKeyDown}
        onBlur={saveEdit}
      />
    )
  }

  /** Render appropriate input for the ghost row */
  const renderGhostInput = (col, colIndex) => {
    const cfg = getFieldConfig(col.field_type)

    if (cfg.inputType === 'checkbox') {
      return (
        <input
          type="checkbox"
          className="row-checkbox"
          checked={newRow[col.slug] === true || newRow[col.slug] === 'true'}
          onChange={(e) => setNewRow({ ...newRow, [col.slug]: e.target.checked })}
          disabled={creatingRow}
        />
      )
    }

    return (
      <input
        type={cfg.inputType === 'textarea' ? 'text' : cfg.inputType}
        step={cfg.step}
        className="inline-cell-input"
        placeholder={col.name}
        value={newRow[col.slug] || ''}
        onChange={(e) => setNewRow({ ...newRow, [col.slug]: e.target.value })}
        onKeyDown={(e) => handleRowKeyDown(e, colIndex)}
        disabled={creatingRow}
      />
    )
  }

  /** Render a cell value with type-aware display */
  const renderCellValue = (record, col) => {
    const val = record[col.slug]
    const cfg = getFieldConfig(col.field_type)

    if (cfg.inputType === 'checkbox') {
      return (
        <span className={`bool-badge ${val === true || val === 'true' ? 'bool-true' : 'bool-false'}`}>
          {val === true || val === 'true' ? '✓' : '✗'}
        </span>
      )
    }

    if (col.field_type === 'url' && val) {
      return <a href={val} target="_blank" rel="noopener" className="cell-link">{String(val)}</a>
    }

    if (col.field_type === 'email' && val) {
      return <a href={`mailto:${val}`} className="cell-link">{String(val)}</a>
    }

    if (col.field_type === 'json' && val) {
      return <code className="cell-json">{typeof val === 'object' ? JSON.stringify(val) : String(val)}</code>
    }

    return String(val == null ? '-' : val)
  }

  // ── Render ──────────────────────────────────────────

  const hasSelection = selectedRows.size > 0
  const allSelected = records.length > 0 && selectedRows.size === records.length

  return (
    <div className="table-container" style={{ position: 'relative' }}>
      {/* Bulk actions toolbar */}
      {hasSelection && (
        <div className="bulk-toolbar">
          <label className="bulk-toolbar-count">
            {t('selected_count', { count: selectedRows.size })}
          </label>
          <button className="btn btn-danger btn-sm" onClick={handleBulkDelete}>
            <Trash width="1rem" height="1rem" style={{ marginRight: '0.5rem' }} /> {t('delete_selected')}
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => setSelectedRows(new Set())}>
            {t('deselect')}
          </button>
        </div>
      )}

      <table className="table">
        <thead>
          <tr>
            <th style={{ width: '40px', textAlign: 'center' }}>
              <input
                type="checkbox"
                className="row-checkbox"
                checked={allSelected}
                onChange={toggleAll}
                title={allSelected ? t('deselect_all') : t('select_all')}
              />
            </th>
            <th style={{ width: '100px' }}>
              <span className="field-type-icon" style={{ background: '#94a3b8', color: '#fff' }}>⚿</span>
              ID
            </th>
            {columns.map(col => {
              const cfg = getFieldConfig(col.field_type)
              return (
                <th key={col.id} style={{ position: 'relative' }}>
                  <ColumnHeaderMenu
                    column={col}
                    icon={<span className="field-type-icon" style={{ background: cfg.color }}>{cfg.icon}</span>}
                    onRename={onRenameColumn}
                    onDelete={handleDeleteColumn}
                  />
                </th>
              )
            })}
            {/* "+" column header */}
            <th style={{ width: '200px', padding: 0 }}>
              {showAddColumn ? (
                <div className="column-add-form">
                  <input
                    ref={newColInputRef}
                    type="text"
                    className="column-add-input"
                    placeholder={t('name')}
                    value={newColName}
                    onChange={(e) => setNewColName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleCreateColumn()
                      if (e.key === 'Escape') setShowAddColumn(false)
                    }}
                    autoFocus
                  />
                  <select
                    className="column-add-select"
                    value={newColType}
                    onChange={(e) => setNewColType(e.target.value)}
                  >
                    {FIELD_TYPE_OPTIONS.map(ft => (
                      <option key={ft.value} value={ft.value}>{ft.icon} {ft.label}</option>
                    ))}
                  </select>
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={handleCreateColumn}
                    disabled={!newColName.trim()}
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                  >✓</button>
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => setShowAddColumn(false)}
                    style={{ padding: '0.25rem 0.375rem', fontSize: '0.75rem' }}
                  >✕</button>
                </div>
              ) : (
                <button
                  className="column-add-btn"
                  onClick={() => setShowAddColumn(true)}
                  title={t('add_column')}
                >+</button>
              )}
            </th>
          </tr>
        </thead>
        <tbody>
          {records.map((record, i) => {
            const isSelected = selectedRows.has(record.id)
            return (
              <tr key={record.id || i} className={isSelected ? 'row-selected' : ''}>
                <td style={{ textAlign: 'center' }}>
                  <input
                    type="checkbox"
                    className="row-checkbox"
                    checked={isSelected}
                    onChange={() => toggleRow(record.id)}
                  />
                </td>
                <td>
                  <code style={{
                    fontSize: '0.75rem',
                    fontFamily: 'var(--font-mono)',
                    background: 'var(--bg-surface)',
                    padding: '0.25rem 0.5rem',
                    borderRadius: '4px',
                    border: '1px solid var(--border-light)',
                  }}>
                    {String(record.id)?.slice(0, 8)}
                  </code>
                </td>
                {columns.map(col => {
                  const isEditing = editingCell?.rowId === record.id && editingCell?.colSlug === col.slug
                  return (
                    <td
                      key={col.id}
                      className={isEditing ? 'cell-editing' : 'cell-clickable'}
                      onClick={() => {
                        if (!isEditing) startEditing(record.id, col.slug, col.field_type, record[col.slug])
                      }}
                    >
                      {isEditing
                        ? renderEditInput(editingCell.fieldType)
                        : renderCellValue(record, col)
                      }
                    </td>
                  )
                })}
                <td></td>
              </tr>
            )
          })}

          {/* Ghost row */}
          <tr className="inline-new-row">
            <td></td>
            <td style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
              {creatingRow ? (
                <div className="loading-spinner" style={{ width: '16px', height: '16px', borderWidth: '2px' }} />
              ) : t('new')}
            </td>
            {columns.map((col, colIndex) => (
              <td key={col.id}>
                {renderGhostInput(col, colIndex)}
              </td>
            ))}
            <td></td>
          </tr>
        </tbody>
      </table>
    </div >
  )
}
