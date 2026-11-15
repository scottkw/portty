import { describe, it, expect, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react'
import raw from '../../components/SettingsTab.tsx?raw'
import { SettingsTab } from '../SettingsTab'

// Every Wails binding resolves to undefined; the DOM case only checks static copy.
vi.mock('../../wailsjs/go/main/App', async (importOriginal) => {
  const mod = await importOriginal<Record<string, unknown>>()
  return Object.fromEntries(Object.keys(mod).map((k) => [k, vi.fn().mockResolvedValue(undefined)]))
})
vi.mock('../PluginsSection', () => ({ PluginsSection: () => null }))
vi.mock('../SettingsJumpBar', () => ({ SettingsJumpBar: () => null, SETTINGS_JUMP_LINKS: [] }))
vi.mock('../SettingsSearch', () => ({ SettingsSearch: () => null }))

// Source-inspection tests for SettingsTab.tsx (UI-02: Settings as sidebar tab).
// Verifies the component was refactored from a modal into an inline sidebar tab.

describe('UI-02 Gap 1: SettingsTab exports', () => {
  it('exports SettingsTab function component', () => {
    expect(raw).toContain('export function SettingsTab')
  })
})

describe('UI-02 Gap 2: SettingsTab props interface', () => {
  it('props do NOT include clis (STRP-01: no CLI detection)', () => {
    expect(raw).not.toMatch(/\bclis\b/)
  })

  it('props include tailscaleHealth', () => {
    expect(raw).toContain('tailscaleHealth')
  })

  it('props include onWebServerStateChange', () => {
    expect(raw).toContain('onWebServerStateChange')
  })

  it('props do NOT include isOpen (modal remnant)', () => {
    // isOpen is a modal-specific prop — must not appear in the SettingsTabProps interface.
    // We check the interface block specifically to avoid false positives from comments.
    const interfaceStart = raw.indexOf('interface SettingsTabProps')
    const interfaceEnd = raw.indexOf('}', interfaceStart)
    expect(interfaceStart).toBeGreaterThan(-1)
    const interfaceBlock = raw.slice(interfaceStart, interfaceEnd + 1)
    expect(interfaceBlock).not.toContain('isOpen')
  })

  it('props do NOT include onClose (modal remnant)', () => {
    // onClose is a modal-specific prop — must not appear in the SettingsTabProps interface.
    const interfaceStart = raw.indexOf('interface SettingsTabProps')
    const interfaceEnd = raw.indexOf('}', interfaceStart)
    expect(interfaceStart).toBeGreaterThan(-1)
    const interfaceBlock = raw.slice(interfaceStart, interfaceEnd + 1)
    expect(interfaceBlock).not.toContain('onClose')
  })
})

describe('UI-02 Gap 3: No modal shell classes', () => {
  it('does NOT contain settings-overlay class', () => {
    expect(raw).not.toContain('settings-overlay')
  })

  it('does NOT contain settings-panel__header class', () => {
    expect(raw).not.toContain('settings-panel__header"')
  })

  it('does NOT contain settings-panel__footer class', () => {
    expect(raw).not.toContain('settings-panel__footer"')
  })

  it('does NOT contain settings-panel__close class', () => {
    expect(raw).not.toContain('settings-panel__close"')
  })
})

describe('UI-02 Gap 4: settings-tab outer wrapper', () => {
  it('has className="settings-tab" as outer wrapper', () => {
    expect(raw).toContain('className="settings-tab"')
  })
})

describe('UI-02 Gap 5: Mount-based useEffect', () => {
  it('has useEffect with empty dependency array []', () => {
    // A mount-only effect ends with }, []) pattern.
    expect(raw).toContain('}, [])')
  })
})

describe('THM-01: Appearance follows the system (no theme picker)', () => {
  it('has no theme props or theme list import', () => {
    expect(raw).not.toContain('ALLOWED_THEMES')
    expect(raw).not.toContain('selectedTheme')
    expect(raw).not.toContain('onThemeChange')
    expect(raw).not.toContain('uiTheme')
  })

  it('renders "Automatic" and no Interface Theme / Terminal Theme control', async () => {
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(React.createElement(SettingsTab, {
        tailscaleHealth: null,
        webServerMode: null,
        onWebServerStateChange: vi.fn().mockResolvedValue(undefined),
      }))
    })
    expect(container.textContent).toContain('Automatic')
    expect(container.textContent).toContain("Follows your Mac's Light or Dark setting in System Settings.")
    const names = Array.from(container.querySelectorAll('button, select, input, [role="switch"]'))
      .map((el) => `${el.getAttribute('aria-label') ?? ''} ${el.closest('.settings-panel__field-group')?.querySelector('label')?.textContent ?? ''}`)
    expect(names.some((n) => /Interface Theme|Terminal Theme/.test(n))).toBe(false)
    expect(container.textContent).not.toMatch(/Interface Theme|Terminal Theme/)
    await act(async () => root.unmount())
    container.remove()
  })
})

