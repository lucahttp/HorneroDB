import axios from 'axios'
import { openobserveRum } from '@openobserve/browser-rum'
import { openobserveLogs } from '@openobserve/browser-logs'
import { API_URL } from '../constants'

let interceptorInstalled = false
let initStarted = false
let rumActive = false
let logsActive = false
let pendingUser = null

/**
 * Generates a lowercase hex string of `byteLength` bytes, ensuring it is never
 * all-zero as required by the W3C Trace Context specification.
 */
function randomNonZeroHex(byteLength) {
  const bytes = new Uint8Array(byteLength)
  crypto.getRandomValues(bytes)
  if (bytes.every((byte) => byte === 0)) {
    bytes[bytes.length - 1] = 1
  }
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

function generateTraceparent() {
  if (typeof crypto === 'undefined' || typeof crypto.getRandomValues !== 'function') {
    return null
  }
  return `00-${randomNonZeroHex(16)}-${randomNonZeroHex(8)}-01`
}

function hasTraceparentHeader(headers) {
  if (!headers) return false
  if (typeof headers.has === 'function') {
    return headers.has('traceparent')
  }
  return Object.keys(headers).some((key) => key.toLowerCase() === 'traceparent')
}

function setTraceparentHeader(headers, value) {
  if (!headers) {
    return { traceparent: value }
  }
  if (typeof headers.set === 'function') {
    headers.set('traceparent', value)
    return headers
  }
  headers.traceparent = value
  return headers
}

/**
 * Installs an idempotent fallback Axios request interceptor that injects a
 * W3C `traceparent` header whenever RUM tracing is inactive and the caller
 * has not already supplied one.
 */
function installFallbackTraceInterceptor() {
  if (interceptorInstalled) return
  interceptorInstalled = true

  axios.interceptors.request.use(
    (config) => {
      // Yield to OpenObserve RUM's XHR/Fetch tracer once RUM is active so we
      // never append duplicate traceparent headers on XMLHttpRequest.
      if (rumActive || hasTraceparentHeader(config.headers)) {
        return config
      }
      const traceparent = generateTraceparent()
      if (traceparent) {
        config.headers = setTraceparentHeader(config.headers, traceparent)
      }
      return config
    },
    (error) => Promise.reject(error),
  )
}

/**
 * Normalizes a configured OpenObserve `site` value (with or without scheme or
 * trailing slash) into the bare `host[:port][/path]` + `insecureHTTP` flag
 * expected by `@openobserve/browser-core`'s endpoint builder.
 */
function normalizeSiteConfig(rawSite) {
  const trimmed = String(rawSite || '').trim().replace(/\/+$/, '')
  if (/^http:\/\//i.test(trimmed)) {
    return {
      site: trimmed.slice('http://'.length).replace(/\/+$/, ''),
      insecureHTTP: true,
    }
  }
  if (/^https:\/\//i.test(trimmed)) {
    return {
      site: trimmed.slice('https://'.length).replace(/\/+$/, ''),
      insecureHTTP: false,
    }
  }
  // Bare host[:port] without scheme: treat localhost / loopback or OpenObserve's
  // default plain-HTTP port (5080) as HTTP unless explicitly TLS-prefixed.
  const isPlainLocalOrDefaultPort =
    /^(localhost|127\.0\.0\.1|\[::1\])(:\d+)?(?:\/|$)/i.test(trimmed) ||
    /:5080(?:\/|$)/.test(trimmed)
  return {
    site: trimmed,
    insecureHTTP: isPlainLocalOrDefaultPort,
  }
}

function buildAllowedTracingUrls() {
  const urls = [
    { match: window.location.origin, propagatorTypes: ['tracecontext'] },
    { match: /^\/api\//, propagatorTypes: ['tracecontext'] },
  ]
  if (/^https?:\/\//i.test(API_URL)) {
    try {
      const apiOrigin = new URL(API_URL).origin
      if (apiOrigin && apiOrigin !== window.location.origin) {
        urls.push({ match: apiOrigin, propagatorTypes: ['tracecontext'] })
      }
    } catch {
      // Ignore invalid API_URL
    }
  }
  return urls
}

function sanitizeUser(user) {
  if (!user || !user.id) return null
  const role = user.role || user.role_name || (user.is_instance_admin ? 'instance_admin' : undefined)
  return role
    ? { id: String(user.id), role: String(role) }
    : { id: String(user.id) }
}

function applyUserContext(userContext) {
  if (!userContext) return
  if (rumActive) {
    try {
      openobserveRum.setUser(userContext)
    } catch {
      // Never let telemetry failures break auth state
    }
  }
  if (logsActive) {
    try {
      openobserveLogs.setUser(userContext)
    } catch {
      // Never let telemetry failures break auth state
    }
  }
}

/**
 * Sets non-PII user context (`id` and `role`) on OpenObserve RUM and Logs.
 * Buffers the user if called before async telemetry initialization completes.
 */
export function setRumUser(user) {
  const sanitized = sanitizeUser(user)
  if (!sanitized) return
  pendingUser = sanitized
  applyUserContext(sanitized)
}

/**
 * Clears user context from OpenObserve RUM and Logs on logout or expiration.
 */
export function clearRumUser() {
  pendingUser = null
  if (rumActive) {
    try {
      openobserveRum.clearUser()
    } catch {
      // Ignore telemetry errors on logout
    }
  }
  if (logsActive) {
    try {
      openobserveLogs.clearUser()
    } catch {
      // Ignore telemetry errors on logout
    }
  }
}

/**
 * Initializes frontend W3C traceparent propagation and, when enabled by the
 * backend `/telemetry/config` endpoint, boots OpenObserve Browser RUM & Logs.
 * Fire-and-forget: errors are swallowed so telemetry never blocks the UI.
 */
export async function initTelemetry() {
  installFallbackTraceInterceptor()

  if (initStarted) return
  initStarted = true

  try {
    // Use fetch rather than axios so a 404/network failure on telemetry config
    // never triggers the global AxiosInterceptor error toast.
    const response = await fetch(`${API_URL}/telemetry/config`, {
      credentials: 'same-origin',
    })
    if (!response.ok) return

    const payload = await response.json()
    const config = payload?.data
    if (!config || config.enabled !== true || !config.site || !config.client_token) {
      return
    }

    const { site, insecureHTTP } = normalizeSiteConfig(config.site)
    if (!site) return

    const organizationIdentifier = config.organization || 'default'
    const service = config.service || 'hornerodb-ui'
    const env = config.env || 'production'
    const version = config.version || '1.0.0'
    const sessionReplayEnabled = config.session_replay === true

    openobserveRum.init({
      applicationId: config.application_id || 'hornerodb-ui',
      clientToken: config.client_token,
      site,
      insecureHTTP,
      organizationIdentifier,
      service,
      env,
      version,
      sessionSampleRate: 100,
      sessionReplaySampleRate: sessionReplayEnabled ? 100 : 0,
      trackResources: true,
      trackLongTasks: true,
      trackUserInteractions: true,
      allowedTracingUrls: buildAllowedTracingUrls(),
      silentMultipleInit: true,
    })
    rumActive = true

    if (sessionReplayEnabled) {
      try {
        openobserveRum.startSessionReplayRecording()
      } catch {
        // Ignore session replay startup failures
      }
    }

    try {
      openobserveLogs.init({
        clientToken: config.client_token,
        site,
        insecureHTTP,
        organizationIdentifier,
        service,
        env,
        version,
        forwardErrorsToLogs: true,
        forwardConsoleLogs: ['error'],
        silentMultipleInit: true,
      })
      logsActive = true
    } catch {
      // Logs init failure must not affect RUM or the application
    }

    if (pendingUser) {
      applyUserContext(pendingUser)
    }
  } catch {
    // Telemetry is strictly best-effort and must never block or crash the UI.
  }
}
