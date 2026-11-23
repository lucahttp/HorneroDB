import { createContext, useContext, useState, useEffect, useCallback } from 'react'
import axios from 'axios'
import { API_URL, AUTH_TOKEN_KEY, AUTH_EXPIRED_EVENT } from '../constants'
import { setRumUser, clearRumUser } from '../telemetry/rum'

const AuthContext = createContext(null)

/**
 * Marker used when the session lives only in the `hornero_session` cookie.
 * The API authenticated the request, but there is no bearer token to keep in
 * `localStorage`. The marker keeps the router guards working (they only check
 * that a session exists) without ever being sent as an `Authorization` header.
 */
const COOKIE_SESSION = '__cookie_session__'

// The session cookie has to travel with every request: in a same-origin
// deployment the API lives behind the same host as the UI.
axios.defaults.withCredentials = true

export const useAuth = () => {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

export const AuthProvider = ({ children }) => {
  const [token, setToken] = useState(localStorage.getItem(AUTH_TOKEN_KEY))
  const [user, setUser] = useState(null)
  // The session is probed on startup, so the app waits for the answer even
  // when there is no token in `localStorage`.
  const [authLoading, setAuthLoading] = useState(true)

  const logout = useCallback(() => {
    // Close the session on the server first: that is what clears the
    // `hornero_session` cookie. The bearer token is attached explicitly
    // because `localStorage` is emptied right after this call. A failing
    // request must not keep the user stuck in the app, so the local state is
    // cleared anyway.
    const currentToken = localStorage.getItem(AUTH_TOKEN_KEY)
    axios.post(`${API_URL}/auth/logout`, null, currentToken
      ? { headers: { Authorization: `Bearer ${currentToken}` } }
      : undefined
    ).catch(() => {})

    localStorage.removeItem(AUTH_TOKEN_KEY)
    delete axios.defaults.headers.common['Authorization']
    clearRumUser()
    setToken(null)
    setUser(null)
    setAuthLoading(false)
  }, [])

  const login = useCallback((newToken, userData) => {
    if (newToken) {
      localStorage.setItem(AUTH_TOKEN_KEY, newToken)
      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`
      setToken(newToken)
    } else {
      localStorage.removeItem(AUTH_TOKEN_KEY)
      delete axios.defaults.headers.common['Authorization']
      setToken(COOKIE_SESSION)
    }
    if (userData) {
      setUser(userData)
      setRumUser(userData)
    }
  }, [])

  /**
   * Probes the session on startup.
   *
   * The session may live in the `hornero_session` cookie only, so the probe
   * runs even when `localStorage` has no token: a 200 answer means the user
   * is authenticated and only the cookie carries the session.
   */
  useEffect(() => {
    let cancelled = false

    const storedToken = localStorage.getItem(AUTH_TOKEN_KEY)
    if (storedToken) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${storedToken}`
    } else {
      delete axios.defaults.headers.common['Authorization']
    }

    axios.get(`${API_URL}/auth/me`)
      .then(res => {
        if (cancelled) return
        const userData = res.data.data
        setUser(userData)
        setRumUser(userData)
        // Keep the bearer token when there is one, otherwise mark the session
        // as cookie-only.
        setToken(prev => prev || COOKIE_SESSION)
      })
      .catch(() => {
        if (cancelled) return
        localStorage.removeItem(AUTH_TOKEN_KEY)
        delete axios.defaults.headers.common['Authorization']
        clearRumUser()
        setToken(null)
        setUser(null)
      })
      .finally(() => {
        if (!cancelled) setAuthLoading(false)
      })

    return () => { cancelled = true }
  }, [])

  /**
   * Global 401 handling: the Axios interceptor dispatches `AUTH_EXPIRED_EVENT`
   * when the API rejects the session, and this listener performs the logout.
   */
  useEffect(() => {
    const handleSessionExpired = () => logout()
    window.addEventListener(AUTH_EXPIRED_EVENT, handleSessionExpired)
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, handleSessionExpired)
  }, [logout])

  const refreshUser = useCallback(async (workspaceId) => {
    const config = workspaceId ? { params: { workspace_id: workspaceId } } : undefined
    const res = await axios.get(`${API_URL}/auth/me`, config)
    const userData = res.data?.data ?? null
    setUser(userData)
    if (userData) {
      setRumUser(userData)
    } else {
      clearRumUser()
    }
    setToken(prev => prev || COOKIE_SESSION)
    return userData
  }, [])

  const value = {
    token,
    user,
    // True as soon as the session probe answered with a user.
    isAuthenticated: !!user,
    // While the session is being probed there is nothing to render yet.
    loading: authLoading,
    login,
    logout,
    refreshUser
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}
