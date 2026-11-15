import { describe, it, expect } from 'vitest'
import raw from '../../App.tsx?raw'
import { readFileSync } from 'fs'
import { resolve } from 'path'

const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

describe('App', () => {
  // UILAY-02: App.tsx imports and renders <StatusBar> inside terminal-wrapper
  describe('StatusBar integration (UILAY-02)', () => {
    it('imports StatusBar from components/StatusBar', () => {
      expect(raw).toContain("import { StatusBar } from './components/StatusBar'")
    })

    it('renders <StatusBar inside terminal-wrapper', () => {
      // Both TerminalPanel and StatusBar must appear inside the terminal-wrapper block
      const wrapperBlock = raw.slice(raw.indexOf('terminal-wrapper'))
      expect(wrapperBlock).toContain('<StatusBar')
    })

    it('passes required props to StatusBar', () => {
      expect(raw).toContain('webServerRunning={webServerRunning}')
      expect(raw).toContain('webEnabled={!!webEnabled[tab.sessionId]}')
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

    it('defines DEFAULT_FONT_SIZE = 13 (UI-SPEC terminal font)', () => {
      expect(raw).toContain('DEFAULT_FONT_SIZE = 13')
    })

    it('defines handleFontSizeChange callback', () => {
      expect(raw).toContain('handleFontSizeChange')
    })

    it('clamps font size between 6 and 32', () => {
      expect(raw).toContain('Math.max(6')
      expect(raw).toContain('Math.min(32')
    })

    it('passes fontSize prop to TerminalPanel', () => {
      expect(raw).toContain('fontSize=')
    })

    it('passes onFontSizeChange prop to TerminalPanel', () => {
      expect(raw).toContain('onFontSizeChange=')
    })

    it('cleans up fontSizes entry on tab close', () => {
      expect(raw).toContain('setFontSizes')
      // Cleanup pattern: delete n[id] in handleCloseTab
      const closeSection = raw.slice(raw.indexOf('handleCloseTab'))
      expect(closeSection).toContain('setFontSizes')
    })
  })

  describe('Tailscale health integration (NET-01)', () => {
    it('imports GetTailscaleStatus from wailsjs bindings', () => {
      expect(raw).toContain('GetTailscaleStatus')
    })

    it('imports TailscaleBanner and the shared TailscaleHealth type', () => {
      expect(raw).toContain("import { TailscaleBanner } from './components/TailscaleBanner'")
      expect(raw).toMatch(/import \{ [^}]*\bTailscaleHealth\b[^}]*\} from '\.\/lib\/tailscale'/)
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

    it('passes tailscaleHealth prop to SettingsTab', () => {
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
      expect(raw).toContain("const terminalTheme = useColorScheme() === 'dark' ? DARK_PALETTE : LIGHT_PALETTE")
    })

    it('passes theme prop to TerminalPanel', () => {
      expect(raw).toContain('theme={terminalTheme}')
    })

    it('has no theme library, theme storage keys or data-ui-theme writes', () => {
      expect(raw).not.toContain("from 'xterm-theme'")
      expect(raw).not.toContain('agenthub:terminalTheme')
      expect(raw).not.toContain('agenthub:uiTheme')
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

  it('imports UpdateInfo type', () => {
    expect(raw).toContain("import type { UpdateInfo } from './components/UpdateBanner'")
  })

  it('imports GetLastUpdateInfo from wailsjs bindings', () => {
    expect(raw).toContain('GetLastUpdateInfo')
  })

  it('declares update state at App level', () => {
    expect(raw).toContain('useState<UpdateInfo | null>(null)')
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

  it('subscribes to menu:settings through a ref to handleOpenSettings (Portty 02-01)', () => {
    expect(raw).toContain("EventsOn('menu:settings', () => openSettingsRef.current?.())")
    expect(raw).toContain('openSettingsRef.current = handleOpenSettings')
    expect(raw).toContain('offMenuSettings()')
  })

  it('does not wire menu:close-tab yet: closing a tab still kills the shell (T-02-04)', () => {
    expect(raw).not.toContain("EventsOn('menu:close-tab'")
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

  it('subscribes to update:available event', () => {
    expect(raw).toContain("EventsOn('update:available'")
  })

  it('passes onDismiss to UpdateBanner', () => {
    expect(raw).toContain('onDismiss={handleDismissUpdate}')
  })

  it('passes className with banner-exit for exit animation', () => {
    expect(raw).toContain("updateExiting ? 'banner-exit' : undefined")
  })

})

// TERM-01: the modal passes only the directory; sessions are always the login shell
describe('TERM-01: createTab threading', () => {
  it('onConfirm passes only the directory to createTab', () => {
    expect(raw).toContain('createTab(workDir)')
  })
  it('createTab calls CreateSession without cli or args', () => {
    expect(raw).toContain('CreateSession(defaultName, workDir, cols, rows)')
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

// Portty 02-05 (ROADMAP SC5): the Share dialog moved out of components/Hub/ and
// is still rendered exactly once at App level.
describe('Share dialog lives in components/Share (Portty 02-05)', () => {
  it('imports SessionShareModal from components/Share', () => {
    expect(raw).toContain("import { SessionShareModal } from './components/Share/SessionShareModal'")
  })

  it('renders exactly one SessionShareModal', () => {
    expect(raw.match(/<SessionShareModal\b/g)).toHaveLength(1)
  })
})
