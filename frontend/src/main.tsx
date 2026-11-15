import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { detectMode } from './lib/webMode'
import './style.css'

// Desktop-only CSS scope (transparency, drag regions); the web share page never gets it.
if (detectMode() === 'desktop') document.documentElement.classList.add('is-desktop')

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
