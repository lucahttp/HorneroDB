import { motion, AnimatePresence } from 'framer-motion'
import { useGlobalError } from '../context/ErrorContext'
import { WarningTriangle } from 'iconoir-react'
import { useTranslation } from 'react-i18next'

export function ErrorModal() {
  const { t } = useTranslation()
  const { error, isModalOpen, hideError } = useGlobalError()

  // Helper to safely stringify error details
  const getErrorDetails = () => {
    if (!error) return ''

    const details = {
      message: error.message,
      code: error.code,
      status: error.response?.status,
      statusText: error.response?.statusText,
      url: error.config?.url,
      method: error.config?.method,
      data: error.response?.data,
      stack: error.stack
    }

    return JSON.stringify(details, null, 2)
  }

  return (
    <AnimatePresence>
      {(isModalOpen && error) && (
        <motion.div
          className="modal-overlay"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={hideError}
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0, 0, 0, 0.6)',
            backdropFilter: 'blur(4px)',
            zIndex: 100,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            padding: '1rem'
          }}
        >
          <motion.div
            className="modal"
            initial={{ scale: 0.95, opacity: 0, y: 20 }}
            animate={{ scale: 1, opacity: 1, y: 0 }}
            exit={{ scale: 0.95, opacity: 0, y: 20 }}
            onClick={e => e.stopPropagation()}
            style={{
              background: 'var(--bg-elevated)',
              border: '2px solid var(--danger)',
              borderRadius: 'var(--radius-lg)',
              boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
              width: '100%',
              maxWidth: '42rem',
              maxHeight: '90vh',
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column'
            }}
          >
            {/* Header */}
            <div style={{
              padding: '1.25rem 1.5rem',
              borderBottom: '1px solid var(--border-light)',
              background: '#FEF2F2', // Light red background
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between'
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <div style={{
                  width: '2.5rem',
                  height: '2.5rem',
                  borderRadius: '50%',
                  background: '#FEE2E2',
                  color: '#DC2626',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '1.25rem',
                  flexShrink: 0
                }}>
                  <WarningTriangle width="1.5rem" height="1.5rem" style={{ pointerEvents: 'none' }} />
                </div>
                <div>
                  <h3 style={{
                    fontSize: '1.125rem',
                    fontWeight: 700,
                    color: '#991B1B',
                    margin: 0,
                    lineHeight: 1.2
                  }}>
                    {t('system_error')}
                  </h3>
                  <p style={{
                    fontSize: '0.875rem',
                    color: '#B91C1C',
                    margin: 0,
                    marginTop: '0.125rem'
                  }}>
                    {error.response?.status ? `Error ${error.response.status}` : t('unknown_error')}
                  </p>
                </div>
              </div>
              <button
                onClick={hideError}
                style={{
                  background: 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  padding: '0.5rem',
                  borderRadius: '0.375rem',
                  color: '#991B1B'
                }}
              >
                ✕
              </button>
            </div>

            {/* Body */}
            <div style={{ padding: '1.5rem', overflowY: 'auto', flex: 1 }}>
              <p style={{
                fontSize: '1rem',
                marginBottom: '1.5rem',
                color: 'var(--text)'
              }}>
                {error.message || t('unexpected_error')}
              </p>

              <div style={{ marginBottom: '1.5rem' }}>
                <label style={{
                  display: 'block',
                  fontSize: '0.75rem',
                  fontWeight: 700,
                  textTransform: 'uppercase',
                  color: 'var(--text-secondary)',
                  marginBottom: '0.5rem'
                }}>
                  {t('technical_details')}
                </label>
                <div style={{ position: 'relative' }}>
                  <pre style={{
                    background: '#1e1e1e', // Dark theme for code
                    color: '#d4d4d4',
                    padding: '1rem',
                    borderRadius: '0.5rem',
                    fontSize: '0.8125rem',
                    overflowX: 'auto',
                    fontFamily: 'var(--font-mono)',
                    margin: 0,
                    maxHeight: '300px'
                  }}>
                    {getErrorDetails()}
                  </pre>
                </div>
              </div>

              {error.config?.url && (
                <div style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
                  <span style={{ fontWeight: 600 }}>{t('endpoint')}:</span> {error.config.method?.toUpperCase()} {error.config.url}
                </div>
              )}
            </div>

            {/* Footer */}
            <div style={{
              padding: '1rem 1.5rem',
              borderTop: '1px solid var(--border-light)',
              background: 'var(--bg-surface)',
              display: 'flex',
              justifyContent: 'flex-end',
              gap: '0.75rem'
            }}>
              <button
                className="btn btn-secondary"
                onClick={() => {
                  navigator.clipboard.writeText(getErrorDetails())
                  // TODO: Show a small "Copied" tooltip
                }}
              >
                {t('copy_error')}
              </button>
              <button
                className="btn btn-danger"
                onClick={hideError}
              >
                {t('close')}
              </button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
