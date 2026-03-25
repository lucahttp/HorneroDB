import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { LogOut, Table2Columns, Settings as SettingsIcon, Menu, Xmark } from 'iconoir-react'
import { LanguageSelector } from './LanguageSelector.jsx'
import { ThemeToggle } from './index.jsx'
import horneroLogo from '../assets/hornero solo.png'
import { useAuth } from '../context/AuthContext'

export default function TopNavbar({ workspaceId }) {
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = window.location
  const [isMenuOpen, setIsMenuOpen] = useState(false)

  const links = workspaceId ? [
    { id: 'data', label: t('data') || 'Data', icon: <Table2Columns width="1rem" height="1rem" />, path: `/workspace/${workspaceId}` },
    { id: 'settings', label: t('settings') || 'Settings', icon: <SettingsIcon width="1rem" height="1rem" />, path: `/workspace/${workspaceId}/settings` },
  ] : []

  return (
    <header className="top-navbar-global" style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '0.75rem 1.25rem',
      borderBottom: 'var(--border-thick) solid var(--border-color)',
      background: 'var(--bg-elevated)',
      position: 'sticky',
      top: 0,
      zIndex: 100,
      gap: '1rem',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '1.5rem' }}>
        <Link to="/dashboard" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', textDecoration: 'none', color: 'inherit' }}>
          <img src={horneroLogo} alt="Logo" style={{ width: '28px', height: '28px', objectFit: 'contain' }} />
          <span style={{ fontSize: '1.1rem', fontWeight: 800, letterSpacing: '-0.02em' }}>HorneroDB</span>
        </Link>

        {links.length > 0 && (
          <nav style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
            {links.map(link => {
              const isActive = pathname === link.path || pathname.startsWith(link.path + '/settings') && link.id === 'settings'
              return (
                <button
                  key={link.id}
                  onClick={() => navigate(link.path)}
                  className={`btn btn-sm ${isActive ? 'btn-primary' : 'btn-ghost'}`}
                  style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', borderRadius: 'var(--radius-pill)', padding: '0.35rem 0.75rem' }}
                >
                  {link.icon}
                  <span className="hidden sm:inline">{link.label}</span>
                </button>
              )
            })}
          </nav>
        )}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <ThemeToggle />
        
        {/* Desktop Actions */}
        <div className="hidden md:flex items-center gap-4" style={{ marginLeft: '0.5rem' }}>
          <LanguageSelector />
          <div className="avatar" style={{ width: '1.85rem', height: '1.85rem', fontSize: '0.75rem', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            {user?.email?.charAt(0).toUpperCase() || 'U'}
          </div>
          <button onClick={logout} className="btn btn-ghost btn-sm" style={{ padding: '0.375rem 0.5rem' }} title={t('logout') || 'Logout'}>
            <LogOut width="1rem" height="1rem" />
          </button>
        </div>

        {/* Mobile Menu Trigger */}
        <button 
          className="md:hidden btn btn-ghost btn-sm" 
          onClick={() => setIsMenuOpen(!isMenuOpen)}
          style={{ padding: '0.35rem' }}
        >
          {isMenuOpen ? <Xmark width="1.5rem" height="1.5rem" /> : <Menu width="1.5rem" height="1.5rem" />}
        </button>
      </div>

      {/* Mobile Menu Drawer */}
      {isMenuOpen && (
        <div 
          className="md:hidden" 
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            background: 'var(--bg-elevated)',
            borderBottom: 'var(--border-thick) solid var(--border-color)',
            padding: '1.5rem',
            display: 'flex',
            flexDirection: 'column',
            gap: '1.25rem',
            boxShadow: 'var(--shadow-lg)',
            zIndex: 99
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <div className="avatar" style={{ width: '2.5rem', height: '2.5rem', fontSize: '1rem' }}>
              {user?.email?.charAt(0).toUpperCase() || 'U'}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontWeight: 700, fontSize: '0.9rem' }}>{user?.email}</span>
              <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{t('user_profile')}</span>
            </div>
          </div>
          
          <div style={{ height: '1px', background: 'var(--border-light)' }} />
          
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            <label style={{ fontSize: '0.75rem', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
              {t('language') || 'Language'}
            </label>
            <LanguageSelector />
          </div>

          <button 
            onClick={logout} 
            className="btn btn-secondary" 
            style={{ justifyContent: 'center', gap: '0.5rem', width: '100%' }}
          >
            <LogOut width="1.1rem" height="1.1rem" />
            {t('logout') || 'Logout'}
          </button>
        </div>
      )}
    </header>
  )
}
