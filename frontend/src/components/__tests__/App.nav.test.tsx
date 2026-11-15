import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'

// Source-inspection tests for App.tsx sidebar navigation wiring (56-01; Portty 02-04).
// These verify the wiring contract without mounting the full component tree
// (which requires Wails runtime mocks).

/** Slice from `marker` to the first `end` after it. */
function slice(marker: string, end: string): string {
  const start = raw.indexOf(marker)
  if (start === -1) throw new Error(`${marker} not found in App.tsx`)
  const stop = raw.indexOf(end, start + marker.length)
  if (stop === -1) throw new Error(`${end} not found after ${marker}`)
  return raw.slice(start, stop)
}

// Portty 02-04 NAV-01: the sidebar lists every daemon session from an always-on poll.
describe('Portty NAV-01: always-on session poll feeds the sidebar', () => {
  const poll = () => {
    const end = raw.indexOf('const list = await ListSessions()')
    expect(end).toBeGreaterThan(-1)
    return raw.slice(raw.lastIndexOf('useEffect(() => {', end), raw.indexOf('}, [])', end))
  }

  it('polls ListSessions every 3000 ms with no tab gate', () => {
    const body = poll()
    expect(body).toContain('setInterval(() => void refresh(), 3000)')
    expect(body).toContain('setSessions(list)')
    expect(body).not.toContain('activeId')
    expect(body).not.toContain('HUB_TAB')
  })

  it('is skipped in web mode and cleans up', () => {
    const body = poll()
    expect(body).toContain("if (mode === 'web') return")
    expect(body).toContain('clearInterval(interval)')
    expect(body).toContain('cancelled = true')
  })

  it('has no hubSessions state left', () => {
    expect(raw).not.toContain('hubSessions')
  })

  it('renders Sidebar with sessions, the active tab and onOpenLocal', () => {
    const block = slice('<Sidebar', '/>')
    expect(block).toContain('sessions={sessions}')
    expect(block).toContain('activeTab={tabs.find((t) => t.id === activeId) ?? null}')
    expect(block).toContain('onOpenLocal={handleOpenSessionTab}')
  })

  it('gates the Sidebar on mode !== web (WEBCHAT-03 guest scope)', () => {
    const idx = raw.indexOf('<Sidebar')
    expect(raw.slice(idx - 40, idx)).toMatch(/\{mode !== 'web' && \(\s*$/)
  })

  it('handleOpenSessionTab focuses an open tab or appends and activates one', () => {
    const body = slice('const handleOpenSessionTab = useCallback(', '}, [tabs])')
    expect(body).toContain('tabs.find((t) => t.id === sessionId)')
    expect(body).toContain('setActiveId(existing.id)')
    expect(body).toContain('setTabs((prev) => [...prev, newTab])')
    expect(body).toContain('setActiveId(newTab.id)')
  })
})

// Portty 02-04 NAV-03: launch goes straight to sidebar plus tabs.
describe('Portty NAV-03: no Welcome tab at launch', () => {
  it('starts with no tabs and no active tab in both modes', () => {
    expect(raw).toContain('useState<Tab[]>([])')
    expect(raw).toContain('useState<string | null>(null)')
    expect(raw).not.toContain('initialTabs')
  })

  it('init still restores running sessions as tabs', () => {
    const body = slice('async function init()', 'void init()')
    expect(body).toContain('setTabs(restoredTabs)')
    expect(body).toContain('setActiveId(restoredTabs[0].id)')
  })

  it('no Home or Hub sidebar wiring remains', () => {
    expect(raw).not.toContain('handleHome')
    expect(raw).not.toContain('handleOpenHub')
    expect(raw).not.toContain('onHome=')
    expect(raw).not.toContain('onOpenHub=')
    expect(raw).not.toContain('activePanel=')
  })
})

// Phase 138 / NAV-02: Remote sidebar item removed.
// App.tsx must NOT contain Remote Sessions wiring anymore.
describe('NAV-02: Remote sidebar item is removed (Phase 138)', () => {
  it('does NOT wire onOpenRemoteSessions to Sidebar', () => {
    expect(raw).not.toContain('onOpenRemoteSessions=')
  })

  it('does NOT contain t.type === remote-sessions routing', () => {
    expect(raw).not.toContain("t.type === 'remote-sessions'")
  })
})

// Phase 138 / NAV-03: Sessions sidebar item removed (DaemonManagerPanel retired).
// App.tsx must NOT contain Sessions/Daemon Manager wiring anymore.
describe('NAV-03: Sessions sidebar item is removed (Phase 138)', () => {
  it('does NOT wire onOpenDaemonManager to Sidebar', () => {
    expect(raw).not.toContain('onOpenDaemonManager=')
  })

  it('does NOT contain t.type === daemon-manager routing', () => {
    expect(raw).not.toContain("t.type === 'daemon-manager'")
  })
})

// Phase 138 / NAV-04: New Session sidebar item removed — creation lives solely on Hub's HubFilterBar.
// App.tsx must NOT pass onAdd={handleAddTab} to Sidebar, and handleAddTab must be gone.
describe('NAV-04: New Session sidebar item is removed (Phase 138)', () => {
  it('does NOT wire onAdd={handleAddTab} to Sidebar', () => {
    expect(raw).not.toContain('onAdd={handleAddTab}')
  })

  it('does NOT define handleAddTab callback', () => {
    expect(raw).not.toContain('const handleAddTab')
  })
})

// The Settings tab stays (opened from the menu) until plan 02-11 replaces it with a sheet.
describe('Settings tab (menu:settings) until plan 02-11', () => {
  it('does not wire Settings through the Sidebar', () => {
    expect(slice('<Sidebar', '/>')).not.toContain('onSettings')
  })

  it('find-or-create pattern checks t.type === settings', () => {
    expect(raw).toContain("t.type === 'settings'")
  })

  it('uses SETTINGS_TAB constant', () => {
    expect(raw).toContain('SETTINGS_TAB')
  })

  it('renders SettingsTab component', () => {
    expect(raw).toContain('<SettingsTab')
  })
})

describe('TAB-01: Tab bar has no action buttons', () => {
  it('TabBar JSX block does not receive onAdd prop', () => {
    // Extract the TabBar JSX block from the raw source
    const tabBarStart = raw.indexOf('<TabBar')
    const tabBarEnd = raw.indexOf('/>', tabBarStart)
    expect(tabBarStart).toBeGreaterThan(-1)
    const tabBarBlock = raw.slice(tabBarStart, tabBarEnd + 2)
    expect(tabBarBlock).not.toContain('onAdd')
  })

  it('TabBar JSX block does not receive onSettings prop', () => {
    const tabBarStart = raw.indexOf('<TabBar')
    const tabBarEnd = raw.indexOf('/>', tabBarStart)
    expect(tabBarStart).toBeGreaterThan(-1)
    const tabBarBlock = raw.slice(tabBarStart, tabBarEnd + 2)
    expect(tabBarBlock).not.toContain('onSettings')
  })

  it('TabBar JSX block does not receive onOpenDaemonManager prop', () => {
    const tabBarStart = raw.indexOf('<TabBar')
    const tabBarEnd = raw.indexOf('/>', tabBarStart)
    expect(tabBarStart).toBeGreaterThan(-1)
    const tabBarBlock = raw.slice(tabBarStart, tabBarEnd + 2)
    expect(tabBarBlock).not.toContain('onOpenDaemonManager')
  })
})
