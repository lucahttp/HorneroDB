import { useTranslation } from 'react-i18next'
import { Language } from 'iconoir-react'

export function LanguageSelector() {
    const { i18n } = useTranslation()

    const changeLanguage = (e) => {
        i18n.changeLanguage(e.target.value)
    }

    return (
        <div className="language-selector" style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
            <Language width="1.2rem" height="1.2rem" style={{ position: 'absolute', left: '0.5rem', pointerEvents: 'none', color: 'var(--text-secondary)' }} />
            <select
                value={i18n.language}
                onChange={changeLanguage}
                className="form-select"
                style={{
                    paddingLeft: '2rem',
                    paddingRight: '2rem', // for chevron
                    paddingTop: '0.3rem',
                    paddingBottom: '0.3rem',
                    fontSize: '0.85rem',
                    background: 'transparent',
                    border: '1px solid transparent',
                    width: 'auto',
                    minWidth: '100px',
                    cursor: 'pointer'
                }}
                aria-label="Select Language"
            >
                <option value="es">Español</option>
                <option value="en">English</option>
            </select>
        </div>
    )
}
