import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'
import { readFileSync } from 'fs'
import { resolve } from 'path'

const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

describe('App', () => {
  // Portty 02-08: the bottom status bar is gone; Share lives in the toolbar.
  describe('toolbar Share replaces the status bar (ROADMAP SC5)', () => {
    it('neither imports nor renders StatusBar', () => {
      expect(raw).not.toContain('StatusBar')
    })

    // Portty 04-05: the button toggles the Share popover, anchored on it.
    it('TabBar toggles the Share popover and gets the share and NET-01 state', () => {
      const tabBar = raw.slice(raw.indexOf('<TabBar'), raw.indexOf('/>', raw.indexOf('<TabBar')))
      expect(tabBar).toContain('onShare={() => setSharePopoverOpen((open) => !open)}')
      expect(tabBar).toContain('shareOpen={sharePopoverOpen}')
      expect(tabBar).toContain('sharePopoverId={SHARE_POPOVER_ID}')
      expect(tabBar).toContain('shareButtonRef={shareButtonRef}')
      expect(raw).toContain('const activeShared = !!sessions.find((s) => s.id === activeId)?.webEnabled')
      expect(tabBar).toContain('shared={activeShared}')
      // 04-09: a shared session's button stays enabled with Tailscale off (App.tailscale.test.tsx).
      expect(tabBar).toContain('shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}')
    })

    it('renders one SharePopover, for the active local session, reporting toggles into the session list', () => {
      expect(raw).toContain("import { SharePopover } from './components/SharePopover'")
      expect(raw.match(/<SharePopover\b/g)).toHaveLength(1)
      const idx = raw.indexOf('<SharePopover')
      const block = raw.slice(raw.lastIndexOf('{(() => {', idx), raw.indexOf('/>', idx))
      expect(block).toContain('sharePopoverOpen ? sessions.find((s) => s.id === activeId) : undefined')
      expect(block).toContain('anchor={shareButtonRef.current}')
      expect(block).toContain('onSharedChange={setSessionWebEnabled}')
      expect(block).toContain('tailscaleOff={tailscaleOff}')
    })

    it('a tab switch and opening Settings close the popover', () => {
      expect(raw).toContain('useEffect(() => { setSharePopoverOpen(false) }, [activeId])')
      expect(raw).toContain('useEffect(() => { if (showSettings) setSharePopoverOpen(false) }, [showSettings])')
    })
  })

  // Portty 02-11 (NAV-03, LOOK-01): Settings is a sheet, never a tab.
  describe('Settings sheet replaces the Settings tab (NAV-03)', () => {
    it('has no settings tab id, constant or tab type', () => {
      expect(raw).not.toMatch(/SETTINGS_TAB|__settings__|type === 'settings'|type !== 'settings'/)
    })

    it('menu:settings sets showSettings', () => {
      expect(raw).toContain('const [showSettings, setShowSettings] = useState(false)')
      expect(raw).toContain("EventsOn('menu:settings', () => setShowSettings(true))")
    })

    it('renders exactly one SettingsSheet, desktop only, closed by onClose', () => {
      expect(raw.match(/<SettingsSheet\b/g)).toHaveLength(1)
      const start = raw.indexOf('<SettingsSheet')
      expect(raw.slice(start - 40, start)).toContain("mode !== 'web' && (")
      const props = raw.slice(start, start + 200)
      expect(props).toContain('isOpen={showSettings}')
      expect(props).toContain('onClose={() => setShowSettings(false)}')
    })
  })

  // Portty 02-13 (THEM-01 amended): Settings ▸ Appearance override, desktop only.
  describe('Appearance override (THEM-01)', () => {
    it('loads GetAppearance only after the web-mode early return in init', () => {
      const webReturn = raw.indexOf("if (mode === 'web') {\n        return\n      }")
      const call = raw.indexOf('GetAppearance()')
      expect(webReturn).toBeGreaterThan(-1)
      expect(call).toBeGreaterThan(webReturn)
      expect(raw.match(/GetAppearance\(\)/g)).toHaveLength(2)
    })

    // Review WR-03: Retry Connection hydrates the override too, after RetryDaemon succeeds.
    it('retryInit loads GetAppearance after RetryDaemon', () => {
      const start = raw.indexOf('const retryInit = useCallback(')
      const body = raw.slice(start, raw.indexOf('}, [])', start))
      expect(body.indexOf('GetAppearance()')).toBeGreaterThan(body.indexOf('await RetryDaemon()'))
    })

    it("passes 'system' to useColorScheme in web mode, the override otherwise", () => {
      expect(raw).toContain("useColorScheme(mode === 'web' ? 'system' : appearance)")
      expect(raw.match(/useColorScheme\(/g)).toHaveLength(1)
    })

    it('wires onAppearanceChange to the appearance state', () => {
      const start = raw.indexOf('<SettingsSheet')
      expect(raw.slice(start, raw.indexOf('/>', start))).toContain('onAppearanceChange={setAppearance}')
    })

    it('adds no data-appearance fallback (WKWebView follows NSApp.appearance, M-68)', () => {
      expect(raw).not.toContain('data-appearance')
      expect(raw).not.toContain('dataset.appearance')
      expect(cssRaw).not.toContain('data-appearance')
    })
  })

  // UILAY-03: Old web-serving-bar overlay has been removed from App.tsx
  describe('old overlay removed (UILAY-03)', () => {
    it('does not contain web-serving-bar class', () => {
      expect(raw).not.toContain('web-serving-bar')
    })

    it('does not contain web-toggle-btn class', () => {
      expect(raw).not.toContain('web-toggle-btn')
    })

    it('does not contain web-session-url class', () => {
      expect(raw).not.toContain('web-session-url')
    })

    it('does not contain copy-token-btn class', () => {
      expect(raw).not.toContain('copy-token-btn')
    })
  })

  describe('per-tab font size state', () => {
    it('declares fontSizes state as Record<string, number>', () => {
      expect(raw).toContain('Record<string, number>')
      expect(raw).toContain('fontSizes')
    })

    it('defaults the terminal font to the lib constant (13) and zoom starts from the setting', () => {
      expect(raw).toContain("import { DEFAULT_TERMINAL_FONT_SIZE, terminalFontFamily, type TerminalFont } from './lib/terminalFont'")
      expect(raw).toContain("useState<TerminalFont>({ family: '', size: DEFAULT_TERMINAL_FONT_SIZE })")
      expect(raw).not.toContain('DEFAULT_FONT_SIZE = 13')
    })

    it('defines handleFontSizeChange starting from terminalFont.size', () => {
      const start = raw.indexOf('const handleFontSizeChange = useCallback(')
      const body = raw.slice(start, raw.indexOf('}, [terminalFont.size])', start))
      expect(body).toContain('prev[sessionId] ?? terminalFont.size')
    })

    it('clamps font size between 6 and 32', () => {
      expect(raw).toContain('Math.max(6')
      expect(raw).toContain('Math.min(32')
    })

    it('passes the built family stack and a fontSize falling back to the setting to TerminalPanel', () => {
      expect(raw).toContain('useMemo(() => terminalFontFamily(terminalFont.family), [terminalFont.family])')
      expect(raw).toContain('fontFamily={terminalFontStack}')
      expect(raw).toContain('fontSize={fontSizes[tab.sessionId] ?? terminalFont.size}')
    })

    // Quick 261116-ku9 R1: Terminal Font loads at init and on reconnect, and Settings updates it.
    it('loads GetTerminalFont at init and on reconnect, and wires onTerminalFontChange', () => {
      expect(raw.match(/GetTerminalFont\(\)/g)).toHaveLength(2)
      const start = raw.indexOf('const retryInit = useCallback(')
      expect(raw.slice(start, raw.indexOf('}, [])', start))).toContain('GetTerminalFont()')
      expect(raw).toContain('onTerminalFontChange={setTerminalFont}')
    })

    it('passes onFontSizeChange prop to TerminalPanel', () => {
      expect(raw).toContain('onFontSizeChange=')
    })

    it('cleans up fontSizes entry on tab close', () => {
      // removeTabState backs both closeTab and session:exit.
      const body = raw.slice(raw.indexOf('const removeTabState = useCallback('), raw.indexOf('removeTabStateRef.current = removeTabState'))
      expect(body).toContain('setFontSizes')
    })
  })

  describe('Tailscale health integration (NET-01)', () => {
    it('imports GetTailscaleStatus from wailsjs bindings', () => {
      expect(raw).toContain('GetTailscaleStatus')
    })

    it('imports TailscaleBanner and the shared TailscaleHealth type', () => {
      expect(raw).toContain("import { TailscaleBanner } from './components/TailscaleBanner'")
      expect(raw).toMatch(/import (type )?\{ [^}]*\bTailscaleHealth\b[^}]*\} from '\.\/lib\/tailscale'/)
    })

    it('no longer references the multi-state LocalNetworkBanner', () => {
      expect(raw).not.toContain('LocalNetworkBanner')
    })

    it('calls GetTailscaleStatus in Promise.all init', () => {
      expect(raw).toContain('GetTailscaleStatus()')
    })

    it('subscribes to tailscale:health event', () => {
      expect(raw).toContain("EventsOn('tailscale:health'")
    })

    it('cleans up health event listener', () => {
      expect(raw).toContain('offHealth()')
    })

    it('derives tailscaleOff from a non-null health that is not connected', () => {
      expect(raw).toContain('const tailscaleOff = tailscaleHealth !== null && !tailscaleHealth.connected')
    })

    it('passes tailscaleHealth prop to SettingsSheet', () => {
      expect(raw).toContain('tailscaleHealth={tailscaleHealth}')
    })

    it('types tailscaleHealth with the slim shared type', () => {
      expect(raw).toContain('useState<TailscaleHealth | null>(null)')
      expect(raw).not.toMatch(/binaryFound|daemonUp|platformHint|permissionLimited/)
    })
  })

  describe('THM-01: terminal palette follows the system appearance', () => {
    it('imports useColorScheme and both built-in palettes', () => {
      expect(raw).toContain("import { useColorScheme } from './lib/useColorScheme'")
      expect(raw).toContain("import { LIGHT_PALETTE, DARK_PALETTE } from './lib/terminalPalettes'")
    })

    it('derives terminalTheme from the colour scheme', () => {
      expect(raw).toContain("const terminalTheme = useColorScheme(mode === 'web' ? 'system' : appearance) === 'dark' ? DARK_PALETTE : LIGHT_PALETTE")
    })

    it('passes theme prop to TerminalPanel', () => {
      expect(raw).toContain('theme={terminalTheme}')
    })

    it('has no theme library, theme storage keys or data-ui-theme writes', () => {
      expect(raw).not.toContain("from 'xterm-theme'")
      expect(raw).not.toMatch(/['"][a-z]+:(terminalTheme|uiTheme)['"]/)
      expect(raw).not.toContain('THEME_STORAGE_KEY')
      expect(raw).not.toContain('data-ui-theme')
      expect(raw).not.toContain('style.colorScheme')
      expect(raw).not.toContain('selectedTheme=')
    })
  })

  describe('daemon error handling (Phase 26)', () => {
    it('imports RetryDaemon from wailsjs bindings', () => {
      expect(raw).toContain('RetryDaemon')
      expect(raw).toContain("from './wailsjs/go/main/App'")
    })

    it('subscribes to daemon:error event on mount', () => {
      expect(raw).toContain("EventsOn('daemon:error'")
    })

    it('unsubscribes from daemon:error in cleanup', () => {
      expect(raw).toContain('offDaemonError()')
    })

    it('retryInit calls RetryDaemon before other methods', () => {
      // RetryDaemon must appear before Promise.all in retryInit
      const retryBlock = raw.slice(raw.indexOf('const retryInit'))
      const retryDaemonPos = retryBlock.indexOf('await RetryDaemon()')
      const promiseAllPos = retryBlock.indexOf('Promise.all')
      expect(retryDaemonPos).toBeGreaterThan(-1)
      expect(promiseAllPos).toBeGreaterThan(-1)
      expect(retryDaemonPos).toBeLessThan(promiseAllPos)
    })

    it('renders daemonError directly in banner (not hardcoded message)', () => {
      // The banner body should use {daemonError} not a static string
      expect(raw).toContain('{daemonError}')
      expect(raw).not.toContain('The background daemon did not start in time')
    })

    it('imports GetDaemonError from wailsjs bindings', () => {
      expect(raw).toContain('GetDaemonError')
    })

    it('calls GetDaemonError before Promise.all in init', () => {
      const initBlock = raw.slice(raw.indexOf('async function init()'))
      const getDaemonPos = initBlock.indexOf('GetDaemonError()')
      const promiseAllPos = initBlock.indexOf('Promise.all')
      expect(getDaemonPos).toBeGreaterThan(-1)
      expect(promiseAllPos).toBeGreaterThan(-1)
      expect(getDaemonPos).toBeLessThan(promiseAllPos)
    })
  })
})

describe('TERM-04: no per-agent theme broadcast', () => {
  it('no per-agent (OpenCode SIGUSR2) theme broadcast remains', () => {
    expect(raw).not.toContain('NotifyThemeChange')
  })
})

describe('BannerStack integration (BAN-01, BAN-02)', () => {
  it('imports UpdateBanner component', () => {
    expect(raw).toContain("import { UpdateBanner } from './components/UpdateBanner'")
  })

  it('imports UpdateStatus type', () => {
    expect(raw).toContain("import type { UpdateStatus } from './components/UpdateBanner'")
  })

  it('imports GetUpdateStatus and DownloadUpdate from wailsjs bindings, not the old update bindings (06-03)', () => {
    expect(raw).toContain('GetUpdateStatus')
    expect(raw).toContain('DownloadUpdate()')
    expect(raw).not.toMatch(/GetLastUpdateInfo|CheckForUpdates|update:available/)
  })

  it('declares update state at App level', () => {
    expect(raw).toContain('useState<UpdateStatus | null>(null)')
  })

  it('declares updateExiting state', () => {
    expect(raw).toContain('updateExiting')
  })

  it('has no dismiss path for the Tailscale banner (persistent, NET-01)', () => {
    expect(raw).not.toContain('localBannerDismissed')
    expect(raw).not.toContain('handleDismissLocalBanner')
  })

  it('defines handleDismissUpdate callback', () => {
    expect(raw).toContain('handleDismissUpdate')
  })

  it('renders banner-stack div', () => {
    expect(raw).toContain('banner-stack')
  })

  it('renders banner-stack inside the main column after the TabBar (Portty 02-01)', () => {
    expect(raw.indexOf('<div className="banner-stack">')).toBeGreaterThan(raw.indexOf('<TabBar'))
  })

  it('subscribes to menu:new-session and opens the New Session dialog (Portty 02-01)', () => {
    expect(raw).toContain("EventsOn('menu:new-session', () => setShowNewSessionModal(true))")
  })

  it('subscribes to menu:settings and opens the Settings sheet (Portty 02-01, 02-11)', () => {
    expect(raw).toContain("EventsOn('menu:settings', () => setShowSettings(true))")
    expect(raw).toContain('offMenuSettings()')
  })

  it('subscribes to menu:close-tab: closes the active tab through activeIdRef, no-op with none (Portty 02-08)', () => {
    const start = raw.indexOf("EventsOn('menu:close-tab'")
    expect(start).toBeGreaterThan(-1)
    const handler = raw.slice(start, raw.indexOf('})', start))
    expect(handler).toContain('activeIdRef.current')
    expect(handler).toContain('if (id !== null) removeTabStateRef.current?.(id)')
    expect(handler).not.toMatch(/KillSession|SetSessionShared/)
    expect(raw).toContain('offMenuCloseTab()')
    expect(raw).toContain('activeIdRef.current = activeId')
  })

  it('renders TailscaleBanner inside banner-stack, gated on tailscaleOff', () => {
    const stackBlock = raw.slice(raw.indexOf('banner-stack'))
    expect(stackBlock).toContain('{tailscaleOff && <TailscaleBanner />}')
  })

  it('includes tailscaleOff in the banner-stack gating expression (INVARIANT)', () => {
    const invariant = raw.indexOf('INVARIANT: this gating expression')
    const gate = raw.slice(invariant, raw.indexOf('<div className="banner-stack">', invariant))
    expect(gate).toContain('tailscaleOff ||')
  })

  it('renders UpdateBanner inside banner-stack', () => {
    const stackBlock = raw.slice(raw.indexOf('banner-stack'))
    expect(stackBlock).toContain('<UpdateBanner')
  })

  it('subscribes to update:status event', () => {
    expect(raw).toContain("EventsOn('update:status'")
  })

  it('passes onDismiss to UpdateBanner', () => {
    expect(raw).toContain('onDismiss={handleDismissUpdate}')
  })

  it('passes className with banner-exit for exit animation', () => {
    expect(raw).toContain("updateExiting ? 'banner-exit' : undefined")
  })

  // Portty 02-10 (UI-SPEC "Banner"): Tailscale, WebGL recovery, update, plugin toggle, then save feedback.
  it('renders the banner stack in the UI-SPEC order', () => {
    const start = raw.indexOf('<div className="banner-stack">')
    const block = raw.slice(start, raw.indexOf('<div className="terminal-container">', start))
    const order = [
      '<TailscaleBanner />',
      '<WebGLRecoveryBanner',
      '<UpdateBanner',
      '<PluginToggleBanner',
      '{saveBanner && (',
    ].map((marker) => block.indexOf(marker))
    expect(order.every((i) => i > -1)).toBe(true)
    expect([...order].sort((a, b) => a - b)).toEqual(order)
  })

  it('the gating expression still covers every banner trigger (INVARIANT, T-02-41)', () => {
    const invariant = raw.indexOf('INVARIANT: this gating expression')
    const gate = raw.slice(invariant, raw.indexOf('<div className="banner-stack">', invariant))
    for (const trigger of [
      'tailscaleOff',
      'showUpdateBanner',
      '(webglContextLost || webglSoftwareDetected) && !webglBannerDismissed',
      'saveBanner !== null',
      'pluginToggleBanners.length > 0',
    ]) {
      expect(gate).toContain(trigger)
    }
  })

  it('the save-feedback banner uses the shared style with an XMarkIcon Dismiss', () => {
    const start = raw.indexOf('{saveBanner && (')
    const block = raw.slice(start, raw.indexOf('</div>', start))
    expect(block).toContain("'banner banner--warning' : 'banner banner--info'")
    expect(block).toContain('className="banner__dismiss"')
    expect(block).toContain('<XMarkIcon aria-hidden="true" />')
    expect(block).not.toContain('>×<')
  })

  it('the daemon error notice is a warning banner keeping its message and Retry action', () => {
    const start = raw.indexOf("{daemonError && tabs.length === 0 && (")
    const block = raw.slice(start, raw.indexOf('Retry Connection', start))
    expect(block).toContain('className="banner banner--warning"')
    expect(block).toContain('<ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />')
    expect(block).toContain('{daemonError}')
    expect(block).toContain('onClick={retryInit}')
  })

  it('has no inline style with a hex colour and no hover colour handler (LOOK-01)', () => {
    expect(raw).not.toMatch(/style=\{\{[^}]*#[0-9a-fA-F]{3,6}/)
    expect(raw).not.toMatch(/#[0-9a-fA-F]{6}\b/)
    expect(raw).not.toContain('onMouseEnter')
  })

})

// TERM-01: the modal passes only the directory; sessions are always the login shell
describe('TERM-01: createTab threading', () => {
  it('onConfirm passes only the directory to createTab', () => {
    expect(raw).toContain('createTab(workDir)')
  })
  it('createTab calls CreateSession without cli or args', () => {
    expect(raw).toContain('CreateSession(defaultName, workDir, grid?.cols ?? 0, grid?.rows ?? 0)')
  })

  // Quick 261116-ku9 R2: the PTY spawns at the grid the new xterm will fit to.
  it('createTab measures the grid with the current terminal font and threads it to the tab', () => {
    const start = raw.indexOf('const createTab = useCallback(')
    const body = raw.slice(start, raw.indexOf('}, [tabCounter, terminalFontStack, terminalFont.size])', start))
    expect(body).toContain('measureInitialGrid(container, terminalFontStack, terminalFont.size)')
    expect(body).toContain('if (grid) spawnGridRef.current[sessionId] = grid')
    expect(body.indexOf('spawnGridRef.current[sessionId] = grid')).toBeLessThan(body.indexOf('setTabs('))
    expect(raw).not.toMatch(/clientWidth \/ 8|clientHeight \/ 17|cols = 220/)
    expect(raw).toContain('initialCols={spawnGridRef.current[tab.sessionId]?.cols}')
    expect(raw).toContain('initialRows={spawnGridRef.current[tab.sessionId]?.rows}')
    const remove = raw.slice(raw.indexOf('const removeTabState = useCallback('), raw.indexOf('removeTabStateRef.current = removeTabState'))
    expect(remove).toContain('delete spawnGridRef.current[id]')
  })
  it('default tab name is shell {n}', () => {
    expect(raw).toContain('const defaultName = `shell ${tabCounter}`')
  })
})

// Phase 98 PRG-02/PRG-03 — Wave 0 RED scaffold (progress-debounce integration).
// This describe block is RED until Wave 2 (Plan 03 Task 2) wires handleProgressChange
// with 200ms debounce + SetTrayProgress dispatch into App.tsx.
// Tagged `progress-debounce` for VALIDATION.md verify targeting.
describe('Phase 98 PRG-02/PRG-03: App.tsx progress-debounce integration — Wave 0 RED scaffold', () => {
  it('progress-debounce: App.tsx imports SetTrayProgress from wailsjs bindings', () => {
    // RED until Wave 2 (Plan 03) adds the SetTrayProgress import.
    expect(raw).toContain('SetTrayProgress')
  })

  it('progress-debounce: App.tsx declares handleProgressChange callback', () => {
    // RED until Wave 2. handleProgressChange receives (sessionId, IProgressState)
    // and feeds the progressRegistry + schedules a debounced SetTrayProgress call.
    expect(raw).toContain('handleProgressChange')
  })

  it('progress-debounce: App.tsx declares trayDebounceRef for the 200ms debounce timer', () => {
    // RED until Wave 2. The debounce ref stores the setTimeout handle so it can
    // be cleared on rapid progress updates (Pattern 4 in 98-RESEARCH.md).
    expect(raw).toContain('trayDebounceRef')
  })

  it('progress-debounce: App.tsx passes onProgressChange prop to TerminalPanel', () => {
    // RED until Wave 2 wires onProgressChange={handleProgressChange} on <TerminalPanel>.
    expect(raw).toContain('onProgressChange={handleProgressChange}')
  })

  it('progress-debounce: App.tsx uses 200ms debounce window before SetTrayProgress dispatch', () => {
    // RED until Wave 2. The debounce timeout must be 200 (ms) to avoid
    // flooding the Go side with progress events during rapid terminal output.
    // Source-level assertion: 200 must appear near the trayDebounceRef assignment.
    const debounceIdx = raw.indexOf('trayDebounceRef')
    if (debounceIdx >= 0) {
      const debounceWindow = raw.slice(debounceIdx, debounceIdx + 500)
      expect(debounceWindow).toContain('200')
    } else {
      // trayDebounceRef not yet present — test fails RED as expected at Wave 0.
      expect(raw).toContain('trayDebounceRef')
    }
  })
})

// Portty 02-04 NAV-03: launch has no Welcome tab (WelcomeTab itself goes in 02-04 Task 3 / 02-05).
describe('launch state (NAV-03)', () => {
  it('initial tabs hold no Welcome tab', () => {
    expect(raw).toContain('useState<Tab[]>([])')
    expect(raw).not.toMatch(/\[WELCOME_TAB\]/)
    expect(raw).not.toContain('WELCOME_TAB.id,')
  })

  it('hides static HTML splash on mount', () => {
    expect(raw).toContain("getElementById('splash-static')")
  })
})

// Portty 02-01 UAT defects A and B: desktop drag strips.
describe('desktop window chrome CSS (LOOK-01)', () => {
  it('the old 48px collapsed sidebar is gone (Portty 02-04)', () => {
    expect(cssRaw).not.toMatch(/(^|\s)\.sidebar--collapsed/m)
  })

  it('hidden sidebar: tab bar takes the 80px traffic-light inset (Portty 02-04)', () => {
    expect(cssRaw).toMatch(/html\.is-desktop \.tab-bar--sidebar-hidden \{\s*padding-left: 80px;/)
  })

  it('inactive window: selected sidebar row uses the inactive selection colour with text colour', () => {
    expect(cssRaw).toMatch(/html\.is-inactive \.sidebar__row\[aria-current="page"\] \{\s*background: var\(--color-selection-inactive\);\s*color: var\(--color-text\);/)
  })

  it('drag strips do not text-select (WebKit prefix included)', () => {
    expect(cssRaw).toMatch(/html\.is-desktop :is\(\.sidebar, \.tab-bar\) \{\s*-webkit-user-select: none;\s*user-select: none;/)
  })
})

// Portty 04-05 (SHAR-01): the old tabbed share dialog is gone; the popover replaces it.
describe('the tabbed share dialog is gone (Portty 04-05)', () => {
  it('imports nothing from components/Share or components/SessionShare', () => {
    expect(raw).not.toMatch(/components\/(Share|SessionShare)\//)
    expect(raw).not.toContain('shareModalSession')
  })
})

// Portty 02-07 (CHAT-01): WebShareSessionView moved out of components/Hub/.
describe('WebShareSessionView lives in components/ (Portty 02-07)', () => {
  it('imports WebShareSessionView from components/WebShareSessionView', () => {
    expect(raw).toContain("import { WebShareSessionView } from './components/WebShareSessionView'")
    expect(raw).not.toContain('components/Hub/')
  })
})

// Portty 02-08 (TERM-03): closing a tab only detaches.
describe('closeTab is detach-only (TERM-03, T-02-27)', () => {
  const closeTabBody = () => {
    const start = raw.indexOf('const closeTab = useCallback(')
    expect(start).toBeGreaterThan(-1)
    return raw.slice(start, raw.indexOf('}, [', start))
  }

  it('closeTab makes no session RPC', () => {
    expect(closeTabBody()).not.toMatch(/KillSession|SetSessionShared|RenameSession/)
    expect(closeTabBody()).toContain('removeTabState(id)')
  })

  it('TabBar onClose is closeTab', () => {
    const tabBar = raw.slice(raw.indexOf('<TabBar'), raw.indexOf('/>', raw.indexOf('<TabBar')))
    expect(tabBar).toContain('onClose={closeTab}')
  })

  it('removeTabState picks the next tab with nextActiveTabId', () => {
    const body = raw.slice(raw.indexOf('const removeTabState = useCallback('), raw.indexOf('removeTabStateRef.current = removeTabState'))
    expect(body).toContain('setActiveId((cur) => nextActiveTabId(prev, id, cur))')
  })

  it('createTab no longer subtracts a status bar height from the row estimate', () => {
    expect(raw).not.toContain('statusBarHeight')
  })
})

describe('empty state (LOOK-01)', () => {
  // Quick 261116-ku9 R4: one boolean gates EmptyState and hides the sidebar's New Session row.
  it('passes the same showEmptyState to Sidebar as hideNewSessionRow', () => {
    expect(raw.match(/const showEmptyState = /g)).toHaveLength(1)
    const sidebar = raw.slice(raw.indexOf('<Sidebar'), raw.indexOf('/>', raw.indexOf('<Sidebar')))
    expect(sidebar).toContain('hideNewSessionRow={showEmptyState}')
  })

  // Quick 261116-ku9 R1: in-app remote tabs use the viewer's font; web guests self-fetch the host's.
  // INPT-06: the desktop size is the peer tab's own (fontSizes by tab id, default terminalFont.size).
  it('passes the viewer font to WebShareSessionView only on the desktop', () => {
    const view = raw.slice(raw.indexOf('<WebShareSessionView'), raw.indexOf('/>', raw.indexOf('<WebShareSessionView')))
    expect(view).toContain("fontFamily={mode !== 'web' ? terminalFontStack : undefined}")
    expect(view).toContain("fontSize={mode !== 'web' && activeWebTab ? fontSizes[activeWebTab.id] ?? terminalFont.size : undefined}")
    expect(view).toContain("onFontSizeChange={mode !== 'web' && activeWebTab ? (delta) => handleFontSizeChange(activeWebTab.id, delta) : undefined}")
  })

  it('renders EmptyState in desktop mode once init has run and no tab is open', () => {
    expect(raw).toContain("import { EmptyState } from './components/EmptyState'")
    expect(raw).toContain("const showEmptyState = mode !== 'web' && relayPort !== null && tabs.length === 0")
    expect(raw).toContain('{showEmptyState && (')
    expect(raw).toContain('<EmptyState onNewSession={() => setShowNewSessionModal(true)} />')
  })
})

// Portty 02-08 (TERM-03): rename is optimistic on the session list and reverts on failure.
describe('renameSession and End Session (TERM-03)', () => {
  const body = (marker: string) => {
    const start = raw.indexOf(marker)
    expect(start, marker).toBeGreaterThan(-1)
    return raw.slice(start, raw.indexOf('}, [', start))
  }

  it('renameSession renames optimistically and restores the old name in its catch', () => {
    const b = body('const renameSession = useCallback(')
    expect(b).toContain('setSessions((prev) => renameInList(prev, id, name))')
    const catchIdx = b.indexOf('catch')
    expect(catchIdx).toBeGreaterThan(b.indexOf('await RenameSession(id, name)'))
    expect(b.slice(catchIdx)).toContain('setSessions((prev) => renameInList(prev, id, previous))')
  })

  it('TabBar labels come from the session list (tabLabel)', () => {
    expect(raw).toContain('tabs={tabs.map((t) => ({ ...t, name: tabLabel(t, sessions) }))}')
  })

  // Portty 04-05: the daemon's kill path drops the share (04-02), so no separate stop call.
  it('endSession kills, then removes the tab', () => {
    const b = body('const endSession = useCallback(')
    const kill = b.indexOf('await KillSession(id)')
    expect(b).not.toContain('SetSessionShared')
    expect(kill).toBeGreaterThan(-1)
    expect(b.indexOf('removeTabState(id)')).toBeGreaterThan(kill)
    // Review WR-06: recorded as exited so an in-flight poll cannot resurrect it.
    expect(b.indexOf('exitedIdsRef.current.add(id)')).toBeGreaterThan(kill)
  })

  it('End Session… opens a destructive confirm with the UI-SPEC copy', () => {
    expect(raw).toContain('title={`End “${target?.name || \'shell\'}”?`}')
    expect(raw).toContain("The shell and everything running in it will stop. This can't be undone.")
    expect(raw).toContain('Anyone attached from another Mac or a browser will be disconnected.')
    expect(raw).toContain('confirmLabel="End Session"')
    expect(raw).toContain('titleId="end-session-title"')
    expect(raw).toMatch(/cancelLabel="Cancel"\s+destructive/)
  })

  it('the separate webEnabled map is gone; share state reads from sessions', () => {
    expect(raw).not.toContain('setWebEnabled')
    expect(raw).toContain('onSharedChange={setSessionWebEnabled}')
  })
})
