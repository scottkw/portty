import { describe, it, expect } from 'vitest'
import raw from '../App.tsx?raw'
import terminalPanelRaw from '../components/TerminalPanel.tsx?raw'
import tabBarRaw from '../components/TabBar.tsx?raw'

describe('Phase 97 SER-01: App.tsx saver-registry round-trip — Plan 97-03 implementation', () => {
  it('App.tsx imports stripAnsi from ./lib/stripAnsi', () => {
    expect(raw).toMatch(/import\s*\{\s*stripAnsi\s*\}\s*from\s*['"]\.\/lib\/stripAnsi['"]/)
  })

  it('App.tsx imports sanitizeFilename from ./lib/sanitizeFilename', () => {
    expect(raw).toMatch(/import\s*\{\s*sanitizeFilename\s*\}\s*from\s*['"]\.\/lib\/sanitizeFilename['"]/)
  })

  it('App.tsx imports SaveTerminalSession from wailsjs binding', () => {
    expect(raw).toMatch(/import\s*\{[^}]*SaveTerminalSession[^}]*\}\s*from\s*['"]\.\/wailsjs\/go\/main\/App['"]/)
  })

  it('App.tsx declares serializerRegistry state (Record<sessionId, () => string | null>)', () => {
    expect(raw).toMatch(/serializerRegistry/)
    expect(raw).toMatch(/setSerializerRegistry/)
  })

  it('App.tsx declares handleRegisterSaver useCallback', () => {
    expect(raw).toMatch(/handleRegisterSaver\s*=\s*useCallback/)
  })

  it('App.tsx declares handleRequestSave useCallback that calls SaveTerminalSession after stripAnsi + sanitizeFilename', () => {
    expect(raw).toMatch(/handleRequestSave\s*=\s*useCallback/)
    // The handleRequestSave body must reference the three load-bearing operations:
    expect(raw).toMatch(/stripAnsi\(/)
    expect(raw).toMatch(/sanitizeFilename\(/)
    expect(raw).toMatch(/SaveTerminalSession\(/)
  })

  it('handleRequestSave shows a banner when registry is empty (Serialize OFF)', () => {
    // The banner must reference Serialize / Settings vocabulary so the user
    // gets actionable guidance.
    expect(raw).toMatch(/Enable the Serialize plugin in Settings/)
  })

  it('App.tsx passes onRegisterSaver={handleRegisterSaver} to TerminalPanel', () => {
    expect(raw).toMatch(/onRegisterSaver\s*=\s*\{handleRegisterSaver/)
  })

  // Portty 02-08 (UI-SPEC "Context menus"): the menu item shows only while the serialize plugin is on.
  it('App.tsx passes handleRequestSave to TabBar while the serialize plugin is on', () => {
    expect(raw).toContain('onRequestSave={pluginConfig?.serialize ? handleRequestSave : undefined}')
  })

  // Defensive: TerminalPanel + TabBar source files exist and contain expected
  // landmark text — these assertions guard against accidental file deletion
  // but DO NOT assert Plan 97-04's wiring (that lands in 97-04 and is
  // covered by TabBar.test.tsx + TerminalPanel.test.tsx).
  it('TerminalPanel.tsx file is loadable (defensive — Plan 97-04 wires onRegisterSaver consumer)', () => {
    expect(terminalPanelRaw.length).toBeGreaterThan(100)
  })

  it('TabBar.tsx file is loadable (defensive — Plan 97-04 wires onRequestSave consumer)', () => {
    expect(tabBarRaw.length).toBeGreaterThan(100)
  })
})
