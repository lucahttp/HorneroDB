import { useState, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { EditPencil, Trash, Check } from 'iconoir-react'
import { useTranslation } from 'react-i18next'

/**
 * Dropdown menu for column header actions.
 * Shows on hover (desktop) via a "⋮" trigger button.
 * Dismisses on click outside or Escape.
 */
export function ColumnHeaderMenu({ column, icon, onRename, onDelete }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const [newName, setNewName] = useState(column.name)
  const [menuCoords, setMenuCoords] = useState(null)
  const menuRef = useRef(null)
  const inputRef = useRef(null)

  // Close on click outside
  useEffect(() => {
    if (!open) return
    const clickHandler = (e) => {
      if (e.target.closest('.column-menu')) return;
      if (menuRef.current && menuRef.current.contains(e.target)) return;
      setOpen(false)
      setEditing(false)
    }
    const scrollHandler = (e) => {
      if (document.activeElement && document.activeElement.closest('.column-menu')) return;

      if (!e.target.closest('.column-menu')) {
        setOpen(false)
        setEditing(false)
      }
    }
    document.addEventListener('mousedown', clickHandler, true)
    window.addEventListener('scroll', scrollHandler, true)
    return () => {
      document.removeEventListener('mousedown', clickHandler, true)
      window.removeEventListener('scroll', scrollHandler, true)
    }
  }, [open])

  // Close on Escape
  useEffect(() => {
    if (!open) return
    const handler = (e) => {
      if (e.key === 'Escape') {
        setOpen(false)
        setEditing(false)
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open])

  // Focus input when editing
  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus()
      inputRef.current.select()
    }
  }, [editing])

  const handleRename = () => {
    if (newName.trim() && newName !== column.name) {
      onRename(column.id, newName.trim())
    }
    setEditing(false)
    setOpen(false)
  }

  const handleDelete = () => {
    onDelete(column.id, column.name)
    setOpen(false)
  }

  return (
    <div className="column-header-wrapper" ref={menuRef}>
      <span className="column-header-label">{icon} {column.name}</span>
      <button
        className="column-menu-trigger"
        onClick={(e) => { 
          e.stopPropagation(); 
          if (!open) {
            const rect = menuRef.current.getBoundingClientRect()
            setMenuCoords({ top: rect.bottom + 4, left: rect.left })
            setOpen(true)
          } else {
            setOpen(false)
          }
        }}
        aria-label={t('options_for', { name: column.name })}
      >
        ⋮
      </button>

      {open && window.document && window.document.body && createPortal(
        <div className="column-menu" style={{ position: 'fixed', top: menuCoords?.top, left: menuCoords?.left, zIndex: 9999, textTransform: 'none', fontWeight: 400, letterSpacing: 'normal' }}>
          {editing ? (
            <div className="column-menu-edit">
              <input
                ref={inputRef}
                type="text"
                className="form-input"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleRename()
                  if (e.key === 'Escape') { setEditing(false); setOpen(false) }
                }}
                style={{ fontSize: '0.8125rem', padding: '0.375rem 0.5rem' }}
              />
              <button className="btn btn-primary btn-sm" onClick={handleRename}>
                <Check width="1rem" height="1rem" />
              </button>
            </div>
          ) : (
            <>
              <button
                className="column-menu-item"
                onClick={() => { setNewName(column.name); setEditing(true) }}
              >
                <EditPencil width="1rem" height="1rem" style={{ marginRight: '0.5rem' }} /> {t('edit_name')}
              </button>
              <button
                className="column-menu-item column-menu-item-danger"
                onClick={handleDelete}
              >
                <Trash width="1rem" height="1rem" style={{ marginRight: '0.5rem' }} /> {t('delete_column')}
              </button>
            </>
          )}
        </div>,
        document.body
      )}
    </div>
  )
}
