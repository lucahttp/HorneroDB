import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { ColumnHeaderMenu } from './ColumnHeaderMenu.jsx'
import { getFieldConfig, FIELD_TYPE_OPTIONS } from '../fieldTypeConfig.jsx'
import { Trash } from 'iconoir-react'
import { useTranslation } from 'react-i18next'
import { RelationPicker } from './RelationPicker.jsx'

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
  workspaceId,
  tables = [],
}) {
  const { t } = useTranslation()
  const [newRow, setNewRow] = useState({})
  const [creatingRow, setCreatingRow] = useState(false)
  const [showAddColumn, setShowAddColumn] = useState(false)
  const [newColName, setNewColName] = useState('')
  const [newColType, setNewColType] = useState('text')
  const [isTypeSelectOpen, setIsTypeSelectOpen] = useState(false)
  const [newColTargetTable, setNewColTargetTable] = useState('')
  const [newColDisplayColumn, setNewColDisplayColumn] = useState('')
  const [addColumnCoords, setAddColumnCoords] = useState(null)
  const newColInputRef = useRef(null)
  const addColumnRef = useRef(null)

  // Selection state
  const [selectedRows, setSelectedRows] = useState(new Set())

  // Inline editing state
  const [editingCell, setEditingCell] = useState(null)
  const [editValue, setEditValue] = useState('')
  const editInputRef = useRef(null)

  // Relation Picker state
  const [pickerState, setPickerState] = useState(null) // { rowId, colSlug, meta, initialValue }

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

    if (fieldType === 'relation') {
      const col = columns.find(c => c.slug === colSlug)
      let meta = {}
      try {
        meta = typeof col.meta === 'string' ? JSON.parse(col.meta) : col.meta
      } catch (e) {
        console.error("Failed to parse column meta", e)
      }
      setPickerState({ rowId, colSlug, meta, initialValue: currentValue })
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

  // Close "Add Column" popover on click outside or scroll
  useEffect(() => {
    if (!showAddColumn) return

    // Use capture phase to handle events before React's synthetic event system
    const clickHandler = (e) => {
      // If click is inside the popover portal, do nothing
      if (e.target.closest('.column-add-popover')) return;
      // If click is inside the add button, do nothing (handled by onClick)
      if (addColumnRef.current && addColumnRef.current.contains(e.target)) return;

      setShowAddColumn(false)
    }

    const scrollHandler = (e) => {
      // Only close if scrolling something outside the popover itself (like the table container or page)
      if (!e.target.closest('.column-add-popover')) {
        setShowAddColumn(false)
      }
    }

    document.addEventListener('mousedown', clickHandler, true)
    window.addEventListener('scroll', scrollHandler, true) // catch all scrolls

    return () => {
      document.removeEventListener('mousedown', clickHandler, true)
      window.removeEventListener('scroll', scrollHandler, true)
    }
  }, [showAddColumn])

  const handleCreateColumn = async () => {
    if (!newColName.trim()) return
    try {
      const meta = {}
      if (newColType === 'relation') {
        meta.target_table = newColTargetTable
        meta.display_column = newColDisplayColumn
      }
      await onCreateColumn(newColName.trim(), newColType, meta)
      setNewColName(''); setNewColType('text'); setShowAddColumn(false)
      setNewColTargetTable(''); setNewColDisplayColumn('')
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

    if (col.field_type === 'relation') {
      const val = newRow[col.slug]
      return (
        <button
          className="inline-cell-input text-left"
          onClick={() => {
            let meta = {}
            try {
              meta = typeof col.meta === 'string' ? JSON.parse(col.meta) : col.meta
            } catch (e) { }
            setPickerState({ isNew: true, colSlug: col.slug, meta, initialValue: val })
          }}
          disabled={creatingRow}
          style={{ cursor: 'pointer', color: val ? 'inherit' : 'var(--text-muted)' }}
        >
          {val ? (
            <span className="relation-chip">{String(val).slice(0, 8)}</span>
          ) : `+ Seleccionar ${col.name}`}
        </button>
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

    if (col.field_type === 'relation') {
      const expandedValue = record.expand?.[col.slug]
      return (
        <span className="relation-chip" title={String(val)}>
          {expandedValue || (val ? String(val).slice(0, 8) : '-')}
        </span>
      )
    }

    return String(val == null ? '-' : val)
  }

  // ── Render ──────────────────────────────────────────

  const hasSelection = selectedRows.size > 0
  const allSelected = records.length > 0 && selectedRows.size === records.length

  return (
    <div className="table-container" style={{ position: 'relative', overflowX: 'auto' }}>
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
            <th style={{ width: '50px', padding: 0, position: 'relative' }} ref={addColumnRef}>
              <button
                className="column-add-btn"
                onClick={(e) => {
                  if (!showAddColumn) {
                    const rect = addColumnRef.current.getBoundingClientRect()
                    setAddColumnCoords({ top: rect.bottom + 8, right: window.innerWidth - rect.right })
                    setShowAddColumn(true)
                  } else {
                    setShowAddColumn(false)
                  }
                }}
                title={t('add_column')}
              >+</button>

              {showAddColumn && window.document && window.document.body && createPortal(
                <div className="column-add-popover" style={{ position: 'fixed', top: addColumnCoords?.top, right: addColumnCoords?.right, zIndex: 9999 }}>
                  <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                    <label className="form-label" style={{ fontSize: '0.65rem' }}>{t('name')}</label>
                    <input
                      ref={newColInputRef}
                      type="text"
                      className="form-input"
                      placeholder={t('name')}
                      value={newColName}
                      onChange={(e) => setNewColName(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') handleCreateColumn()
                        // Escape naturally closes because we have global event listeners, but inline is fine too:
                        if (e.key === 'Escape') setShowAddColumn(false)
                      }}
                      autoFocus
                    />
                  </div>

                  <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                    <label className="form-label" style={{ fontSize: '0.65rem' }}>{t('type')}</label>
                    <div style={{ position: 'relative' }}>
                      <div
                        className="form-select"
                        style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', cursor: 'pointer', height: '2.5rem', background: 'var(--bg-elevated)', border: isTypeSelectOpen ? '1px solid var(--primary)' : '1px solid var(--border)' }}
                        onClick={() => setIsTypeSelectOpen(!isTypeSelectOpen)}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                          <div style={{ color: getFieldConfig(newColType).color || 'var(--text-muted)', display: 'flex', alignItems: 'center' }}>
                            {getFieldConfig(newColType).icon}
                          </div>
                          {getFieldConfig(newColType).label}
                        </div>
                        <span style={{ fontSize: '0.7rem', opacity: 0.5 }}>▼</span>
                      </div>

                      {isTypeSelectOpen && (
                        <div style={{
                          position: 'absolute', top: '100%', left: 0, right: 0, marginTop: '4px',
                          background: 'var(--bg-elevated)', border: '1px solid var(--border)', borderRadius: '6px',
                          boxShadow: 'var(--shadow-md)', zIndex: 10000, maxHeight: '220px', overflowY: 'auto'
                        }}>
                          {FIELD_TYPE_OPTIONS.map(ft => (
                            <div
                              key={ft.value}
                              onClick={() => {
                                setNewColType(ft.value)
                                setIsTypeSelectOpen(false)
                              }}
                              style={{
                                padding: '0.5rem 0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem',
                                cursor: 'pointer', background: newColType === ft.value ? 'var(--bg-subtle)' : 'transparent',
                                fontSize: '0.875rem'
                              }}
                              onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-subtle)'}
                              onMouseLeave={(e) => e.currentTarget.style.background = newColType === ft.value ? 'var(--bg-subtle)' : 'transparent'}
                            >
                              <div style={{ width: '1.25rem', height: '1.25rem', display: 'flex', alignItems: 'center', justifyContent: 'center', color: ft.color || 'var(--text-muted)' }}>
                                {ft.icon}
                              </div>
                              {ft.label}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>

                  {newColType === 'relation' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', marginBottom: '1rem', padding: '0.75rem', background: 'var(--bg-surface)', borderRadius: '6px', border: '1px solid var(--border-light)' }}>
                      <label style={{ fontSize: '0.65rem', fontWeight: 700, textTransform: 'uppercase', color: 'var(--text-muted)' }}>Tabla Destino</label>
                      <select
                        className="form-select"
                        style={{ height: 'auto', padding: '0.375rem 0.5rem' }}
                        value={newColTargetTable}
                        onChange={(e) => {
                          setNewColTargetTable(e.target.value)
                          const target = tables.find(t => t.slug === e.target.value)
                          if (target?.columns?.length) {
                            setNewColDisplayColumn(target.columns.find(c => c.slug !== 'id')?.slug || 'id')
                          }
                        }}
                      >
                        <option value="">Seleccionar tabla...</option>
                        {tables.map(t => (
                          <option key={t.id} value={t.slug}>{t.name}</option>
                        ))}
                      </select>

                      <label style={{ fontSize: '0.65rem', fontWeight: 700, textTransform: 'uppercase', color: 'var(--text-muted)', marginTop: '0.25rem' }}>Columna a Mostrar</label>
                      <select
                        className="form-select"
                        style={{ height: 'auto', padding: '0.375rem 0.5rem' }}
                        value={newColDisplayColumn}
                        onChange={(e) => setNewColDisplayColumn(e.target.value)}
                        disabled={!newColTargetTable}
                      >
                        <option value="">Seleccionar columna...</option>
                        {tables.find(t => t.slug === newColTargetTable)?.columns?.map(c => (
                          <option key={c.id} value={c.slug}>{c.name}</option>
                        ))}
                      </select>
                    </div>
                  )}

                  <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                    <button
                      className="btn btn-primary btn-sm"
                      onClick={handleCreateColumn}
                      disabled={!newColName.trim() || (newColType === 'relation' && (!newColTargetTable || !newColDisplayColumn))}
                    >{t('create')}</button>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => setShowAddColumn(false)}
                    >{t('cancel')}</button>
                  </div>
                </div>,
                window.document.body
              )}
            </th>
            <th style={{ width: '80px' }}></th>
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
                <td style={{ padding: '0.5rem' }}>
                  <div className="row-actions">
                    <button
                      className="btn btn-ghost btn-sm"
                      style={{ padding: '4px' }}
                      onClick={() => onDeleteRecord(record.id)}
                      title={t('delete_record')}
                    >
                      <Trash width="1rem" height="1rem" style={{ color: 'var(--danger)' }} />
                    </button>
                  </div>
                </td>
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
            <td></td>
          </tr>
        </tbody>
      </table>

      {pickerState && (
        <RelationPicker
          workspaceId={workspaceId}
          targetTableSlug={pickerState.meta?.target_table}
          displayColumn={pickerState.meta?.display_column}
          initialValue={pickerState.initialValue}
          onClose={() => setPickerState(null)}
          onSelect={(id, label) => {
            if (pickerState.isNew) {
              setNewRow({ ...newRow, [pickerState.colSlug]: id })
            } else {
              onUpdateRecord(pickerState.rowId, { [pickerState.colSlug]: id })
            }
            setPickerState(null)
          }}
        />
      )}
    </div >
  )
}
