import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext.jsx'

export default function Callback() {
  const navigate = useNavigate()
  const { login } = useAuth()

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')

    if (token) {
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`
    } else {
      delete axios.defaults.headers.common['Authorization']
    }

    // Works for both ?token= (dev/cross-origin) and HttpOnly hornero_session cookie (same-origin prod)
    axios.get(`${API_URL}/auth/me`, { withCredentials: true })
      .then(res => {
        login(token || null, res.data.data)
        navigate('/dashboard', { replace: true })
      })
      .catch(() => {
        navigate('/login', { replace: true })
      })
  }, [navigate, login])

  return (
    <div className="flex-center" style={{ height: '100vh' }}>
      <div className="loading-spinner" />
    </div>
  )
}
