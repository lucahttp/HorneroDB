
import { createContext, useContext, useState, useCallback, useEffect } from 'react'

const ErrorContext = createContext(null)

export function ErrorProvider({ children }) {
  const [error, setError] = useState(null)
  const [isModalOpen, setIsModalOpen] = useState(false)

  const showError = useCallback((err) => {
    setError(err)
    setIsModalOpen(true)
  }, [])


  const hideError = useCallback(() => {
    setIsModalOpen(false)
    // Optional: clear error after animation or delay, 
    // but clearing immediately allows the modal to close cleanly primarily
    // We keep the error for a moment if we want to support re-opening, 
    // but simplicity suggests clearing it or just hiding the modal.
    setTimeout(() => setError(null), 300) 
  }, [])

  return (
    <ErrorContext.Provider value={{ error, isModalOpen, showError, hideError }}>
      {children}
    </ErrorContext.Provider>
  )
}

export function useGlobalError() {
  const context = useContext(ErrorContext)
  if (!context) {
    throw new Error('useGlobalError must be used within an ErrorProvider')
  }
  return context
}
