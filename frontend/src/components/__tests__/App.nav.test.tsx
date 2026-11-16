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
    // Review WR-06: a poll in flight across an exit must not resurrect the session.
    expect(body).toContain('setSessions(list.filter((s) => !exitedIdsRef.current.has(s.id)))')
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

  it('wires the sidebar New Session to the New Session dialog (quick 261116-c9f)', () => {
    expect(slice('<Sidebar', '/>')).toContain('onNewSession={() => setShowNewSessionModal(true)}')
  })

  it('gates the Sidebar on mode !== web (WEBCHAT-03 guest scope)', () => {
    const idx = raw.indexOf('<Sidebar')
    expect(raw.slice(idx - 60, idx)).toMatch(/\{mode !== 'web' && [^(]*\(\s*$/)
  })

  it('handleOpenSessionTab focuses an open tab or appends and activates one', () => {
    const body = slice('const handleOpenSessionTab = useCallback(', '}, [tabs])')
    expect(body.indexOf('if (exitedIdsRef.current.has(sessionId)) return')).toBeGreaterThan(-1)
    expect(body.indexOf('if (exitedIdsRef.current.has(sessionId)) return')).toBeLessThan(body.indexOf('setTabs('))
    expect(body).toContain('tabs.find((t) => t.id === sessionId)')
    expect(body).toContain('setActiveId(existing.id)')
    expect(body).toContain('setTabs((prev) => [...prev, newTab])')
    expect(body).toContain('setActiveId(newTab.id)')
  })
})

// Review WR-02: the tray reopens a detached session (no tab) instead of doing nothing.
describe('tray:focus-session opens or focuses a tab', () => {
  it('skips exited sessions, focuses an open tab, otherwise appends one', () => {
    const body = slice("EventsOn('tray:focus-session'", '\n    })')
    expect(body).toContain('if (exitedIdsRef.current.has(sessionId)) return')
    expect(body).toContain('setActiveId(tab ? tab.id : sessionId)')
    expect(body).toContain("[...prev, { id: sessionId, name: 'shell', sessionId }]")
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

// Phase 138 / NAV-04: creation goes through the New Session dialog, opened by ⌘T, the toolbar "+",
// the empty state and, since quick 261116-c9f, the sidebar's This Mac "+" and empty-group row, all
// via setShowNewSessionModal(true). The old onAdd={handleAddTab} path must stay gone.
describe('NAV-04: the old onAdd/handleAddTab sidebar path stays removed (Phase 138)', () => {
  it('does NOT wire onAdd={handleAddTab} to Sidebar', () => {
    expect(raw).not.toContain('onAdd={handleAddTab}')
  })

  it('does NOT define handleAddTab callback', () => {
    expect(raw).not.toContain('const handleAddTab')
  })
})

// Portty 02-04 NAV-03: App renders only session tabs (plus Settings until 02-11).
describe('Portty NAV-03: no Hub, Welcome or Help tabs', () => {
  it('has no __hub__, __welcome__ or __help__ tab ids', () => {
    for (const id of ['__hub__', '__welcome__', '__help__']) expect(raw).not.toContain(id)
  })

  it('does not import or render HubPanel, WelcomeTab or HelpTab', () => {
    for (const c of ['HubPanel', 'WelcomeTab', 'HelpTab']) {
      expect(raw).not.toMatch(new RegExp(`import \\{ ${c} \\}`))
      expect(raw).not.toContain(`<${c}`)
    }
  })

  it('has no Hub group state or Hub cap callbacks', () => {
    for (const s of ['groupDefs', 'activeGroupId', 'capAcquiredRef', 'capCancelledRef', 'hubError', "'hub-modal'"]) {
      expect(raw).not.toContain(s)
    }
  })

  it("the Share dialog's help link opens HELP_URL in the browser", () => {
    expect(raw).toContain("import { HELP_URL } from './lib/helpUrl'")
    expect(raw).toContain('onOpenHelp={() => BrowserOpenURL(HELP_URL)}')
    expect(raw).not.toContain('handleOpenHelp')
  })
})

// Portty 02-04: Hide/Show Sidebar and the inactive-window selection colour.
describe('Portty 02-04: sidebar hide/show and inactive window', () => {
  it('subscribes menu:toggle-sidebar with a functional update and unsubscribes', () => {
    expect(raw).toContain("EventsOn('menu:toggle-sidebar', () => setSidebarHidden((h) => !h))")
    expect(raw).toContain('offMenuToggleSidebar()')
  })

  it('does not render the Sidebar while hidden; TabBar gets the inset and Show Sidebar', () => {
    const idx = raw.indexOf('<Sidebar')
    expect(raw.slice(idx - 60, idx)).toContain("mode !== 'web' && !sidebarHidden && (")
    expect(slice('<Sidebar', '/>')).toContain('onHide={() => setSidebarHidden(true)}')
    const tabBar = slice('<TabBar', '/>')
    expect(tabBar).toContain("sidebarHidden={mode !== 'web' && sidebarHidden}")
    expect(tabBar).toContain('onShowSidebar={() => setSidebarHidden(false)}')
  })

  it('sidebarHidden is plain state, not persisted', () => {
    expect(raw).toContain('const [sidebarHidden, setSidebarHidden] = useState(false)')
    expect(raw).not.toMatch(/localStorage\.[a-zA-Z]+\([^)]*sidebar/i)
  })

  it('toggles html.is-inactive on window blur/focus and removes both listeners (desktop only)', () => {
    const body = slice("const onBlur = () => root.classList.add('is-inactive')", '}, [mode])')
    expect(body).toContain("const onFocus = () => root.classList.remove('is-inactive')")
    expect(body).toContain("window.addEventListener('blur', onBlur)")
    expect(body).toContain("window.addEventListener('focus', onFocus)")
    expect(body).toContain("window.removeEventListener('blur', onBlur)")
    expect(body).toContain("window.removeEventListener('focus', onFocus)")
    const start = raw.lastIndexOf('useEffect(() => {', raw.indexOf("const onBlur = () =>"))
    expect(raw.slice(start, raw.indexOf("const onBlur = () =>"))).toContain("if (mode === 'web') return")
  })
})

// Portty 02-11: Settings is a sheet opened from the menu, never a tab.
describe('Settings sheet (menu:settings)', () => {
  it('does not wire Settings through the Sidebar', () => {
    expect(slice('<Sidebar', '/>')).not.toContain('onSettings')
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
