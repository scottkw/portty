import { describe, it, expect } from 'vitest'
import raw from '../../components/SettingsTab.tsx?raw'
import appRaw from '../../App.tsx?raw'
import themesRaw from '../../themes.ts?raw'

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

describe('THM-01: Appearance section with theme selector', () => {
  it('imports ALLOWED_THEMES from themes module', () => {
    expect(raw).toContain("import { ALLOWED_THEMES } from '../themes'")
  })

  it('defines ALLOWED_THEMES constant at module level in themes.ts', () => {
    expect(themesRaw).toContain('ALLOWED_THEMES: string[]')
  })

  it('sets THEME_NAMES to ALLOWED_THEMES', () => {
    expect(raw).toContain('THEME_NAMES = ALLOWED_THEMES')
  })

  it('props include selectedTheme', () => {
    expect(raw).toContain('selectedTheme: string')
  })

  it('props include onThemeChange callback', () => {
    expect(raw).toContain('onThemeChange: (name: string) => void')
  })

  it('renders theme select with THEME_NAMES options', () => {
    expect(raw).toContain('THEME_NAMES.map')
  })

  it('select value is bound to selectedTheme prop', () => {
    expect(raw).toContain('value={selectedTheme}')
  })

  it('select onChange calls onThemeChange', () => {
    expect(raw).toContain('onThemeChange(e.target.value)')
  })

  it('displays theme names with underscores replaced by spaces', () => {
    expect(raw).toContain("name.replace(/_/g, ' ')")
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
  it('contains theme selector (Appearance group)', () => {
    expect(raw).toContain('settings-panel__theme-select')
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

  it('shows installed-but-not-connected message', () => {
    expect(raw).toContain('Daemon running but not connected to a Tailscale network')
  })

  it('shows not detected message', () => {
    expect(raw).toContain('Not detected')
  })

  it('has no tailscale path override input (Portty 01-11)', () => {
    expect(raw).not.toContain('Path to tailscale')
  })
})

describe('THM-04: Allowlist-only theme picker', () => {
  it('ALLOWED_THEMES contains Tomorrow_Night (default theme survives audit)', () => {
    expect(themesRaw).toContain('"Tomorrow_Night"')
  })

  it('ALLOWED_THEMES contains at least one light-background theme (Novel)', () => {
    expect(themesRaw).toContain('"Novel"')
  })

  it('ALLOWED_THEMES contains at least one dark-background theme (Dracula)', () => {
    expect(themesRaw).toContain('"Dracula"')
  })

  it('does NOT contain "default" in ALLOWED_THEMES (namespace artifact excluded)', () => {
    const allowlistStart = themesRaw.indexOf('ALLOWED_THEMES: string[]')
    const allowlistEnd = themesRaw.indexOf(']', allowlistStart)
    expect(allowlistStart).toBeGreaterThan(-1)
    const allowlistBlock = themesRaw.slice(allowlistStart, allowlistEnd + 1)
    expect(allowlistBlock).not.toContain('"default"')
  })

  it('does NOT derive theme names from Object.keys(xtermThemes)', () => {
    expect(raw).not.toContain('Object.keys(xtermThemes).sort()')
  })
})

describe('THM-04: localStorage fallback guard in App.tsx', () => {
  it('validates stored theme against ALLOWED_THEMES before using it', () => {
    expect(appRaw).toContain('ALLOWED_THEMES.includes(stored)')
  })

  it('imports ALLOWED_THEMES from themes module', () => {
    expect(appRaw).toContain('ALLOWED_THEMES')
  })
})

describe('TS-01/TS-02: 4-state Tailscale detection', () => {
  it('tailscaleHealth type includes binaryFound field', () => {
    expect(raw).toContain('binaryFound: boolean')
  })

  it('tailscaleHealth type includes daemonUp field', () => {
    expect(raw).toContain('daemonUp: boolean')
  })

  it('tailscaleHealth type includes platformHint field', () => {
    expect(raw).toContain('platformHint: string')
  })

  it('shows Daemon Stopped status text', () => {
    expect(raw).toContain("'Daemon Stopped'")
  })

  it('has Show diagnostics collapsible section', () => {
    expect(raw).toContain('Show diagnostics')
  })

  it('diagnostics includes Binary detected step', () => {
    expect(raw).toContain('Binary detected')
  })

  it('diagnostics includes Daemon running step', () => {
    expect(raw).toContain('Daemon running')
  })

  it('diagnostics includes Connected to Tailscale step', () => {
    expect(raw).toContain('Connected to Tailscale')
  })

  it('diagnostics includes TLS certificates ready step', () => {
    expect(raw).toContain('TLS certificates ready')
  })

  it('has platform-specific instruction for macOS', () => {
    expect(raw).toContain('open Tailscale from Applications or the menu bar')
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
