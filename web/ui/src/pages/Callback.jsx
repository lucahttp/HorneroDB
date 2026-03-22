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
      
      axios.get(`${API_URL}/auth/me`)
        .then(res => {
          login(token, res.data.data)
          navigate('/dashboard')
        })
        .catch(() => {
          navigate('/')
        })
    } else {
      navigate('/')
    }
  }, [navigate, login])

  return (
    <div className="flex-center" style={{ height: '100vh' }}>
      <div className="loading-spinner" />
    </div>
  )
}
