import { createElement } from 'react'
import { Calendar, Clock, Link as LinkIcon, Attachment } from 'iconoir-react'
import i18n from './i18n/index.js'

/**
 * Centralized field type configuration — PocketBase style.
 *
 * Each type has: icon (string or React element), labelKey (i18n key), color,
 * and HTML input type. Labels use i18n keys so they respond to language
 * changes.
 *
 * The list below mirrors EXACTLY the 13 field types accepted by the backend
 * (`internal/handlers/api/column.go` -> `ValidFieldTypes`):
 *
 *   text, long_text, number, integer, boolean, date, datetime,
 *   email, url, attachment, select, relation, json
 *
 * Any other type (for example the legacy `float` or `file`) is rejected by
 * the API with a 400, so it must not be offered here.
 */
export const FIELD_TYPES = {
  text: { icon: 'T', labelKey: 'field_text', color: '#6366f1', inputType: 'text' },
  long_text: { icon: '≡', labelKey: 'field_long_text', color: '#6366f1', inputType: 'textarea' },
  number: { icon: '#', labelKey: 'field_number', color: '#f59e0b', inputType: 'number', step: '0.01' },
  integer: { icon: '123', labelKey: 'field_integer', color: '#f59e0b', inputType: 'number', step: '1' },
  boolean: { icon: '☑', labelKey: 'field_boolean', color: '#10b981', inputType: 'checkbox' },
  date: { icon: createElement(Calendar, { width: '1em', height: '1em' }), labelKey: 'field_date', color: '#8b5cf6', inputType: 'date' },
  datetime: { icon: createElement(Clock, { width: '1em', height: '1em' }), labelKey: 'field_datetime', color: '#8b5cf6', inputType: 'datetime-local' },
  email: { icon: '@', labelKey: 'field_email', color: '#ef4444', inputType: 'email' },
  url: { icon: createElement(LinkIcon, { width: '1em', height: '1em' }), labelKey: 'field_url', color: '#3b82f6', inputType: 'url' },
  attachment: { icon: createElement(Attachment, { width: '1em', height: '1em' }), labelKey: 'field_attachment', color: '#78716c', inputType: 'text' },
  select: { icon: '▾', labelKey: 'field_select', color: '#ec4899', inputType: 'select' },
  relation: { icon: '⟲', labelKey: 'field_relation', color: '#14b8a6', inputType: 'text' },
  json: { icon: '{ }', labelKey: 'field_json', color: '#64748b', inputType: 'textarea' },
}

/** The field types accepted by the backend, in display order. */
export const VALID_FIELD_TYPES = Object.keys(FIELD_TYPES)

/** Get config for a type — falls back to text. Label is resolved at call time via i18n. */
export function getFieldConfig(fieldType) {
  const cfg = FIELD_TYPES[fieldType] || FIELD_TYPES.text
  return { ...cfg, label: i18n.t(cfg.labelKey) }
}

/** Ordered array for UI dropdowns — labels resolved at render time */
export const FIELD_TYPE_OPTIONS = Object.entries(FIELD_TYPES).map(([value, cfg]) => ({
  value,
  ...cfg,
  get label() { return i18n.t(cfg.labelKey) },
}))

/**
 * Parse the `meta` payload of a column, tolerating malformed JSON.
 * Always returns a plain object (never throws).
 */
export function parseColumnMeta(column) {
  const raw = column?.meta
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

/**
 * Normalized options of a `select` column.
 *
 * The backend stores them as `meta.choices = [{ value, label, color }]`;
 * `meta.options` is accepted as a legacy alternative. Returns `[]` when the
 * column has no fixed options (free-form select).
 */
export function getSelectChoices(column) {
  const meta = parseColumnMeta(column)
  const raw = meta.choices ?? meta.options
  if (!Array.isArray(raw)) return []

  return raw
    .map((choice) => {
      if (typeof choice === 'string' || typeof choice === 'number') {
        return { value: String(choice), label: String(choice) }
      }
      if (choice && typeof choice === 'object' && choice.value !== undefined && choice.value !== null) {
        return {
          value: String(choice.value),
          label: String(choice.label ?? choice.value),
          color: choice.color,
        }
      }
      return null
    })
    .filter(Boolean)
}
