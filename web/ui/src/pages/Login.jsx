import { useTranslation } from 'react-i18next'
import { motion } from 'framer-motion'
import { Lock, EmojiSingLeftNote } from 'iconoir-react'
import { API_URL } from '../constants'
import horneroLogo from '../assets/hornero solo.png'

export default function Login() {
  const { t } = useTranslation()
  const handleLogin = () => {
    window.location.href = `${API_URL}/auth/oidc/login?redirect=${encodeURIComponent(window.location.origin + '/callback')}`
  }

  return (
    <div className="login-container">
      {/* Left panel — bold yellow brand */}
      <div className="login-left">
        <motion.div
          style={{ textAlign: 'center' }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
        >
          <img src={horneroLogo} alt="HorneroDB" style={{ width: '180px', height: '180px', objectFit: 'contain', marginBottom: '1rem', filter: 'drop-shadow(0 4px 6px rgba(0,0,0,0.1))' }} />
          <h1 style={{ fontSize: '3rem', fontWeight: 900, color: '#FFFFFF', letterSpacing: '-0.03em', marginBottom: '0.5rem' }}>
            HorneroDB
          </h1>
          <p style={{ fontSize: '1.125rem', color: 'rgba(255,255,255,0.7)', fontWeight: 500 }}>
            {t('your_personal_db')}
          </p>
        </motion.div>
      </div>

      {/* Right panel — login form */}
      <div className="login-right">
        <motion.div
          style={{ width: '100%', maxWidth: '400px' }}
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2 }}
        >
          <h2 style={{ fontSize: '2rem', fontWeight: 800, marginBottom: '0.5rem', letterSpacing: '-0.02em', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            {t('welcome')} <EmojiSingLeftNote width="1.25em" height="1.25em" />
          </h2>
          <p style={{ color: 'var(--text-secondary)', marginBottom: '2rem', fontSize: '1rem' }}>
            {t('login_subtitle')}
          </p>

          <button
            onClick={handleLogin}
            className="btn btn-primary btn-lg"
            style={{ width: '100%', gap: '0.75rem', fontSize: '1rem', padding: '0.875rem 1.5rem' }}
          >
            <Lock width="1.25rem" height="1.25rem" />
            {t('login_button')}
          </button>

          <p style={{ color: 'var(--text-muted)', fontSize: '0.8125rem', textAlign: 'center', marginTop: '2rem', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.375rem' }}>
            <Lock width="0.875em" height="0.875em" /> {t('secure_access')}
          </p>
        </motion.div>
      </div>
    </div>
  )
}
