import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import i18n from './i18n'
import App from './App.jsx'
import { initTelemetry } from './telemetry/rum'

initTelemetry()

/**
 * Keeps the `lang` attribute of the document in sync with the active
 * language: screen readers use it to pick the right pronunciation rules.
 */
const syncDocumentLanguage = (language) => {
  document.documentElement.lang = language || 'es'
}

syncDocumentLanguage(i18n.language)
i18n.on('languageChanged', syncDocumentLanguage)

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
