import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { DesktopView } from './components/DesktopView'
import { ErrorBoundary } from './components/ErrorBoundary'
import { detectMode } from './lib/webMode'
import './style.css'

// Computed once. 'desktopview' (/desktop/) boots DesktopView and never App, which is Wails-coupled.
const mode = detectMode()

// Desktop-only CSS scope (transparency, drag regions); the web share page and the desktop viewer never get it.
if (mode === 'desktop') document.documentElement.classList.add('is-desktop')

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    {/* Last resort (Portty 05-09): a recovery screen instead of a blank window. */}
    <ErrorBoundary
      label="root"
      fallback={
        <div className="banner banner--warning" role="status">
          <span className="banner__message">Portty hit an error.</span>
          <button type="button" className="banner__action" onClick={() => window.location.reload()}>Reload</button>
        </div>
      }
    >
      {mode === 'desktopview' ? <DesktopView /> : <App />}
    </ErrorBoundary>
  </React.StrictMode>
)
