
import { useEffect, useRef } from 'react'
import axios from 'axios'
import { useGlobalError } from '../context/ErrorContext'
import { notifyError } from './Toast'

export function AxiosInterceptor() {
  const { showError } = useGlobalError()
  
  useEffect(() => {
    // Request interceptor to add Authorization header
    const requestInterceptor = axios.interceptors.request.use(
      config => {
        const token = localStorage.getItem('hornero_token')
        if (token) {
          config.headers.Authorization = `Bearer ${token}`
        }
        return config
      },
      error => Promise.reject(error)
    )

    // Response interceptor for error handling
    const responseInterceptor = axios.interceptors.response.use(
      response => response,
      error => {
        // Ignore cancelled requests
        if (axios.isCancel(error)) {
          return Promise.reject(error)
        }

        // Don't show modal for 401 (handled by App.jsx or Login)
        // unless it's a specific auth call that failed
        if (error.response?.status === 401 && !error.config.url.includes('/auth/me')) {
           return Promise.reject(error)
        }

        console.error('Global API Error:', error)

        const message = error.response?.data?.error?.message 
          || error.response?.data?.message 
          || error.message 
          || 'Ha ocurrido un error inesperado'

        // Show toast with action to open modal
        notifyError(message, {
          title: 'Ver detalles',
          onClick: () => {
            console.log('Opening error modal for:', error);
            showError(error);
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
