/**
 * Client-side mirror of the backend CLS permission resolution.
 *
 * The rules implemented here are a 1:1 port of
 * `internal/services/permission/service.go` so the UI can tell the user what
 * the current role is allowed to do before the API answers with a 403.
 *
 * Precedence (per operation):
 *   1. the rule of the table itself wins over the `*` wildcard rule;
 *   2. a table rule that inherits (or is empty) for that operation falls back
 *      to the wildcard rule;
 *   3. when neither rule decides, access is denied.
 */

/** Reserved key holding the workspace wide (system) permissions of a role. */
export const SYSTEM_ACCESS_KEY = '__system__'

/** Reserved key holding the default rule applied to every table. */
export const WILDCARD_KEY = '*'

/** Table level access levels. */
export const TABLE_ACCESS_LEVELS = ['none', 'own', 'all', 'inherit']

/** System level access levels, ordered by increasing power. */
export const SYSTEM_ACCESS_LEVELS = ['none', 'view', 'manage', 'all']

/** System areas that can be granted to a role. */
export const SYSTEM_AREAS = ['webhooks', 'roles', 'api_keys', 'tables', 'settings', 'mcp']

const asMap = (value) => (
  value && typeof value === 'object' && !Array.isArray(value) ? value : null
)

/**
 * Normalize a permission document. The API returns an object, but a raw JSON
 * string is accepted as well.
 */
const parsePermissions = (raw) => {
  if (!raw) return {}
  if (typeof raw === 'object') return raw
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

const levelRank = (level) => {
  const index = SYSTEM_ACCESS_LEVELS.indexOf(level)
  return index < 0 ? 0 : index
}

/**
 * System level permissions of a role.
 * The root `admin` role always keeps full system access.
 */
export function getSystemPermissions(permissions, roleName) {
  if (roleName === 'admin') {
    return SYSTEM_AREAS.reduce((acc, area) => ({ ...acc, [area]: 'all' }), {})
  }

  const system = asMap(parsePermissions(permissions)[SYSTEM_ACCESS_KEY])
  if (!system) {
    return SYSTEM_AREAS.reduce((acc, area) => ({ ...acc, [area]: 'none' }), {})
  }

  return SYSTEM_AREAS.reduce((acc, area) => {
    const raw = system[area]
    const level = typeof raw === 'string' && SYSTEM_ACCESS_LEVELS.includes(raw) ? raw : 'none'
    return { ...acc, [area]: level }
  }, {})
}

/** True when the role holds at least `minLevel` on a system area. */
export function hasSystemAccess(systemPermissions, area, minLevel = 'manage') {
  return levelRank(systemPermissions?.[area]) >= levelRank(minLevel)
}

/**
 * Decides the access level of a single operation inside a rule.
 *
 * @returns {{level: string, decided: boolean}} `decided` is false when the
 * rule does not decide (inherit / empty) and the wildcard must be used.
 */
function resolveOperationLevel(rule, operation) {
  if (!rule) return { level: 'none', decided: false }

  const raw = rule[operation]
  if (raw === undefined || raw === null) {
    // The rule exists but says nothing about this operation: no access.
    return { level: 'none', decided: true }
  }
  if (typeof raw === 'string' && raw.trim() === '') {
    // An empty value behaves like an absent entry: fall back to the wildcard.
    return { level: 'none', decided: false }
  }
  if (raw === 'inherit') {
    return { level: 'none', decided: false }
  }
  if (raw === 'none' || raw === 'own' || raw === 'all') {
    return { level: raw, decided: true }
  }
  return { level: 'none', decided: true }
}

/** Effective access level of a role for an operation on a table. */
export function resolveTableAccess(permissions, tableSlug, operation) {
  const permMap = parsePermissions(permissions)

  const tableRule = asMap(permMap[tableSlug])
  if (tableRule) {
    const resolved = resolveOperationLevel(tableRule, operation)
    if (resolved.decided) return resolved.level
  }

  const wildcardRule = asMap(permMap[WILDCARD_KEY])
  if (wildcardRule) {
    const resolved = resolveOperationLevel(wildcardRule, operation)
    if (resolved.decided) return resolved.level
  }

  return 'none'
}

/**
 * Reads the column restriction of an operation out of a rule.
 *
 * @returns {string[]|undefined} `undefined` means "no restriction" (every
 * column is allowed), an empty array means "no column is allowed".
 */
function readColumns(columnsObject, operation) {
  const obj = asMap(columnsObject)
  if (!obj) return undefined

  const raw = obj[operation]
  if (raw === undefined || raw === null) return undefined
  if (!Array.isArray(raw)) return undefined

  return raw.map(value => String(value))
}

/**
 * Effective column restriction of a role for an operation on a table.
 *
 * @returns {string[]|undefined} `undefined` = all columns, `[]` = no column.
 */
export function resolveColumnPermissions(permissions, tableSlug, operation) {
  const permMap = parsePermissions(permissions)

  const tableRule = asMap(permMap[tableSlug])
  if (tableRule) {
    const columns = readColumns(tableRule.columns, operation)
    if (columns !== undefined) return columns
  }

  const wildcardRule = asMap(permMap[WILDCARD_KEY])
  if (wildcardRule) {
    const columns = readColumns(wildcardRule.columns, operation)
    if (columns !== undefined) return columns
  }

  return undefined
}

/** True when the column may be read (no restriction => readable). */
export function isColumnReadable(readableColumns, slug) {
  if (readableColumns === undefined) return true
  return readableColumns.includes(slug)
}

/** True when the column may be written (no restriction => writable). */
export function isColumnWritable(writableColumns, slug) {
  if (writableColumns === undefined) return true
  return writableColumns.includes(slug)
}

/** True when the column may be part of a create payload (no restriction => yes). */
export function isColumnCreatable(creatableColumns, slug) {
  if (creatableColumns === undefined) return true
  return creatableColumns.includes(slug)
}

/**
 * Everything the grid needs to know about the current role for one table.
 */
export function getEffectivePermissions(permissions, roleName, tableSlug) {
  const permMap = parsePermissions(permissions)

  return {
    system: getSystemPermissions(permMap, roleName),
    create: resolveTableAccess(permMap, tableSlug, 'create'),
    read: resolveTableAccess(permMap, tableSlug, 'read'),
    update: resolveTableAccess(permMap, tableSlug, 'update'),
    delete: resolveTableAccess(permMap, tableSlug, 'delete'),
    readableColumns: resolveColumnPermissions(permMap, tableSlug, 'read'),
    writableColumns: resolveColumnPermissions(permMap, tableSlug, 'update'),
    creatableColumns: resolveColumnPermissions(permMap, tableSlug, 'create'),
  }
}
