import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'
import raw from '../../components/SettingsSheet.tsx?raw'

// Use fs.readFileSync for CSS — vitest/jsdom does not support ?raw imports for CSS.
const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// Portty 02-11: the running-only URL row in Settings ▸ Sharing has Open /
// Copy / QR secondary push buttons (UI-SPEC "Sharing pane").

describe('WEB-01: Open in browser button', () => {
  it('imports BrowserOpenURL from Wails runtime', () => {
    expect(raw).toContain('BrowserOpenURL')
  })
  it('calls BrowserOpenURL on Open button click', () => {
    expect(raw).toContain('BrowserOpenURL(serverURL)')
  })
  it('has aria-label for Open button', () => {
    expect(raw).toContain('Open dashboard in browser')
  })
})

describe('WEB-02: Copy URL to clipboard', () => {
  it('imports ClipboardSetText from Wails runtime', () => {
    expect(raw).toContain('ClipboardSetText')
  })
  it('has handleCopyURL function', () => {
    expect(raw).toContain('async function handleCopyURL')
  })
  it('calls ClipboardSetText in handleCopyURL', () => {
    expect(raw).toContain('ClipboardSetText(serverURL)')
  })
  it('shows Copied! feedback', () => {
    expect(raw).toContain("urlCopied ? 'Copied!' : 'Copy'")
  })
  it('has aria-label for Copy button', () => {
    expect(raw).toContain('Copy dashboard URL to clipboard')
  })
})

describe('WEB-03: QR code display', () => {
  it('imports GetWebServerQRCode from App bindings', () => {
    expect(raw).toContain('GetWebServerQRCode')
  })
  it('has handleToggleDashQR function', () => {
    expect(raw).toContain('async function handleToggleDashQR')
  })
  it('renders QR image with correct alt text', () => {
    expect(raw).toContain('alt="QR code for dashboard URL"')
  })
  it('renders QR image with base64 data URI', () => {
    expect(raw).toContain('data:image/png;base64,${dashQRb64}')
  })
  it('resets QR state when server stops', () => {
    expect(raw).toContain('setDashQRb64(null)')
  })
  it('has aria-labels and QR / Hide QR labels for the toggle', () => {
    expect(raw).toContain('Show QR code')
    expect(raw).toContain('Hide QR code')
    expect(raw).toContain("showDashQR ? 'Hide QR' : 'QR'")
  })
})

describe('WEB-03: QR error handling', () => {
  it('shows error message on QR fetch failure', () => {
    expect(raw).toContain('QR unavailable')
  })
})

describe('Portty 02-11: URL row uses the shared row and button styles', () => {
  it('the three URL actions are secondary push buttons', () => {
    const start = raw.indexOf('{isServerRunning && serverURL && (')
    const block = raw.slice(start, raw.indexOf('{isServerRunning && (qrError', start))
    expect(block.match(/className="button button--secondary"/g)).toHaveLength(3)
    expect(block).toContain('className="settings-row__text settings-row__mono"')
  })
  it('the URL is 12px ui-monospace with an ellipsis', () => {
    const idx = cssRaw.indexOf('.settings-row__mono {')
    const block = cssRaw.slice(idx, cssRaw.indexOf('}', idx) + 1)
    expect(block).toContain('ui-monospace')
    expect(block).toContain('font-size: 12px')
    expect(block).toContain('text-overflow: ellipsis')
  })
  it('QR image keeps its 200px dimensions', () => {
    const idx = cssRaw.indexOf('.settings-row__qr {')
    const block = cssRaw.slice(idx, cssRaw.indexOf('}', idx) + 1)
    expect(block).toContain('width: 200px')
    expect(block).toContain('height: 200px')
  })
})