describe('SETT-01: Single scrollable page (no sub-tabs)', () => {
  it('does NOT contain settings-panel__tabs div', () => {
    expect(raw).not.toContain('settings-panel__tabs')
  })

  it('does NOT contain settings-panel__tab-btn class', () => {
    expect(raw).not.toContain('settings-panel__tab-btn')
  })

  it('does NOT contain role="tablist"', () => {
    expect(raw).not.toContain('role="tablist"')
  })

  it('does NOT contain activeTab conditional gating', () => {
    expect(raw).not.toContain("activeTab === ")
  })

  it('does NOT have activeTab in props interface', () => {
    const interfaceStart = raw.indexOf('interface SettingsTabProps')
    const interfaceEnd = raw.indexOf('}', interfaceStart)
    expect(interfaceStart).toBeGreaterThan(-1)
    const interfaceBlock = raw.slice(interfaceStart, interfaceEnd + 1)
    expect(interfaceBlock).not.toContain('activeTab')
  })

  it('does NOT have onActiveTabChange in props interface', () => {
    const interfaceStart = raw.indexOf('interface SettingsTabProps')
    const interfaceEnd = raw.indexOf('}', interfaceStart)
    expect(interfaceStart).toBeGreaterThan(-1)
    const interfaceBlock = raw.slice(interfaceStart, interfaceEnd + 1)
    expect(interfaceBlock).not.toContain('onActiveTabChange')
  })
})

describe('SETT-02: Section headers present', () => {
  it('has Appearance section header', () => {
    // Phase 104: header carries id="settings-appearance" anchor for the jump-bar.
    expect(raw).toMatch(/<h3[^>]*>Appearance<\/h3>/)
  })

  it('has Web Server section header', () => {
    // Phase 104: header carries id="settings-web-server" anchor for the jump-bar.
    expect(raw).toMatch(/<h3[^>]*>Web Server<\/h3>/)
  })

  it('has no Paths section header (Portty 01-11)', () => {
    expect(raw).not.toMatch(/<h3[^>]*>Paths<\/h3>/)
  })
})

describe('SETT-03: All content groups present simultaneously', () => {
  it('contains the Appearance row (Appearance group)', () => {
    expect(raw).toContain('settings-panel__appearance-value')
  })

  it('contains CT disclosure (Web Server group)', () => {
    expect(raw).toContain('ct-disclosure')
  })

  it('has no Paths section: no settings-paths anchor and no Save Paths button (Portty 01-11)', () => {
    expect(raw).not.toContain('settings-paths')
    expect(raw).not.toContain('Save Paths')
    expect(raw).not.toContain('settings-panel__table')
  })
})

describe('TAILSCALE-PATH-01: Tailscale status in Settings', () => {
  it('renders Tailscale label', () => {
    expect(raw).toContain('Tailscale')
  })

  it('shows connected status with domain or ip when tailscaleHealth.connected is true', () => {
    expect(raw).toContain('tailscaleHealth.connected')
    expect(raw).toContain('tailscaleHealth.domain || tailscaleHealth.ip')
  })

  it('shows the NET-01 sentence when not connected (Portty 01-12)', () => {
    expect(raw).toContain(': TAILSCALE_OFF_REASON)')
  })

  it('has no tailscale path override input (Portty 01-11)', () => {
    expect(raw).not.toContain('Path to tailscale')
  })
})

