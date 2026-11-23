import { useEffect, useRef } from 'react'
import axios from 'axios'
import { useGlobalError } from '../context/ErrorContext'
import { notifyError } from './toast.js'
import i18n from '../i18n/index.js'
import { AUTH_TOKEN_KEY, AUTH_EXPIRED_EVENT } from '../constants'

/**
 * Endpoints that must NOT trigger the global "session expired" flow:
 *  - `/auth/me`      : the session probe itself; a 401 there just means "not logged in".
 *  - `/auth/oidc/*`  : the OIDC login/callback flow.
 *  - `/auth/login`   : credentials were rejected, the session was not expired.
 *  - `/auth/logout`  : the session is being closed on purpose.
 */
const AUTH_EXEMPT_PATHS = ['/auth/me', '/auth/oidc/', '/auth/login', '/auth/logout']

const isAuthExempt = (url) => {
  if (!url) return false
  return AUTH_EXEMPT_PATHS.some((path) => url.includes(path))
}

export function AxiosInterceptor() {
  const { showError } = useGlobalError()
  const handledRef = useRef(false)

  useEffect(() => {
    // Request interceptor to add Authorization header
    const requestInterceptor = axios.interceptors.request.use(
      config => {
        // The session cookie has to travel with every request.
        config.withCredentials = true
        const token = localStorage.getItem(AUTH_TOKEN_KEY)
        if (token) {
          config.headers.Authorization = `Bearer ${token}`
          // A new session means the previous "session expired" was handled.
          handledRef.current = false
        }
        return config
      },
      error => Promise.reject(error)
    )

    /** Drop the local session and send the user back to the login screen. */
    const handleSessionExpired = (error) => {
      if (handledRef.current) return
      handledRef.current = true

      localStorage.removeItem(AUTH_TOKEN_KEY)
      delete axios.defaults.headers.common['Authorization']

      // Let AuthContext clear its state (user, token) and unmount protected views.
      window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT))

      notifyError(i18n.t('session_expired'), {
        title: i18n.t('view_details', { defaultValue: 'Ver detalles' }),
        onClick: () => showError(error),
      })

      if (!window.location.pathname.startsWith('/login')) {
        window.location.assign('/login')
      }
    }

    // Response interceptor for error handling
    const responseInterceptor = axios.interceptors.response.use(
      response => response,
      error => {
        // Ignore cancelled requests
        if (axios.isCancel(error)) {
          return Promise.reject(error)
        }

        const url = error.config?.url || ''

        if (error.response?.status === 401) {
          // A 401 on a protected endpoint means the session expired: clean it
          // up and send the user back to the login screen.
          if (!isAuthExempt(url)) {
            handleSessionExpired(error)
          }
          // A 401 on an auth-exempt endpoint (the session probe, a rejected
          // login) is an expected answer, not an error worth a toast.
          return Promise.reject(error)
        }

        console.error('Global API Error:', error)

        const message = error.response?.data?.error?.message
          || error.response?.data?.message
          || error.message
          || i18n.t('unexpected_error', { defaultValue: 'Ha ocurrido un error inesperado' })

        // Show toast with action to open modal
        notifyError(message, {
          title: i18n.t('view_details', { defaultValue: 'Ver detalles' }),
          onClick: () => {
            console.log('Opening error modal for:', error)
            showError(error)
          }
        })

        return Promise.reject(error)
      }
    )

    return () => {
      axios.interceptors.request.eject(requestInterceptor)
      axios.interceptors.response.eject(responseInterceptor)
    }
  }, [showError])

  return null
}
