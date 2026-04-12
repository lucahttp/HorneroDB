import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import TopNavbar from '../components/TopNavbar.jsx'
import { notify } from '../components/Toast.jsx'
import { Button } from '../components/index.jsx'
import { CheckCircle, Xmark, User } from 'iconoir-react'

export default function InstanceAdmin() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [isAdmin, setIsAdmin] = useState(false)

  useEffect(() => {
    // Verificar si el usuario actual es admin de instancia
    axios.get(`${API_URL}/auth/me`)
      .then(res => {
        if (res.data.data?.can_create_workspaces) {
          setIsAdmin(true)
          loadUsers()
        } else {
          setIsAdmin(false)
          setLoading(false)
        }
      })
      .catch(() => {
        setIsAdmin(false)
        setLoading(false)
      })
  }, [])

  const loadUsers = async () => {
    try {
      const res = await axios.get(`${API_URL}/admin/users`)
      setUsers(res.data.data || [])
    } catch (err) {
      notify(t('error_loading_users'), 'error')
    } finally {
      setLoading(false)
    }
  }

  const togglePermission = async (userId, currentValue) => {
    try {
      await axios.patch(`${API_URL}/admin/users/${userId}`, {
        can_create_workspaces: !currentValue
      })

      // Actualizar localmente
      setUsers(users.map(u =>
        u.id === userId
          ? { ...u, can_create_workspaces: !currentValue }
          : u
      ))

      notify(
        !currentValue
          ? (t('permissions_granted'))
          : (t('permissions_revoked')),
        'success'
      )
    } catch (err) {
      notify(t('error_updating_permissions'), 'error')
    }
  }

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  if (!isAdmin) {
    return (
      <div style={{ minHeight: '100vh', background: 'var(--bg)' }}>
        <TopNavbar />
        <div style={{ maxWidth: '960px', margin: '0 auto', padding: '3rem 2rem', textAlign: 'center' }}>
          <h1 style={{ fontSize: '1.5rem', marginBottom: '1rem' }}>
            {t('access_denied')}
          </h1>
          <p style={{ color: 'var(--text-secondary)' }}>
            {t('admin_only_page')}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg)' }}>
      <TopNavbar />

      <div style={{ maxWidth: '960px', margin: '0 auto', padding: '3rem 2rem' }}>
        <div style={{ marginBottom: '2rem' }}>
          <h1 style={{ fontSize: '2rem', fontWeight: 900, marginBottom: '0.5rem' }}>
            {t('instance_admin_title')}
          </h1>
          <p style={{ color: 'var(--text-secondary)' }}>
            {t('instance_admin_subtitle')}
          </p>
        </div>

        <div style={{ background: 'var(--surface)', borderRadius: '0.75rem', overflow: 'hidden' }}>
          <div style={{
            padding: '1rem 1.5rem',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center'
          }}>
            <h2 style={{ fontSize: '1.125rem', fontWeight: 600 }}>
              {t('users_list')}
              <span style={{
                marginLeft: '0.5rem',
                padding: '0.25rem 0.5rem',
                background: 'var(--primary)',
                color: 'white',
                borderRadius: '9999px',
                fontSize: '0.875rem'
              }}>
                {users.length}
              </span>
            </h2>
          </div>

          {users.length === 0 ? (
            <div style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-secondary)' }}>
              {t('no_users_found')}
            </div>
          ) : (
            <div style={{ maxHeight: '600px', overflow: 'auto' }}>
              {users.map((u) => (
                <div
                  key={u.id}
                  style={{
                    padding: '1rem 1.5rem',
                    borderBottom: '1px solid var(--border)',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    ':hover': { background: 'var(--bg)' }
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    {u.picture ? (
                      <img
                        src={u.picture}
                        alt={u.name || u.email}
                        style={{ width: '40px', height: '40px', borderRadius: '50%' }}
                      />
                    ) : (
                      <div style={{
                        width: '40px',
                        height: '40px',
                        borderRadius: '50%',
                        background: 'var(--primary)',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: 'white'
                      }}>
                        <User width="20" height="20" />
                      </div>
                    )}
                    <div>
                      <div style={{ fontWeight: 600 }}>
                        {u.name || u.email}
                        {u.id === user?.id && (
                          <span style={{
                            marginLeft: '0.5rem',
                            fontSize: '0.75rem',
                            color: 'var(--primary)'
                          }}>
                            ({t('you')})
                          </span>
                        )}
                      </div>
                      <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        {u.email}
                      </div>
                    </div>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                    <div style={{ textAlign: 'right' }}>
                      <div style={{
                        fontSize: '0.875rem',
                        fontWeight: 500,
                        color: u.can_create_workspaces ? 'var(--success)' : 'var(--text-secondary)'
                      }}>
                        {u.can_create_workspaces
                          ? (t('can_create_workspaces'))
                          : (t('cannot_create_workspaces'))
                        }
                      </div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                        {t('last_login')}: {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : 'Nunca'}
                      </div>
                    </div>

                    <Button
                      onClick={() => togglePermission(u.id, u.can_create_workspaces)}
                      variant={u.can_create_workspaces ? "danger" : "success"}
                      size="sm"
                      style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
                    >
                      {u.can_create_workspaces ? (
                        <>
                          <Xmark width="16" height="16" />
                          {t('revoke')}
                        </>
                      ) : (
                        <>
                          <CheckCircle width="16" height="16" />
                          {t('grant')}
                        </>
                      )}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <div style={{
          marginTop: '2rem',
          padding: '1rem',
          background: 'var(--surface)',
          borderRadius: '0.5rem',
          fontSize: '0.875rem',
          color: 'var(--text-secondary)'
        }}>
          <strong>{t('note')}:</strong> {t('instance_admin_note')}
        </div>
      </div>
    </div>
  )
}
