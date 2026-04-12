import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button } from '../components/index.jsx'
import { notify } from '../components/Toast.jsx'
import { Settings, CheckCircle, Shield } from 'iconoir-react'

export default function InitialSetup() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  
  const [loading, setLoading] = useState(true)
  const [needsSetup, setNeedsSetup] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  
  // Form state
  const [instanceName, setInstanceName] = useState('')
  const [contactEmail, setContactEmail] = useState(user?.email || '')
  const [pocketIDEnabled, setPocketIDEnabled] = useState(false)
  const [pocketIDURL, setPocketIDURL] = useState('')
  const [defaultRateLimit, setDefaultRateLimit] = useState(60)
  const [maxWorkspaces, setMaxWorkspaces] = useState(10)

  useEffect(() => {
    checkSetupStatus()
  }, [])

  const checkSetupStatus = async () => {
    try {
      const res = await axios.get(`${API_URL}/setup/status`)
      const { needs_setup, is_admin } = res.data.data
      
      if (!needs_setup) {
        // Setup ya completado
        if (is_admin) {
          navigate('/dashboard')
        } else {
          // No es admin y no necesita setup - redirigir igual
          navigate('/dashboard')
        }
        return
      }
      
      setNeedsSetup(true)
    } catch (err) {
      console.error('Error checking setup status:', err)
      notify(t('error_checking_setup'), 'error')
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    
    if (!instanceName.trim()) {
      notify(t('instance_name_required'), 'error')
      return
    }

    // Validar rate limit
    const rateLimitValue = parseInt(defaultRateLimit)
    if (isNaN(rateLimitValue) || rateLimitValue < 10 || rateLimitValue > 10000) {
      notify(t('invalid_rate_limit'), 'error')
      return
    }

    // Validar max workspaces
    const maxWsValue = parseInt(maxWorkspaces)
    if (isNaN(maxWsValue) || maxWsValue < 1 || maxWsValue > 100) {
      notify(t('invalid_max_workspaces'), 'error')
      return
    }

    // Validar PocketID URL si está habilitado
    if (pocketIDEnabled && !pocketIDURL.trim()) {
      notify(t('pocketid_url_required'), 'error')
      return
    }

    setIsSubmitting(true)
    
    try {
      await axios.post(`${API_URL}/setup/complete`, {
        instance_name: instanceName,
        contact_email: contactEmail || user?.email,
        pocketid_enabled: pocketIDEnabled,
        pocketid_url: pocketIDURL,
        default_rate_limit: rateLimitValue,
        max_workspaces: maxWsValue
      })
      
      notify(t('setup_completed'), 'success')
      
      // Esperar un momento y redirigir
      setTimeout(() => {
        navigate('/dashboard')
      }, 1500)
    } catch (err) {
      console.error('Error completing setup:', err)
      const msg = err.response?.data?.error?.message || t('error_completing_setup')
      notify(msg, 'error')
    } finally {
      setIsSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div style={{ 
        minHeight: '100vh', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center', 
        background: 'var(--bg)' 
      }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  if (!needsSetup) {
    return null // Ya redirigió
  }

  return (
    <div style={{ 
      minHeight: '100vh', 
      background: 'var(--bg)', 
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: '2rem'
    }}>
      <div style={{ 
        maxWidth: '500px', 
        width: '100%',
        background: 'var(--surface)',
        borderRadius: '1rem',
        padding: '2.5rem',
        boxShadow: 'var(--shadow-lg)'
      }}>
        <div style={{ textAlign: 'center', marginBottom: '2rem' }}>
          <div style={{ 
            width: '64px', 
            height: '64px', 
            background: 'var(--primary)', 
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            margin: '0 auto 1.5rem'
          }}>
            <Settings width="32" height="32" color="white" />
          </div>
          
          <h1 style={{ fontSize: '1.75rem', fontWeight: 800, marginBottom: '0.5rem' }}>
            {t('initial_setup_title')}
          </h1>
          
          <p style={{ color: 'var(--text-secondary)', fontSize: '1rem' }}>
            {t('initial_setup_subtitle')}
          </p>
        </div>

        <div style={{ 
          background: 'var(--bg)', 
          borderRadius: '0.75rem', 
          padding: '1rem',
          marginBottom: '2rem',
          display: 'flex',
          alignItems: 'center',
          gap: '0.75rem'
        }}>
          <Shield width="24" height="24" color="var(--success)" />
          <div>
            <div style={{ fontWeight: 600, fontSize: '0.875rem' }}>
              {t('you_will_become_admin')}
            </div>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
              {user?.email}
            </div>
          </div>
        </div>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ 
              display: 'block', 
              marginBottom: '0.5rem', 
              fontWeight: 500,
              fontSize: '0.875rem'
            }}>
              {t('instance_name_label')}
            </label>
            <input
              type="text"
              value={instanceName}
              onChange={(e) => setInstanceName(e.target.value)}
              placeholder={t('instance_name_placeholder')}
              style={{
                width: '100%',
                padding: '0.75rem 1rem',
                borderRadius: '0.5rem',
                border: '1px solid var(--border)',
                background: 'var(--bg)',
                fontSize: '1rem'
              }}
              required
            />
            <p style={{ 
              fontSize: '0.75rem', 
              color: 'var(--text-secondary)', 
              marginTop: '0.5rem' 
            }}>
              {t('instance_name_help')}
            </p>
          </div>

          <div style={{ marginBottom: '2rem' }}>
            <label style={{ 
              display: 'block', 
              marginBottom: '0.5rem', 
              fontWeight: 500,
              fontSize: '0.875rem'
            }}>
              {t('contact_email_label')}
            </label>
            <input
              type="email"
              value={contactEmail}
              onChange={(e) => setContactEmail(e.target.value)}
              placeholder={user?.email || 'admin@ejemplo.com'}
              style={{
                width: '100%',
                padding: '0.75rem 1rem',
                borderRadius: '0.5rem',
                border: '1px solid var(--border)',
                background: 'var(--bg)',
                fontSize: '1rem'
              }}
            />
            <p style={{ 
              fontSize: '0.75rem', 
              color: 'var(--text-secondary)', 
              marginTop: '0.5rem' 
            }}>
              {t('contact_email_help')}
            </p>
          </div>

          {/* PocketID Configuration */}
          <div style={{ marginBottom: '1.5rem' }}>
            <div style={{ 
              display: 'flex', 
              alignItems: 'center', 
              gap: '0.75rem',
              marginBottom: '0.75rem'
            }}>
              <input
                type="checkbox"
                id="pocketid-enabled"
                checked={pocketIDEnabled}
                onChange={(e) => setPocketIDEnabled(e.target.checked)}
                style={{ width: 'auto' }}
              />
              <label 
                htmlFor="pocketid-enabled"
                style={{ 
                  display: 'flex', 
                  alignItems: 'center', 
                  gap: '0.5rem',
                  fontWeight: 500,
                  fontSize: '0.875rem',
                  cursor: 'pointer'
                }}
              >
                {t('pocketid_enabled')}
              </label>
            </div>
            
            {pocketIDEnabled && (
              <div style={{ marginLeft: '1.5rem' }}>
                <label style={{ 
                  display: 'block', 
                  marginBottom: '0.5rem', 
                  fontWeight: 500,
                  fontSize: '0.875rem'
                }}>
                  {t('pocketid_url_label')}
                </label>
                <input
                  type="url"
                  value={pocketIDURL}
                  onChange={(e) => setPocketIDURL(e.target.value)}
                  placeholder="https://pocketid.ejemplo.com"
                  style={{
                    width: '100%',
                    padding: '0.75rem 1rem',
                    borderRadius: '0.5rem',
                    border: '1px solid var(--border)',
                    background: 'var(--bg)',
                    fontSize: '1rem'
                  }}
                  required={pocketIDEnabled}
                />
                <p style={{ 
                  fontSize: '0.75rem', 
                  color: 'var(--text-secondary)', 
                  marginTop: '0.5rem' 
                }}>
                  {t('pocketid_url_help')}
                </p>
              </div>
            )}
          </div>

          {/* Rate Limits Configuration */}
          <div style={{ 
            marginBottom: '2rem',
            padding: '1rem',
            background: 'var(--bg)',
            borderRadius: '0.5rem'
          }}>
            <h3 style={{ 
              fontSize: '1rem', 
              fontWeight: 600, 
              marginBottom: '1rem',
              display: 'flex',
              alignItems: 'center',
              gap: '0.5rem'
            }}>
              {t('rate_limits_section')}
            </h3>
            
            <div style={{ display: 'grid', gap: '1rem' }}>
              <div>
                <label style={{ 
                  display: 'block', 
                  marginBottom: '0.5rem', 
                  fontWeight: 500,
                  fontSize: '0.875rem'
                }}>
                  {t('default_rate_limit_label')}
                </label>
                <input
                  type="number"
                  min="10"
                  max="10000"
                  value={defaultRateLimit}
                  onChange={(e) => setDefaultRateLimit(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem 1rem',
                    borderRadius: '0.5rem',
                    border: '1px solid var(--border)',
                    background: 'var(--bg)',
                    fontSize: '1rem'
                  }}
                />
                <p style={{ 
                  fontSize: '0.75rem', 
                  color: 'var(--text-secondary)', 
                  marginTop: '0.5rem' 
                }}>
                  {t('default_rate_limit_help')}
                </p>
              </div>

              <div>
                <label style={{ 
                  display: 'block', 
                  marginBottom: '0.5rem', 
                  fontWeight: 500,
                  fontSize: '0.875rem'
                }}>
                  {t('max_workspaces_label')}
                </label>
                <input
                  type="number"
                  min="1"
                  max="100"
                  value={maxWorkspaces}
                  onChange={(e) => setMaxWorkspaces(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '0.75rem 1rem',
                    borderRadius: '0.5rem',
                    border: '1px solid var(--border)',
                    background: 'var(--bg)',
                    fontSize: '1rem'
                  }}
                />
                <p style={{ 
                  fontSize: '0.75rem', 
                  color: 'var(--text-secondary)', 
                  marginTop: '0.5rem' 
                }}>
                  {t('max_workspaces_help')}
                </p>
              </div>
            </div>
          </div>

          <Button
            type="submit"
            disabled={isSubmitting}
            style={{ 
              width: '100%', 
              padding: '1rem',
              fontSize: '1rem',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '0.5rem'
            }}
          >
            {isSubmitting ? (
              <>
                <div className="loading-spinner" style={{ width: '20px', height: '20px' }} />
                {t('completing')}
              </>
            ) : (
              <>
                <CheckCircle width="20" height="20" />
                {t('complete_setup')}
              </>
            )}
          </Button>
        </form>

        <div style={{ 
          marginTop: '2rem', 
          padding: '1rem', 
          background: 'var(--bg)', 
          borderRadius: '0.5rem',
          fontSize: '0.875rem',
          color: 'var(--text-secondary)'
        }}>
          <strong>{t('what_this_means')}:</strong>
          <ul style={{ marginTop: '0.5rem', marginLeft: '1rem', lineHeight: 1.6 }}>
            <li>{t('setup_bullet_1')}</li>
            <li>{t('setup_bullet_2')}</li>
            <li>{t('setup_bullet_3')}</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
