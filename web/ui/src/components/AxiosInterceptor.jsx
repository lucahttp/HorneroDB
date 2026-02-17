
import { useEffect, useRef } from 'react'
import axios from 'axios'
import { useGlobalError } from '../context/ErrorContext'
import { notifyError } from './Toast'

export function AxiosInterceptor() {
  const { showError } = useGlobalError()
  const interceptorId = useRef(null)

  useEffect(() => {
    interceptorId.current = axios.interceptors.response.use(
      response => response,
      error => {
        // Ignore cancelled requests
        if (axios.isCancel(error)) {
          return Promise.reject(error)
        }

        console.error('Global API Error:', error)

        const message = error.response?.data?.message 
          || error.message 
          || 'Ha ocurrido un error inesperado'

        // Show toast with action to open modal
        notifyError(message, {
          title: 'Ver detalles',
          onClick: () => showError(error)
        })
        
        return Promise.reject(error)
      }
    )

    return () => {
      if (interceptorId.current !== null) {
        axios.interceptors.response.eject(interceptorId.current)
      }
    }
  }, [showError])

  return null
}
