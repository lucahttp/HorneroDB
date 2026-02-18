import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Group, UserPlus, Trash, ShieldCheck, QrCode } from 'iconoir-react'
import axios from 'axios'
import { Button, Badge, Modal, EmptyState, LoadingSpinner } from './index.jsx'

// Helper for toast notifications (assuming window.notify exists from App.jsx or we pass it)
// Ideally we should use a proper context or prop. For now, we'll accept `notify` as prop.
// Also needs `API_URL`.

export default function SettingsUsers({ workspaceId, roles, notify }) {
    const { t } = useTranslation()
    // We assume API_URL is available globally or we should pass it. 
    // App.jsx defines API_URL constant. We should probably export it or pass it.
    // Let's assume it's passed as prop or we use relative path if proxy set?
    // Playwright config sets baseURL.
    // In App.jsx: const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1'
    const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1'

    const [users, setUsers] = useState([])
    const [loading, setLoading] = useState(true)
    const [showImport, setShowImport] = useState(false)
    const [importing, setImporting] = useState(false)

    // Import Form State
    const [email, setEmail] = useState('')
    const [selectedRole, setSelectedRole] = useState('')

    // QR Code State
    const [qrCodeData, setQrCodeData] = useState(null)
    const [showQrModal, setShowQrModal] = useState(false)

    useEffect(() => {
        loadUsers()
    }, [workspaceId])

    const loadUsers = async () => {
        setLoading(true)
        try {
            const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/users`)
            setUsers(res.data || [])
        } catch (err) {
            console.error(err)
            if (notify) notify(t('system_error'), 'error')
        } finally {
            setLoading(false)
        }
    }

    const handleImport = async () => {
        if (!email.trim() || !selectedRole) return

        setImporting(true)
        try {
            const res = await axios.post(`${API_URL}/workspaces/${workspaceId}/users`, {
                email: email,
                role_id: selectedRole
            })
            notify(t('user_imported'), 'success')
            setShowImport(false)
            setEmail('')
            setSelectedRole('')

            // Check for QR code in response
            if (res.data.qr_code) {
                setQrCodeData(res.data.qr_code)
                setShowQrModal(true)
            }

            loadUsers()
        } catch (err) {
            console.error(err)
            // Check for specific error message
            const msg = err.response?.data?.error || t('error_import_user')
            notify(msg, 'error')
        } finally {
            setImporting(false)
        }
    }

    const removeUser = async (userId, name) => {
        if (!confirm(t('confirm_remove_user', { name }))) return

        try {
            await axios.delete(`${API_URL}/workspaces/${workspaceId}/users/${userId}`)
            setUsers(prev => prev.filter(u => u.id !== userId))
            notify(t('user_removed'), 'success')
        } catch (err) {
            console.error(err)
            notify(t('error_remove_user'), 'error')
        }
    }

    const handleShowQR = async () => {
        try {
            const res = await axios.get(`${API_URL}/auth/qr`)
            if (res.data.qr_code) {
                setQrCodeData(res.data.qr_code)
                setShowQrModal(true)
            } else {
                notify(t('error_qr_code') || 'No se pudo obtener el QR', 'error')
            }
        } catch (err) {
            console.error(err)
            const msg = err.response?.data?.error || t('error_qr_code') || 'Error obteniendo QR'
            notify(msg, 'error')
        }
    }

    if (loading) return <div style={{ padding: '3rem', display: 'flex', justifyContent: 'center' }}><LoadingSpinner /></div>

    return (
        <div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                <p style={{ color: 'var(--text-secondary)' }}>{t('manage_users_desc')}</p>
                <Button size="sm" onClick={() => setShowImport(true)}>
                    <UserPlus width="1.25rem" height="1.25rem" style={{ marginRight: '0.5rem' }} />
                    {t('add_user')}
                </Button>
            </div>

            {users.length === 0 ? (
                <EmptyState
                    icon={<Group width="2rem" height="2rem" />}
                    title={t('no_users')}
                    description={t('invite_users_hint')}
                    action={
                        <Button onClick={() => setShowImport(true)} variant="secondary">
                            {t('add_user')}
                        </Button>
                    }
                />
            ) : (
                <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                        <thead>
                            <tr style={{ borderBottom: '1px solid var(--border-color)', background: 'var(--bg-surface)' }}>
                                <th style={{ textAlign: 'left', padding: '1rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>{t('name')}</th>
                                <th style={{ textAlign: 'left', padding: '1rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>Email</th>
                                <th style={{ textAlign: 'left', padding: '1rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>{t('role_label')}</th>
                                <th style={{ textAlign: 'right', padding: '1rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}></th>
                            </tr>
                        </thead>
                        <tbody>
                            {users.map(user => (
                                <tr key={user.id} style={{ borderBottom: '1px solid var(--border-color)' }}>
                                    <td style={{ padding: '1rem', display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                                        {user.picture ? (
                                            <img src={user.picture} alt="" style={{ width: '2rem', height: '2rem', borderRadius: '50%' }} />
                                        ) : (
                                            <div className="avatar" style={{ width: '2rem', height: '2rem', fontSize: '0.8rem' }}>
                                                {user.name?.charAt(0) || user.email?.charAt(0) || '?'}
                                            </div>
                                        )}
                                        <span style={{ fontWeight: 500 }}>{user.name || t('user_fallback')}</span>
                                    </td>
                                    <td style={{ padding: '1rem', color: 'var(--text-secondary)' }}>{user.email}</td>
                                    <td style={{ padding: '1rem' }}>
                                        <Badge variant="gray">
                                            <ShieldCheck width="0.8rem" height="0.8rem" style={{ marginRight: '0.25rem' }} />
                                            {user.role_name}
                                        </Badge>
                                    </td>
                                    <td style={{ padding: '1rem', textAlign: 'right' }}>
                                        <button
                                            className="btn btn-ghost btn-sm"
                                            onClick={() => handleShowQR()}
                                            title={t('show_qr') || 'Ver QR'}
                                            style={{ marginRight: '0.5rem' }}
                                        >
                                            <QrCode width="1rem" height="1rem" />
                                        </button>
                                        <button
                                            className="btn btn-ghost btn-sm"
                                            style={{ color: 'var(--danger)' }}
                                            onClick={() => removeUser(user.id, user.name || user.email)}
                                            title={t('remove_user')}
                                        >
                                            <Trash width="1rem" height="1rem" />
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Import Modal */}
            {showImport && (
                <Modal
                    isOpen={showImport}
                    onClose={() => setShowImport(false)}
                    title={t('import_user_title')}
                    footer={
                        <>
                            <Button variant="secondary" onClick={() => setShowImport(false)}>{t('cancel')}</Button>
                            <Button onClick={handleImport} loading={importing} disabled={!email || !selectedRole}>
                                {t('import_button')}
                            </Button>
                        </>
                    }
                >
                    <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)' }}>
                        {t('import_user_desc')}
                    </p>

                    <div className="form-group">
                        <label className="form-label">{t('email_label')}</label>
                        <input
                            type="email"
                            className="form-input"
                            value={email}
                            onChange={e => setEmail(e.target.value)}
                            placeholder={t('email_placeholder')}
                            autoFocus
                        />
                    </div>

                    <div className="form-group">
                        <label className="form-label">{t('role_label')}</label>
                        <select
                            className="form-select"
                            value={selectedRole}
                            onChange={e => setSelectedRole(e.target.value)}
                        >
                            <option value="">{t('select_role') || 'Seleccionar...'}</option>
                            {roles.map(r => (
                                <option key={r.id} value={r.id}>{r.name}</option>
                            ))}
                        </select>
                    </div>
                </Modal>
            )}

            {/* QR Code Modal */}
            {showQrModal && (
                <Modal
                    isOpen={showQrModal}
                    onClose={() => setShowQrModal(false)}
                    title={t('qr_code_title') || 'Login QR Code'}
                    footer={
                        <Button onClick={() => setShowQrModal(false)}>{t('close') || 'Cerrar'}</Button>
                    }
                >
                    <div style={{ textAlign: 'center', padding: '1rem' }}>
                        <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)' }}>
                            {t('qr_code_desc') || 'Escanea este código para acceder rápidamente al portal de login.'}
                        </p>
                        {qrCodeData && (
                            <img
                                src={`data:image/png;base64,${qrCodeData}`}
                                alt="Login QR Code"
                                style={{
                                    maxWidth: '100%',
                                    height: 'auto',
                                    borderRadius: '0.5rem',
                                    border: '1px solid var(--border-color)',
                                    display: 'block',
                                    margin: '0 auto'
                                }}
                            />
                        )}
                        <p style={{ marginTop: '1rem', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                            {t('qr_code_note') || 'El usuario también recibirá las instrucciones por email si es la primera vez que ingresa.'}
                        </p>
                    </div>
                </Modal>
            )}
        </div>
    )
}
