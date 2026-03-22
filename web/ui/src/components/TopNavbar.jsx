import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { LogOut, Table2Columns, Settings as SettingsIcon } from 'iconoir-react'
import { LanguageSelector } from './LanguageSelector.jsx'
import { ThemeToggle } from './index.jsx'
import horneroLogo from '../assets/hornero solo.png'
import { useAuth } from '../context/AuthContext'

export default function TopNavbar({ workspaceId }) {
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = window.location

  const links = workspaceId ? [
    { id: 'data', label: t('data') || 'Data', icon: <Table2Columns width="1rem" height="1rem" />, path: `/workspace/${workspaceId}` },
    { id: 'settings', label: t('settings') || 'Settings', icon: <SettingsIcon width="1rem" height="1rem" />, path: `/workspace/${workspaceId}/settings` },
  ] : []

  return (
    <header className="top-navbar-global" style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '1rem 2rem',
      borderBottom: 'var(--border-thick) solid var(--border-color)',
      background: 'var(--bg-elevated)',
      position: 'sticky',
      top: 0,
      zIndex: 100,
      gap: '1rem',
      flexWrap: 'wrap'
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '2rem', flexWrap: 'wrap' }}>
        <Link to="/dashboard" style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', textDecoration: 'none', color: 'inherit' }}>
          <img src={horneroLogo} alt="Logo" style={{ width: '32px', height: '32px', objectFit: 'contain' }} />
          <span style={{ fontSize: '1.25rem', fontWeight: 800, letterSpacing: '-0.02em', display: 'block' }}>HorneroDB</span>
        </Link>

        {links.length > 0 && (
          <nav style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            {links.map(link => {
              const isActive = pathname === link.path || pathname.startsWith(link.path + '/settings') && link.id === 'settings'
              return (
                <button
                  key={link.id}
                  onClick={() => navigate(link.path)}
                  className={`btn btn-sm ${isActive ? 'btn-primary' : 'btn-ghost'}`}
                  style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderRadius: 'var(--radius-pill)' }}
                >
                  {link.icon}
                  {link.label}
                </button>
              )
            })}
          </nav>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
        <LanguageSelector />
        <ThemeToggle />
        <div className="avatar" style={{ width: '2rem', height: '2rem', fontSize: '0.8rem', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          {user?.email?.charAt(0).toUpperCase() || 'U'}
        </div>
        <button onClick={logout} className="btn btn-ghost btn-sm" style={{ padding: '0.375rem 0.5rem' }} title={t('logout') || 'Logout'}>
          <LogOut width="1rem" height="1rem" />
        </button>
      </div>
    </header>
  )
}
