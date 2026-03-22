import { createContext, useContext, useState, useEffect } from 'react'
import axios from 'axios'
import { API_URL, AUTH_TOKEN_KEY } from '../constants'

const AuthContext = createContext(null)

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
  const [loading, setLoading] = useState(!!token)

  useEffect(() => {
    if (token) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`
      axios.get(`${API_URL}/auth/me`)
        .then(res => {
          setUser(res.data.data)
          setLoading(false)
        })
        .catch(() => {
          localStorage.removeItem(AUTH_TOKEN_KEY)
          setToken(null)
          setLoading(false)
        })
    } else {
      delete axios.defaults.headers.common['Authorization']
      setLoading(false)
    }
  }, [token])

  const login = (newToken, userData) => {
    localStorage.setItem(AUTH_TOKEN_KEY, newToken)
    setToken(newToken)
    if (userData) {
      setUser(userData)
    }
  }

  const logout = () => {
    localStorage.removeItem(AUTH_TOKEN_KEY)
    setToken(null)
    setUser(null)
  }

  const value = {
    token,
    user,
    loading,
    login,
    logout
  }

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}
