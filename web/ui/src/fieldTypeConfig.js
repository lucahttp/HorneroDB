/**
 * Centralized field type configuration — PocketBase style.
 * Each type has: icon, label (Spanish), color, and HTML input type.
 */

export const FIELD_TYPES = {
  text:      { icon: 'T',   label: 'Texto',         color: '#6366f1', inputType: 'text' },
  long_text: { icon: '≡',   label: 'Texto largo',   color: '#6366f1', inputType: 'textarea' },
  number:    { icon: '#',   label: 'Número',        color: '#f59e0b', inputType: 'number', step: '0.01' },
  integer:   { icon: '123', label: 'Entero',        color: '#f59e0b', inputType: 'number', step: '1' },
  float:     { icon: '#.#', label: 'Decimal',       color: '#f59e0b', inputType: 'number', step: '0.01' },
  boolean:   { icon: '☑',   label: 'Booleano',      color: '#10b981', inputType: 'checkbox' },
  date:      { icon: '📅',  label: 'Fecha',          color: '#8b5cf6', inputType: 'date' },
  datetime:  { icon: '🕐',  label: 'Fecha y hora',   color: '#8b5cf6', inputType: 'datetime-local' },
  email:     { icon: '@',   label: 'Email',         color: '#ef4444', inputType: 'email' },
  url:       { icon: '🔗',  label: 'URL',            color: '#3b82f6', inputType: 'url' },
  select:    { icon: '▾',   label: 'Selección',     color: '#ec4899', inputType: 'text' },
  relation:  { icon: '⟲',   label: 'Relación',      color: '#14b8a6', inputType: 'text' },
  json:      { icon: '{ }', label: 'JSON',          color: '#64748b', inputType: 'textarea' },
  file:      { icon: '📎',  label: 'Archivo',        color: '#78716c', inputType: 'file' },
}

/** Get config for a type — falls back to text */
export function getFieldConfig(fieldType) {
  return FIELD_TYPES[fieldType] || FIELD_TYPES.text
}

/** Ordered array for UI dropdowns */
export const FIELD_TYPE_OPTIONS = Object.entries(FIELD_TYPES).map(([value, cfg]) => ({
  value,
  ...cfg,
}))
