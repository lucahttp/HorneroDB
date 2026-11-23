import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { ColumnHeaderMenu } from './ColumnHeaderMenu.jsx'
import { getFieldConfig, FIELD_TYPE_OPTIONS, getSelectChoices, parseColumnMeta } from '../fieldTypeConfig.js'
import { Trash, EditPencil, CheckCircle, Lock } from 'iconoir-react'
import { useTranslation } from 'react-i18next'
import { RelationPicker } from './RelationPicker.jsx'
import { isColumnReadable, isColumnWritable, isColumnCreatable, hasSystemAccess } from '../utils/permissions.js'

/**
 * Spreadsheet-style data table with:
 * - PocketBase-style type icons in column headers
 * - Per-type inputs (date picker, checkbox, textarea, real select, etc.)
 * - Checkbox row selection + select-all + bulk actions toolbar
 * - Inline cell editing on click (Enter saves, Escape cancels)
 * - Inline "ghost row" at bottom for creating new records
 * - "+" button in header for inline column creation
 * - Column header dropdown menus for edit/delete
 * - CLS permission awareness: restricted columns are locked and hidden actions
 *   are removed instead of failing with a 403.
 *
 * `pagination` (optional):
 *   { total, page, pageSize, onPageChange, onPageSizeChange }
 * `permissions` (optional): effective permissions of the current role, as
 *   returned by `getEffectivePermissions()`.
 * `sortBy` (optional): `{ column, direction }` with `direction` in
 *   `asc | desc` and `column` in `null | <column slug>`.
 * `onSortChange` (optional): called with the column slug that should be
 *   sorted next. The parent owns the state and asks the API for the rows.
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
  pagination = null,
  permissions = null,
  sortBy = null,
  onSortChange = null,
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
  const [newColPrefix, setNewColPrefix] = useState('')
  const [newColDigits, setNewColDigits] = useState('3')
  const [newColStartVal, setNewColStartVal] = useState('1')
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

  // ── Effective permissions of the current role (CLS) ───────────
  const hasTableAccess = (level) => level === 'all' || level === 'own'
  const canUpdateRecords = !permissions || hasTableAccess(permissions.update)
  const canDeleteRecords = !permissions || hasTableAccess(permissions.delete)
  const canCreateRecords = !permissions || hasTableAccess(permissions.create)
  const canManageTables = !permissions || hasSystemAccess(permissions.system, 'tables', 'manage')

  const columnIsReadable = (col) => !permissions || isColumnReadable(permissions.readableColumns, col.slug)
  const columnIsWritable = (col) => canUpdateRecords && (!permissions || isColumnWritable(permissions.writableColumns, col.slug))
  const columnIsCreatable = (col) => canCreateRecords && (!permissions || isColumnCreatable(permissions.creatableColumns, col.slug))

  /**
   * `aria-sort` value of a column header: the column is sorted ascending,
   * descending, or not sorted at all.
   */
  const ariaSortFor = (col) => {
    if (!sortBy || sortBy.column !== col.slug) return 'none'
    return sortBy.direction === 'desc' ? 'descending' : 'ascending'
  }

  useEffect(() => {
    if (editingCell && editInputRef.current) {
      editInputRef.current.focus()
      // <select> elements expose select() too — only text-like inputs should
      // have their content pre-selected.
      if (editInputRef.current.tagName !== 'SELECT' && editInputRef.current.select) {
        editInputRef.current.select()
      }
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
      setSelectedRows(new Set())
    }
  }

  const handleEditSelected = () => {
    if (selectedRows.size !== 1) return
    const id = [...selectedRows][0]
    const record = records.find(r => r.id === id)
    if (record && columns.length > 0) {
      const firstCol = columns.find(c => columnIsWritable(c))
      if (!firstCol) return
      startEditing(id, firstCol.slug, firstCol.field_type, record[firstCol.slug])
      setSelectedRows(new Set())
    }
  }

  // ── Inline editing ──────────────────────────────────

  const startEditing = (rowId, colSlug, fieldType, currentValue) => {
    const col = columns.find(c => c.slug === colSlug)

    // The role is not allowed to write this column: the cell is read-only.
    if (col && !columnIsWritable(col)) return

    const cfg = getFieldConfig(fieldType)
    if (cfg.inputType === 'checkbox') {
      // Toggle immediately for booleans
      const newVal = currentValue === true || currentValue === 'true' ? false : true
      onUpdateRecord(rowId, { [colSlug]: newVal })
      return
    }

    if (fieldType === 'relation') {
      const meta = parseColumnMeta(col)
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
      } catch { /* parent handles */ }
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
    } catch { /* parent */ }
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
      // e.target may not be an Element (e.g. document), guard against that
      if (e.target?.closest && e.target.closest('.column-add-popover')) return;
      // If click is inside the add button, do nothing (handled by onClick)
      if (addColumnRef.current && addColumnRef.current.contains(e.target)) return;

      setShowAddColumn(false)
    }

    const scrollHandler = (e) => {
      // If focus is inside the popover, don't close it on scroll (handling mobile keyboard opening)
      if (document.activeElement && document.activeElement.closest('.column-add-popover')) return;

      // e.target can be document or non-Element nodes which lack .closest()
      if (!e.target?.closest || !e.target.closest('.column-add-popover')) {
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
      } else if (newColType === 'autonumber') {
        meta.prefix = newColPrefix
        meta.digits = parseInt(newColDigits, 10) || 0
        meta.current_value = parseInt(newColStartVal, 10) || 1
      }
      await onCreateColumn(newColName.trim(), newColType, meta)
      setNewColName(''); setNewColType('text'); setShowAddColumn(false)
      setNewColTargetTable(''); setNewColDisplayColumn('')
      setNewColPrefix(''); setNewColDigits('3'); setNewColStartVal('1')
    } catch { /* parent */ }
  }

  const handleDeleteColumn = (id, name) => {
    if (confirm(t('confirm_delete_column', { name }))) {
      onDeleteColumn(id)
    }
  }

  // ── Render helpers ──────────────────────────────────

  /** Render appropriate input for editing a cell */
  const renderEditInput = (fieldType, column) => {
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

    // A select column with fixed options is edited through a real <select>.
    if (cfg.inputType === 'select') {
      const choices = getSelectChoices(column)
      if (choices.length > 0) {
        return (
          <select
            ref={editInputRef}
            className="inline-cell-input inline-cell-editing"
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            onKeyDown={handleEditKeyDown}
            onBlur={saveEdit}
          >
            <option value="">{t('select_placeholder')}</option>
            {choices.map(choice => (
              <option key={choice.value} value={choice.value}>{choice.label}</option>
            ))}
          </select>
        )
      }
    }

    return (
      <input
        ref={editInputRef}
        type={cfg.inputType === 'textarea' || cfg.inputType === 'select' ? 'text' : cfg.inputType}
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
    // The API drops every column the role may not write, so the input is
    // disabled instead of silently sending a value that gets discarded.
    const creatable = columnIsCreatable(col)

    if (cfg.inputType === 'checkbox') {
      return (
        <input
          type="checkbox"
          className="row-checkbox"
          checked={newRow[col.slug] === true || newRow[col.slug] === 'true'}
          onChange={(e) => setNewRow({ ...newRow, [col.slug]: e.target.checked })}
          disabled={creatingRow || !creatable}
        />
      )
    }

    if (cfg.inputType === 'select') {
      const choices = getSelectChoices(col)
      if (choices.length > 0) {
        return (
          <select
            className="inline-cell-input"
            value={newRow[col.slug] || ''}
            onChange={(e) => setNewRow({ ...newRow, [col.slug]: e.target.value })}
            onKeyDown={(e) => handleRowKeyDown(e, colIndex)}
            disabled={creatingRow || !creatable}
          >
            <option value="">{t('select_placeholder')}</option>
            {choices.map(choice => (
              <option key={choice.value} value={choice.value}>{choice.label}</option>
            ))}
          </select>
        )
      }
    }

    if (col.field_type === 'autonumber') {
      return (
        <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', fontStyle: 'italic', padding: '0 4px' }}>
          (Auto ID)
        </span>
      )
    }

    if (col.field_type === 'relation') {
      const val = newRow[col.slug]
      return (
        <button
          className="inline-cell-input text-left"
          onClick={() => {
            const meta = parseColumnMeta(col)
            setPickerState({ isNew: true, colSlug: col.slug, meta, initialValue: val })
          }}
          disabled={creatingRow || !creatable}
          style={{ cursor: 'pointer', color: val ? 'inherit' : 'var(--text-muted)' }}
        >
          {val ? (
            <span className="relation-chip">{String(val).slice(0, 8)}</span>
          ) : t('select_relation', { name: col.name })}
        </button>
      )
    }

    return (
      <input
        type={cfg.inputType === 'textarea' || cfg.inputType === 'select' ? 'text' : cfg.inputType}
        step={cfg.step}
        className="inline-cell-input"
        placeholder={col.name}
        value={newRow[col.slug] || ''}
        onChange={(e) => setNewRow({ ...newRow, [col.slug]: e.target.value })}
        onKeyDown={(e) => handleRowKeyDown(e, colIndex)}
        disabled={creatingRow || !creatable}
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

    if (col.field_type === 'select' && val !== null && val !== undefined && val !== '') {
      const choices = getSelectChoices(col)
      const match = choices.find(choice => choice.value === String(val))
      return <span className="cell-select">{match ? match.label : String(val)}</span>
    }

    if (col.field_type === 'autonumber' && val != null) {
      return (
        <span style={{
          fontFamily: 'var(--font-mono, monospace)',
          fontSize: '0.8rem',
          fontWeight: 600,
          color: '#0891b2',
          background: 'rgba(6, 182, 212, 0.12)',
          padding: '2px 8px',
          borderRadius: '4px',
          letterSpacing: '0.5px'
        }}>
          {String(val)}
        </span>
      )
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

  // ── Pagination ────────────────────────────────────────
  const PAGE_SIZE_OPTIONS = [25, 50, 100]
  const totalRecords = pagination?.total ?? records.length
  const currentPage = pagination?.page ?? 0
  const currentPageSize = pagination?.pageSize ?? 50
  const totalPages = Math.max(1, Math.ceil(totalRecords / (currentPageSize || 1)))
  const firstRecord = totalRecords === 0 ? 0 : currentPage * currentPageSize + 1
  const lastRecord = Math.min(totalRecords, (currentPage + 1) * currentPageSize)

  return (
    <div className="table-container" style={{ position: 'relative' }}>
      {/* Bulk actions toolbar — it lives outside of the scroll region, so the
          actions are reachable no matter how far the rows are scrolled. */}
      {hasSelection && (
        <div className="bulk-toolbar" style={{ flexWrap: 'nowrap', gap: '0.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flex: 1 }}>
            <label className="bulk-toolbar-count" style={{ whiteSpace: 'nowrap' }}>
              {selectedRows.size} {selectedRows.size === 1 ? t('selected') : t('selected_plural')}
            </label>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <button className="btn btn-secondary btn-sm" onClick={toggleAll}>
                {allSelected ? t('deselect_all') : t('select_all')}
              </button>
              {selectedRows.size === 1 && canUpdateRecords && (
                <button className="btn btn-secondary btn-sm" onClick={handleEditSelected}>
                  <EditPencil width="1rem" height="1rem" style={{ marginRight: '0.35rem' }} />
                  {t('edit')}
                </button>
              )}
            </div>
          </div>

          <div style={{ display: 'flex', gap: '0.5rem' }}>
            {canDeleteRecords && (
              <button className="btn btn-danger btn-sm" onClick={handleBulkDelete}>
                <Trash width="1rem" height="1rem" style={{ marginRight: '0.35rem' }} />
                {t('delete_selected')}
              </button>
            )}
            <button className="btn btn-ghost btn-sm" onClick={() => setSelectedRows(new Set())}>
              {t('cancel')}
            </button>
          </div>
        </div>
      )}

      {/* The grid scrolls on its own, so the header row can stay pinned. */}
      <div className="data-table-scroll">
        <table className="table data-table">
        <thead>
          <tr>
            <th className={hasSelection ? "" : "hidden sm:table-cell"} style={{ width: '40px', textAlign: 'center' }}>
              <input
                type="checkbox"
                className="row-checkbox"
                checked={allSelected}
                onChange={toggleAll}
                title={allSelected ? t('deselect_all') : t('select_all')}
                aria-label={allSelected ? t('deselect_all') : t('select_all')}
              />
            </th>
            <th className="hidden sm:table-cell" style={{ width: '100px' }}>
              <span className="field-type-icon" style={{ background: '#94a3b8', color: '#fff' }}>⚿</span>
              ID
            </th>
            {columns.map(col => {
              const cfg = getFieldConfig(col.field_type)
              const readable = columnIsReadable(col)
              const writable = columnIsWritable(col)
              return (
                <th
                  key={col.id}
                  aria-sort={ariaSortFor(col)}
                >
                  <ColumnHeaderMenu
                    column={col}
                    icon={<span className="field-type-icon" style={{ background: cfg.color }}>{cfg.icon}</span>}
                    onRename={onRenameColumn}
                    onDelete={handleDeleteColumn}
                    canManage={canManageTables}
                    onSort={onSortChange}
                    sortState={sortBy}
                  />
                  {!readable && (
                    <span
                      className="col-restricted-badge"
                      title={t('column_read_restricted')}
                      style={{ marginLeft: '0.35rem' }}
                    >
                      <Lock width="0.7rem" height="0.7rem" />
                    </span>
                  )}
                  {readable && !writable && (
                    <span
                      className="col-restricted-badge"
                      title={t('column_write_restricted')}
                      style={{ marginLeft: '0.35rem' }}
                    >
                      <Lock width="0.7rem" height="0.7rem" />
                    </span>
                  )}
                </th>
              )
            })}
            {/* "+" column header */}
            <th style={{ width: '50px', padding: 0 }} ref={addColumnRef}>
              {canManageTables && (
                <button
                  className="column-add-btn"
                  onClick={() => {
                    if (!showAddColumn) {
                      const rect = addColumnRef.current.getBoundingClientRect()
                      setAddColumnCoords({ top: rect.bottom + 8, right: window.innerWidth - rect.right })
                      setShowAddColumn(true)
                    } else {
                      setShowAddColumn(false)
                    }
                  }}
                  title={t('add_column')}
                  aria-label={t('add_column')}
                >+</button>
              )}

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
                      <label style={{ fontSize: '0.65rem', fontWeight: 700, textTransform: 'uppercase', color: 'var(--text-muted)' }}>{t('target_table')}</label>
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
                        <option value="">{t('select_table_placeholder')}</option>
                        {tables.map(t => (
                          <option key={t.id} value={t.slug}>{t.name}</option>
                        ))}
                      </select>

                      <label style={{ fontSize: '0.65rem', fontWeight: 700, textTransform: 'uppercase', color: 'var(--text-muted)', marginTop: '0.25rem' }}>{t('display_column')}</label>
                      <select
                        className="form-select"
                        style={{ height: 'auto', padding: '0.375rem 0.5rem' }}
                        value={newColDisplayColumn}
                        onChange={(e) => setNewColDisplayColumn(e.target.value)}
                        disabled={!newColTargetTable}
                      >
                        <option value="">{t('select_column_placeholder')}</option>
                        {tables.find(t => t.slug === newColTargetTable)?.columns?.map(c => (
                          <option key={c.id} value={c.slug}>{c.name}</option>
                        ))}
                      </select>
                    </div>
                  )}
                  {newColType === 'autonumber' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', marginBottom: '1rem', padding: '0.75rem', background: 'var(--bg-surface)', borderRadius: '6px', border: '1px solid var(--border-light)' }}>
                      <label style={{ fontSize: '0.65rem', fontWeight: 700, textTransform: 'uppercase', color: 'var(--text-muted)' }}>Configuración de Secuencia</label>

                      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.5rem' }}>
                        <div>
                          <label style={{ fontSize: '0.65rem', color: 'var(--text-muted)' }}>Prefijo (Opcional, ej: PED-)</label>
                          <input
                            type="text"
                            className="form-input"
                            style={{ height: 'auto', padding: '0.35rem 0.5rem', fontSize: '0.8rem' }}
                            placeholder="Sin prefijo"
                            value={newColPrefix}
                            onChange={(e) => setNewColPrefix(e.target.value)}
                          />
                        </div>

                        <div>
                          <label style={{ fontSize: '0.65rem', color: 'var(--text-muted)' }}>Ceros (ej: 3)</label>
                          <input
                            type="number"
                            className="form-input"
                            style={{ height: 'auto', padding: '0.35rem 0.5rem', fontSize: '0.8rem' }}
                            placeholder="3 (001, 002...)"
                            value={newColDigits}
                            onChange={(e) => setNewColDigits(e.target.value)}
                          />
                        </div>
                      </div>

                      <div>
                        <label style={{ fontSize: '0.65rem', color: 'var(--text-muted)' }}>Inicio de secuencia</label>
                        <input
                          type="number"
                          className="form-input"
                          style={{ height: 'auto', padding: '0.35rem 0.5rem', fontSize: '0.8rem' }}
                          value={newColStartVal}
                          onChange={(e) => setNewColStartVal(e.target.value)}
                        />
                      </div>

                      <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', background: 'var(--bg-elevated)', padding: '0.35rem 0.5rem', borderRadius: '4px', border: '1px solid var(--border-subtle, #e2e8f0)' }}>
                        Vista previa: <strong style={{ color: '#0891b2', fontFamily: 'var(--font-mono, monospace)' }}>
                          {newColPrefix || ''}{String(newColStartVal || 1).padStart(parseInt(newColDigits) || 0, '0')}
                        </strong>
                      </div>
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
              <tr
                key={record.id || i}
                className={isSelected ? 'row-selected' : ''}
                onClick={() => {
                  if (hasSelection) toggleRow(record.id)
                }}
              >
                <td className={hasSelection ? "" : "hidden sm:table-cell"} style={{ textAlign: 'center' }}>
                  <input
                    type="checkbox"
                    className="row-checkbox"
                    checked={isSelected}
                    aria-label={t('select')}
                    onChange={(e) => {
                      e.stopPropagation();
                      toggleRow(record.id);
                    }}
                  />
                </td>
                <td className="hidden sm:table-cell">
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
                  const readable = columnIsReadable(col)
                  const writable = columnIsWritable(col)
                  const cellClass = isEditing
                    ? 'cell-editing'
                    : (writable ? 'cell-clickable' : 'cell-locked')
                  return (
                    <td
                      key={col.id}
                      className={readable ? cellClass : `${cellClass} cell-restricted`}
                      title={!readable ? t('column_read_restricted') : (!writable ? t('column_write_restricted') : undefined)}
                      onClick={(e) => {
                        if (hasSelection) {
                          e.stopPropagation();
                          toggleRow(record.id);
                        } else if (!isEditing && writable) {
                          startEditing(record.id, col.slug, col.field_type, record[col.slug])
                        }
                      }}
                    >
                      {isEditing
                        ? renderEditInput(editingCell.fieldType, col)
                        : (readable
                          ? renderCellValue(record, col)
                          : <span className="cell-restricted-placeholder"><Lock width="0.75rem" height="0.75rem" /></span>)
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
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleRow(record.id);
                      }}
                      title={t('select')}
                      aria-label={t('select')}
                    >
                      <CheckCircle width="1rem" height="1rem" style={{ color: isSelected ? 'var(--primary)' : 'var(--text-muted)' }} />
                    </button>
                    {canDeleteRecords && (
                      <button
                        className="btn btn-ghost btn-sm"
                        style={{ padding: '4px' }}
                        onClick={(e) => {
                          e.stopPropagation();
                          onDeleteRecord(record.id);
                        }}
                        title={t('delete_record')}
                        aria-label={t('delete_record')}
                      >
                        <Trash width="1rem" height="1rem" style={{ color: 'var(--danger)' }} />
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            )
          })}

          {/* Ghost row — only shown when the role may create records */}
          {canCreateRecords && (
            <tr className="inline-new-row">
              <td className={hasSelection ? "" : "hidden sm:table-cell"}></td>
              <td className="hidden sm:table-cell" style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
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
          )}
        </tbody>
      </table>
      </div>

      {pagination && (
        <div className="pagination-bar">
          <span className="pagination-info">
            {totalRecords === 0
              ? t('pagination_empty')
              : t('pagination_showing', { from: firstRecord, to: lastRecord, total: totalRecords })}
          </span>
          <div className="pagination-controls">
            <select
              className="form-select pagination-page-size"
              value={currentPageSize}
              onChange={(e) => pagination.onPageSizeChange(Number(e.target.value))}
              aria-label={t('pagination_page_size')}
            >
              {PAGE_SIZE_OPTIONS.map(size => (
                <option key={size} value={size}>{size}</option>
              ))}
            </select>
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => pagination.onPageChange(currentPage - 1)}
              disabled={currentPage <= 0}
            >
              {t('previous')}
            </button>
            <span className="pagination-page-indicator">
              {currentPage + 1} / {totalPages}
            </span>
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => pagination.onPageChange(currentPage + 1)}
              disabled={currentPage + 1 >= totalPages}
            >
              {t('next')}
            </button>
          </div>
        </div>
      )}

      {pickerState && (
        <RelationPicker
          workspaceId={workspaceId}
          targetTableSlug={pickerState.meta?.target_table}
          displayColumn={pickerState.meta?.display_column}
          initialValue={pickerState.initialValue}
          onClose={() => setPickerState(null)}
          onSelect={(id) => {
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