// NET-01 (Portty 01-12, CONTEXT Amendment 1): two states only, no diagnostics.
describe('NET-01: two-state Tailscale status (replaces TS-01/TS-02 4-state detection)', () => {
  it('uses the shared TailscaleHealth type with no multi-state fields', () => {
    expect(raw).toContain('tailscaleHealth: TailscaleHealth | null')
    expect(raw).not.toMatch(/binaryFound|daemonUp|platformHint|permissionLimited/)
  })

  it('has no multi-state labels or diagnostics checklist', () => {
    for (const s of ['Daemon Stopped', 'Not Installed', 'Permission Limited', 'Show diagnostics', 'Binary detected', 'TLS certificates ready']) {
      expect(raw).not.toContain(s)
    }
  })

  it('shows Connected or Not connected', () => {
    expect(raw).toContain("h.connected ? 'Connected' : 'Not connected'")
  })
})

describe('SET-02: No inline fontSize override on description elements', () => {
  it('does NOT have fontSize: 0.8rem on any description paragraph', () => {
    // Check that no settings-panel__description element has an inline fontSize override.
    // Other elements (e.g. diagnostics summary/div) may legitimately use inline fontSize.
    expect(raw).not.toContain('settings-panel__description" style={{ marginTop: \'0.25rem\', fontSize: \'0.8rem\' }}')
  })

  it('does NOT have marginTop: 0.25rem on Tailscale status description', () => {
    const statusIdx = raw.indexOf('tailscaleStatusText(tailscaleHealth)')
    expect(statusIdx).toBeGreaterThan(-1)
    const nearStatus = raw.slice(statusIdx, statusIdx + 500)
    expect(nearStatus).not.toContain("marginTop: '0.25rem'")
  })
})

// CONTEXT Amendment 2: tabs always close on exit, so there is no auto-close toggle.
describe('TERM-05: no auto-close toggle', () => {
  it('Session Behavior has no auto-close toggle or bindings', () => {
    expect(raw).not.toContain('Auto-close tab on exit')
    expect(raw).not.toContain('AutoCloseSession')
    expect(raw).toContain('id="settings-session-behavior"')
  })
})

describe('Portty 01-11: Behavior has only the start-minimized toggle', () => {
  it('Behavior renders exactly one toggle (start minimized) and no awaiting-input toggle', () => {
    const start = raw.indexOf('id="settings-behavior"')
    const end = raw.indexOf('id="settings-session-behavior"', start)
    expect(start).toBeGreaterThan(-1)
    expect(end).toBeGreaterThan(start)
    const behavior = raw.slice(start, end)
    expect(behavior.match(/className="settings-panel__toggle-input"/g) ?? []).toHaveLength(1)
    expect(behavior).toContain('Start minimized to system tray')
    expect(raw).not.toContain('Notify me when a session is awaiting input')
    expect(raw).not.toMatch(/NotifyOnWaiting|notification:permission|not allowed to send notifications/)
  })
})

// Portty 02-06 (NAV-03): the Hub is gone, so the stay-on-Hub preference is too.
describe('NAV-03: no stay-on-Hub toggle', () => {
  it('renders no "Stay on Hub" text and has no stay-on-Hub bindings', async () => {
    expect(raw).not.toMatch(/StayOnHub/i)
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    await act(async () => {
      root.render(React.createElement(SettingsTab, {
        tailscaleHealth: null,
        webServerMode: null,
        onWebServerStateChange: vi.fn().mockResolvedValue(undefined),
      }))
    })
    expect(container.textContent).toContain('Session Behavior')
    expect(container.textContent).not.toMatch(/Stay on Hub/i)
    await act(async () => root.unmount())
    container.remove()
  })
})
