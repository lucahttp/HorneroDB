import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { SunLight, MoonSat as MoonSaturn } from 'iconoir-react'


export function ThemeToggle() {
  const [theme, setTheme] = useState(() => {
    if (typeof window !== 'undefined') {
      return localStorage.getItem('theme') ||
        (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
    }
    return 'light'
  })

  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
    localStorage.setItem('theme', theme)
  }, [theme])

  const toggle = () => setTheme(theme === 'light' ? 'dark' : 'light')

  return (
    <motion.button
      className="btn btn-ghost btn-sm"
      onClick={toggle}
      whileTap={{ scale: 0.95 }}
      title={theme === 'light' ? 'Modo oscuro' : 'Modo claro'}
      style={{ fontSize: '1.1rem', padding: '0.4rem 0.6rem' }}
    >
      {theme === 'light' ? <MoonSaturn width="1rem" height="1rem" /> : <SunLight width="1rem" height="1rem" />}
    </motion.button>
  )
}

export function Button({
  children,
  variant = 'primary',
  size = 'md',
  disabled = false,
  loading = false,
  onClick,
  type = 'button',
  className = '',
  ...props
}) {
  const sizeClasses = {
    sm: 'btn-sm',
    md: 'btn-md',
    lg: 'btn-lg',
  }

  const variantClasses = {
    primary: 'btn-primary',
    secondary: 'btn-secondary',
    ghost: 'btn-ghost',
    danger: 'btn-danger',
  }

  return (
    <motion.button
      type={type}
      className={`btn ${variantClasses[variant] || ''} ${sizeClasses[size] || ''} ${className}`}
      disabled={disabled || loading}
      onClick={onClick}
      whileTap={{ scale: disabled ? 1 : 0.97 }}
      style={{
        opacity: disabled ? 0.5 : 1,
        cursor: disabled ? 'not-allowed' : 'pointer',
      }}
      {...props}
    >
      {loading ? (
        <div style={{
          width: '1rem',
          height: '1rem',
          border: '2px solid rgba(0,0,0,0.2)',
          borderTopColor: 'currentColor',
          borderRadius: '50%',
          animation: 'spin 0.6s linear infinite',
        }} />
      ) : children}
    </motion.button>
  )
}

export function Badge({ children, variant = 'primary', className = '' }) {
  const variantClasses = {
    primary: 'badge-primary',
    success: 'badge-success',
    warning: 'badge-warning',
    error: 'badge-error',
    gray: 'badge-gray',
  }

  return (
    <span className={`badge ${variantClasses[variant] || ''} ${className}`}>
      {children}
    </span>
  )
}

export function Modal({ isOpen, onClose, title, children, footer }) {
  if (!isOpen) return null

  return (
    <motion.div
      className="modal-overlay"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className="modal"
        initial={{ scale: 0.95, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        exit={{ scale: 0.95, opacity: 0 }}
        onClick={e => e.stopPropagation()}
      >
        <div className="modal-header">
          <h3 className="modal-title">{title}</h3>
          <button
            className="btn btn-ghost btn-sm"
            onClick={onClose}
            style={{ borderRadius: '8px' }}
          >
            <svg style={{ width: '1.25rem', height: '1.25rem' }} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div className="modal-body">
          {children}
        </div>
        {footer && (
          <div className="modal-footer">
            {footer}
          </div>
        )}
      </motion.div>
    </motion.div>
  )
}

export function LoadingSpinner() {
  return (
    <div className="loading-spinner" />
  )
}

export function EmptyState({ icon, title, description, action }) {
  return (
    <div className="empty-state">
      {icon && <div className="empty-icon">{icon}</div>}
      {title && <h3 style={{ fontSize: '1.125rem', fontWeight: 700, marginBottom: '0.5rem' }}>{title}</h3>}
      {description && <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem', maxWidth: '24rem' }}>{description}</p>}
      {action}
    </div>
  )
}

// Re-export tour components (currently disabled)
export { OnboardingTour, resetTour } from './OnboardingTour'
export { DashboardTour } from './DashboardTour'
export { TableEditTour } from './TableEditTour'
