import { Calendar, Clock, Link as LinkIcon, Attachment } from 'iconoir-react'
import i18n from './i18n/index.js'

/**
 * Centralized field type configuration — PocketBase style.
 * Each type has: icon (string or React element), labelKey (i18n key), color, and HTML input type.
 * Labels use i18n keys so they respond to language changes.
 */

export const FIELD_TYPES = {
  text: { icon: 'T', labelKey: 'field_text', color: '#6366f1', inputType: 'text' },
  long_text: { icon: '≡', labelKey: 'field_long_text', color: '#6366f1', inputType: 'textarea' },
  number: { icon: '#', labelKey: 'field_number', color: '#f59e0b', inputType: 'number', step: '0.01' },
  integer: { icon: '123', labelKey: 'field_integer', color: '#f59e0b', inputType: 'number', step: '1' },
  float: { icon: '#.#', labelKey: 'field_float', color: '#f59e0b', inputType: 'number', step: '0.01' },
  boolean: { icon: '☑', labelKey: 'field_boolean', color: '#10b981', inputType: 'checkbox' },
  date: { icon: <Calendar width="1em" height="1em" />, labelKey: 'field_date', color: '#8b5cf6', inputType: 'date' },
  datetime: { icon: <Clock width="1em" height="1em" />, labelKey: 'field_datetime', color: '#8b5cf6', inputType: 'datetime-local' },
  email: { icon: '@', labelKey: 'field_email', color: '#ef4444', inputType: 'email' },
  url: { icon: <LinkIcon width="1em" height="1em" />, labelKey: 'field_url', color: '#3b82f6', inputType: 'url' },
  select: { icon: '▾', labelKey: 'field_select', color: '#ec4899', inputType: 'text' },
  relation: { icon: '⟲', labelKey: 'field_relation', color: '#14b8a6', inputType: 'text' },
  json: { icon: '{ }', labelKey: 'field_json', color: '#64748b', inputType: 'textarea' },
  file: { icon: <Attachment width="1em" height="1em" />, labelKey: 'field_file', color: '#78716c', inputType: 'file' },
}

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
