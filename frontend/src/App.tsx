import React, { useEffect, useState, useCallback, useRef, useMemo } from 'react'
import type { IProgressState } from '@xterm/addon-progress'
import { TabBar, type Tab } from './components/TabBar'
import { EmptyState } from './components/EmptyState'
import { nextActiveTabId, renameInList, tabLabel } from './lib/tabs'
import { RegenerateKeyModal } from './components/RegenerateKeyModal'
import { Sidebar } from './components/Sidebar'
import { SettingsSheet, type Appearance } from './components/SettingsSheet'
import { stripAnsi } from './lib/stripAnsi'
import { sanitizeFilename } from './lib/sanitizeFilename'
import { detectMode, readWebModeParams } from './lib/webMode'
import {
  CreateSession,
  ListSessions,
  KillSession,
  RenameSession,
  GetRelayPort,
  GetTailscaleStatus,
  SetSessionShared,
  RetryDaemon,
  GetDaemonError,
  GetRemoteSessionsWithMeta,
  ConnectPeerSession,
  GetLastUpdateInfo,
  GetPluginSettings,
  QuitGUIOnly,
  QuitAll,
  SaveTerminalSession,
  SetTrayProgress,
  GetAppearance,
  GetTerminalFont,
} from './wailsjs/go/main/App'
import { aggregateProgress } from './lib/aggregateProgress'
import type { SessionInfo, RemotePeerSessions } from './wailsjs/go/main/App'
import type { daemon } from './wailsjs/go/models'
type PluginSettings = daemon.PluginSettings
import { EventsOn } from './wailsjs/wailsjs/runtime/runtime'
import { NewSessionModal } from './components/NewSessionModal'
import { SharePopover } from './components/SharePopover'
import { menuShareError, menuStopError } from './lib/shareCopy'
import { adaptRemoteSession } from './lib/remoteAdapter'
import { peerConnectVerdict } from './lib/closeReasons'
import type { ReconnectVerdict } from './lib/relayClient'
import { peerView, shortHost } from './lib/sidebarPeers'
import type { AdaptedRemoteSessionInfo } from './lib/remoteAdapter'
import { ExclamationTriangleIcon, InformationCircleIcon, XMarkIcon } from '@heroicons/react/24/outline'
import { TailscaleBanner } from './components/TailscaleBanner'
import type { TailscaleHealth } from './lib/tailscale'
import { WebShareSessionView } from './components/WebShareSessionView'
import { LinkNoLongerWorks } from './components/LinkNoLongerWorks'
import { TerminalPanel } from './components/TerminalPanel'
import { ErrorBoundary } from './components/ErrorBoundary'
import { UpdateBanner } from './components/UpdateBanner'
import type { UpdateInfo } from './components/UpdateBanner'
import { WebGLRecoveryBanner } from './components/WebGLRecoveryBanner'
import { PluginToggleBanner } from './components/PluginToggleBanner'
import { QuitConfirmModal } from './components/QuitConfirmModal'
import { useColorScheme } from './lib/useColorScheme'
import { LIGHT_PALETTE, DARK_PALETTE } from './lib/terminalPalettes'
import { DEFAULT_TERMINAL_FONT_SIZE, terminalFontFamily, type TerminalFont } from './lib/terminalFont'
import { measureInitialGrid } from './lib/initialGrid'


const SHARE_POPOVER_ID = 'share-popover'

/**
 * App is the root component — it owns all tab state and wires
 * the Wails-generated TypeScript bindings to the child components.
 */
function App(): React.ReactElement {
  // Phase 120-06 — single source of truth for "is this React shell running in
  // a regular browser under /app/ vs inside the Wails desktop runtime?" Used
  // to gate the Wails RPC suite (init, retryInit, sessions polls) so the SPA
  // can be served to web-share viewers without crashing on Wails-only calls.
  // See 120-VERIFICATION.md Human Verification #2 + lib/webMode.ts.
  const mode = detectMode()
  // webParams is captured once on first mount — URL params don't change for
  // the lifetime of the SPA. useMemo keeps the reference stable so dependency
  // arrays of downstream effects never see false re-evaluations.
  const webParams = useMemo(() => readWebModeParams(), [])
  // Portty 04-06 (SHAR-03): web mode with an incomplete link, or a cap the server
  // refuses, shows Link No Longer Works and no tab.
  const [webLinkDead, setWebLinkDead] = useState(() => mode === 'web' && (!webParams.sessionId || !webParams.capToken))
  // Host label from the cap-verified session info, for the stopped-sharing banner.
  const [webHost, setWebHost] = useState('')
  // NAV-03: both modes start with no tabs. Desktop init restores running
  // sessions as tabs; web mode opens the ?session= view (effect below).
  const [tabs, setTabs] = useState<Tab[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [relayPort, setRelayPort] = useState<number | null>(null)
  const [tabCounter, setTabCounter] = useState(1)
  const [showNewSessionModal, setShowNewSessionModal] = useState(false)
  // Hide/Show Sidebar (button, toolbar button, View menu ⌃⌘S). Not persisted.
  const [sidebarHidden, setSidebarHidden] = useState(false)
  // Session id the End Session… confirm is open for (TERM-03); null = closed.
  const [endSessionTarget, setEndSessionTarget] = useState<string | null>(null)
  // Portty 04-05: the sidebar menu's last Share / Stop Sharing failure (UI-SPEC
  // warning banner); cleared on Dismiss or that session's next success.
  const [shareFailure, setShareFailure] = useState<{ sessionId: string; text: string } | null>(null)
  // Track font size per session: sessionId -> fontSize (pixels)
  const [fontSizes, setFontSizes] = useState<Record<string, number>>({})
  // Quick 261116-ku9 R1: Settings ▸ Terminal Font. Unzoomed tabs follow its size;
  // ⌘+/⌘− zoom starts from it. The family applies to every local tab.
  const [terminalFont, setTerminalFont] = useState<TerminalFont>({ family: '', size: DEFAULT_TERMINAL_FONT_SIZE })
  const terminalFontStack = useMemo(() => terminalFontFamily(terminalFont.family), [terminalFont.family])
  // Quick 261116-ku9 R2: the grid each new session's PTY was spawned at, so its xterm starts there too.
  const spawnGridRef = useRef<Record<string, { cols: number; rows: number }>>({})
  // Tailscale health state
  const [tailscaleHealth, setTailscaleHealth] = useState<TailscaleHealth | null>(null)
  // NET-01: null means "not checked yet" (and always in web mode), which is not "off".
  const tailscaleOff = tailscaleHealth !== null && !tailscaleHealth.connected
  const [daemonError, setDaemonError] = useState<string | null>(null)
  // Plugin settings state (PLUG-03): updated by GetPluginSettings on mount
  // and by the EventsOn('settings:plugins') subscription. Threaded into every
  // open TerminalPanel via prop. Phase 92 contract: TerminalPanel accepts the
  // prop but does not consume it; Phase 93 wires consumption.
  const [pluginConfig, setPluginConfig] = useState<PluginSettings | null>(null)

  // Phase 97 SER-01: saver registry. TerminalPanel registers a closure
  // that returns the addon's serialize() output keyed by sessionId; App
  // uses it when handleRequestSave fires from the TabBar context menu.
  // Cleared on unmount via TerminalPanel's useEffect cleanup (Pitfall #6).
  const [serializerRegistry, setSerializerRegistry] = useState<
    Record<string, (() => string) | null>
  >({})

  // Phase 97 SER-01: one-shot save-feedback banner. Mirrors the
  // localBanner pattern — info kind for "Serialize disabled"
  // affordance, error kind for write/dialog failures.
  const [saveBanner, setSaveBanner] = useState<{ kind: 'info' | 'error'; text: string } | null>(null)

  // Phase 98 PRG-02/PRG-03 — progress registry (mirrors Phase 97 saver registry shape).
  // useRef Map is the aggregation source: no re-render on .set/.delete; the actual
  // per-tab UI prop lives in tabProgress (useState Record). The Wails-RPC dispatch
  // is debounced via trayDebounceRef + idempotency-guarded by lastDispatchedQuartileRef.
  const progressRegistry = useRef(new Map<string, IProgressState>())
  const [tabProgress, setTabProgress] = useState<Record<string, number>>({})
  const trayDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null) // 200ms debounce window
  const lastDispatchedQuartileRef = useRef<number>(-1)

  // NAV-01: every daemon session, from the always-on 3 s poll (desktop only).
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const activeShared = !!sessions.find((s) => s.id === activeId)?.webEnabled
  // Portty 04-05: the Share popover for the active local session, anchored on the toolbar button.
  const [sharePopoverOpen, setSharePopoverOpen] = useState(false)
  const shareButtonRef = useRef<HTMLButtonElement>(null)
  // NAV-02: remote peers for the sidebar peer groups (30 s poll while Tailscale is connected).
  const [remotePeers, setRemotePeers] = useState<RemotePeerSessions[]>([])
  // True after the first successful peer poll; clears the "Looking for other Macs…" line.
  const [remoteLoaded, setRemoteLoaded] = useState(false)

  // Update notification state (lifted to App — Phase 81 D-06)
  const [update, setUpdate] = useState<UpdateInfo | null>(null)
  // Local network banner dismiss state (session-only, D-04)
  const [updateExiting, setUpdateExiting] = useState(false)

  // Phase 93 WGL-02 / WGL-03: WebGL recovery banner state. One-shot per
  // app session — webglBannerDismissed gates rendering even if the
  // underlying webglContextLost / webglSoftwareDetected event fires
  // multiple times (e.g., user toggles WebGL OFF/ON while context lost).
  const [webglContextLost, setWebglContextLost] = useState(false)
  const [webglSoftwareDetected, setWebglSoftwareDetected] = useState(false)
  const [webglBannerDismissed, setWebglBannerDismissed] = useState(false)

  // Phase 99 PUI-02: one-shot post-save Unicode 11 / Image toggle banners.
  // Each kind appears at most once at any moment; dismissing removes that kind
  // from the set; auto-dismiss fires from the PluginToggleBanner component.
  // Set deduplication via Array.from(new Set(...)) caps the array at 2 entries max.
  type PluginToggleKindLocal = 'unicode11' | 'image'
  const [pluginToggleBanners, setPluginToggleBanners] = useState<PluginToggleKindLocal[]>([])

  const handlePluginToggleSideEffect = useCallback((kinds: PluginToggleKindLocal[]) => {
    setPluginToggleBanners((prev) => Array.from(new Set([...prev, ...kinds])))
  }, [])

  // Quit confirmation modal state (Phase 85)
  const [showQuitModal, setShowQuitModal] = useState(false)
  // NAV-03: Settings is a sheet over the window (menu Settings… ⌘,), never a tab.
  const [showSettings, setShowSettings] = useState(false)
  const [quitSessions, setQuitSessions] = useState<Array<{ id: string; name: string }>>([])
  // Ref to removeTabState so the session:exit event handler (with [] deps) can call
  // the latest version without a stale closure
  const removeTabStateRef = useRef<((id: string) => void) | undefined>(undefined)
  // The menu:close-tab listener ([] deps) reads the active tab through this ref.
  const activeIdRef = useRef<string | null>(null)
  activeIdRef.current = activeId
  const tabsRef = useRef(tabs)
  tabsRef.current = tabs
  // Every session:exit id, so createTab and the init/retryInit restore skip a
  // shell that exited while their RPCs were pending (exits are reported once).
  const exitedIdsRef = useRef(new Set<string>())
  // Bumped on every tailscale:health event; init/retryInit apply their
  // GetTailscaleStatus reply only if no event arrived while it was pending.
  const healthEventSeqRef = useRef(0)

  // THEM-01: follow the system appearance. Palettes are module constants, so
  // the TerminalPanel theme effect only re-runs on a real scheme change.
  // Portty 02-13: the Settings override is desktop only; the web share page
  // always follows the browser.
  const [appearance, setAppearance] = useState<Appearance>('system')
  const terminalTheme = useColorScheme(mode === 'web' ? 'system' : appearance) === 'dark' ? DARK_PALETTE : LIGHT_PALETTE

  // Phase 97 SER-01: TerminalPanel calls this on attach/detach to
  // register/unregister its serialize() closure. Empty deps because
  // setSerializerRegistry is stable.
  const handleRegisterSaver = useCallback(
    (sessionId: string, fn: (() => string) | null) => {
      setSerializerRegistry((prev) => ({ ...prev, [sessionId]: fn }))
    },
    []
  )

  // Phase 98 PRG-02/PRG-03 — fired by every TerminalPanel via the onProgressChange prop.
  // state:1 (set) updates registry+tabProgress; everything else (state:0/2/3/4) clears the entry
  // (v3.2 ships state:1+state:0 in the UI; 2/3/4 deferred to v3.3 per RESEARCH "Cuttable Inside Cuttable").
  const handleProgressChange = useCallback(
    (sessionId: string, state: IProgressState) => {
      if (state.state === 1) {
        progressRegistry.current.set(sessionId, state)
        setTabProgress((prev) => ({ ...prev, [sessionId]: state.value }))
      } else {
        progressRegistry.current.delete(sessionId)
        setTabProgress((prev) => {
          // Strip the key without mutating the previous object.
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          const { [sessionId]: _drop, ...rest } = prev
          return rest
        })
      }
      // Recompute aggregate quartile + schedule debounced RPC.
      const quartile = aggregateProgress(progressRegistry.current)
      if (trayDebounceRef.current) {
        clearTimeout(trayDebounceRef.current)
      }
      trayDebounceRef.current = setTimeout(() => {
        if (lastDispatchedQuartileRef.current === quartile) return  // idempotent
        lastDispatchedQuartileRef.current = quartile
        void SetTrayProgress(quartile)
      }, 200)
    },
    []
  )

  // Phase 97 SER-01: TabBar calls this when the user picks "Save Terminal As…"
  // from the right-click context menu. Looks up the saver closure for the
  // tab's session, strips ANSI, sanitizes the filename, and invokes the
  // SaveTerminalSession Wails RPC. Shows a banner if no saver is registered
  // (Serialize toggled OFF) — per RESEARCH §"User Constraints / Whether
  // the Save menu item appears when Serialize is toggled OFF": always show
  // the menu item; show toast on click if disabled (discoverability).
  const handleRequestSave = useCallback(
    async (tabId: string) => {
      const tab = tabs.find((t) => t.id === tabId)
      if (!tab || !tab.sessionId) {
        setSaveBanner({ kind: 'info', text: 'Open a terminal session to save its scrollback.' })
        return
      }
      const fn = serializerRegistry[tab.sessionId]
      if (!fn) {
        setSaveBanner({ kind: 'info', text: 'Enable the Serialize plugin in Settings to save sessions.' })
        return
      }
      const plainText = stripAnsi(fn())
      const stamp = new Date().toISOString().replace(/[:T]/g, '-').replace(/\..+/, '')
      const fname = sanitizeFilename(tabLabel(tab, sessions)) + '-' + stamp + '.txt'
      try {
        await SaveTerminalSession('', fname, plainText)
      } catch (err) {
        setSaveBanner({ kind: 'error', text: 'Could not save terminal: ' + String(err) })
      }
    },
    [tabs, sessions, serializerRegistry]
  )

  const handleDismissUpdate = useCallback(() => {
    setUpdateExiting(true)
    setTimeout(() => {
      setUpdate(null)
      setUpdateExiting(false)
    }, 200)
  }, [])

  // Phase 93 WGL-02 / WGL-03: stable callback for TerminalPanel's hot-swap
  // useEffect dep array. useCallback with [] keeps the identity stable so
  // the hot-swap effect doesn't re-run when App re-renders for unrelated
  // reasons.
  const handleWebGLContextLost = useCallback((reason: 'context-loss' | 'software-rasterized') => {
    if (reason === 'software-rasterized') {
      setWebglSoftwareDetected(true)
    } else {
      setWebglContextLost(true)
    }
  }, [])

  // Phase 98 PRG-03 — clear the debounce timer on App unmount so a pending
  // tray-RPC doesn't fire after the React tree is torn down (cosmetic but
  // matches the established cleanup pattern from Phase 94 BannerStack).
  useEffect(() => {
    return () => {
      if (trayDebounceRef.current) {
        clearTimeout(trayDebounceRef.current)
        trayDebounceRef.current = null
      }
    }
  }, [])

  // On mount: hide static HTML splash and initialize.
  useEffect(() => {
    const splashEl = document.getElementById('splash-static')
    if (splashEl) splashEl.style.display = 'none'

    async function init() {
      // Phase 120-06 — skip the entire Wails RPC suite when running in web
      // mode (regular browser under /app/?session=…&cap=…). Wails RPCs are
      // unreachable from a browser; session+cap come from URL params and the
      // session view is auto-opened by a separate effect below that runs
      // AFTER openWebSessionTab is declared. We return early here before
      // any Wails-bound call to avoid the partially-functional shell flagged
      // in 120-VERIFICATION.md Human Verification #2.
      if (mode === 'web') {
        return
      }
      // Check if startup() failed before calling methods that need a.client.
      const startupErr = await GetDaemonError()
      if (startupErr) {
        setDaemonError(startupErr)
        return
      }
      try {
        const healthSeq = healthEventSeqRef.current
        const [port, allSessions, health] = await Promise.all([
          GetRelayPort(),
          ListSessions(),
          GetTailscaleStatus(),
        ])
        // A shell that exited while no GUI was watching is removed, not restored (TERM-05).
        const sessions = allSessions.filter((s) => s.state !== 'stopped' && !exitedIdsRef.current.has(s.id))
        allSessions.filter((s) => s.state === 'stopped').forEach((s) => {
          KillSession(s.id).catch((err) => console.warn('[App] KillSession of stopped session failed:', err))
        })
        setRelayPort(port)
        if (healthEventSeqRef.current === healthSeq) setTailscaleHealth(health)
        // Seed the sidebar and Share lookup from the restore call, before the first poll tick.
        setSessions(sessions)

        // Portty 02-13 THEM-01: the stored Appearance override (keep 'system' on failure).
        GetAppearance()
          .then((v) => { if (v === 'light' || v === 'dark') setAppearance(v) })
          .catch(() => { /* stay 'system' */ })
        // Quick 261116-ku9 R1: the stored Terminal Font (keep the default on failure).
        GetTerminalFont().then(setTerminalFont).catch(() => { /* keep default */ })

        // Restore existing sessions as tabs (SESS-02 reattachment after window re-show).
        if (sessions.length > 0) {
          const restoredTabs: Tab[] = sessions.map((s) => ({
            id: s.id,
            name: s.name || 'shell',
            sessionId: s.id,
          }))
          setTabs(restoredTabs)
          setActiveId(restoredTabs[0].id)
        }
      } catch (err) {
        console.error('[App] init failed:', err)
        setDaemonError(String(err))
      }
    }
    void init()

    const offHealth = EventsOn('tailscale:health', (h: TailscaleHealth) => {
      healthEventSeqRef.current++
      setTailscaleHealth(h)
    })

    const offDaemonError = EventsOn('daemon:error', (msg: string) => {
      setDaemonError(msg)
    })

    // Plugin settings (PLUG-03): initial fetch + change subscription. The
    // PluginsSection component does its own fetch on mount; this top-level
    // fetch ensures App-state has the value for the prop drill before any
    // TerminalPanel mounts. Failure leaves pluginConfig at null (toggles in
    // PluginsSection surface the error via its own loadError state).
    GetPluginSettings()
      .then((s) => setPluginConfig(s))
      .catch(() => { /* keep null; PluginsSection surfaces its own error */ })

    const offPlugins = EventsOn('settings:plugins', (s: PluginSettings) => {
      setPluginConfig(s)
    })

    // Closing a tab only detaches (TERM-03), so the tray lists sessions with no
    // tab: focus the open tab or open one. The label comes from tabLabel.
    const cancelTrayFocus = EventsOn('tray:focus-session', (sessionId: string) => {
      if (exitedIdsRef.current.has(sessionId)) return
      setTabs(prev => {
        const tab = prev.find(t => t.sessionId === sessionId)
        setActiveId(tab ? tab.id : sessionId)
        return tab ? prev : [...prev, { id: sessionId, name: 'shell', sessionId }]
      })
    })

    const offExit = EventsOn(
      'session:exit',
      (data: { sessionId: string }) => {
        // TERM-05: any shell exit, at any age and with any exit code, closes
        // its tab. The Go exit watcher has already removed the session from
        // the daemon, so this only updates UI state (no daemon calls).
        exitedIdsRef.current.add(data.sessionId)
        setSessions((prev) => prev.filter((s) => s.id !== data.sessionId))
        removeTabStateRef.current?.(data.sessionId)
      }
    )

    // 05-13: the Go wake/network detector, re-dispatched on window so RelayClient
    // (which also runs in the browser) never imports the Wails runtime.
    const offWake = EventsOn('system:wake', (reason: string) => {
      window.dispatchEvent(new CustomEvent('portty:wake', { detail: reason }))
    })

    // Native menu: File > New Session (Cmd+T).
    const offMenuNewSession = EventsOn('menu:new-session', () => setShowNewSessionModal(true))
    // Native menu: File > Close Tab (Cmd+W). Detach only; with no tab open it does nothing.
    const offMenuCloseTab = EventsOn('menu:close-tab', () => {
      const id = activeIdRef.current
      if (id !== null) removeTabStateRef.current?.(id)
    })
    // Native menu: Settings… (Cmd+,).
    const offMenuSettings = EventsOn('menu:settings', () => setShowSettings(true))
    // Native menu: View > Hide/Show Sidebar (Ctrl+Cmd+S). Functional update: no stale closure.
    const offMenuToggleSidebar = EventsOn('menu:toggle-sidebar', () => setSidebarHidden((h) => !h))

    // Subscribe to quit-requested event from Go backend (Phase 85)
    const offQuit = EventsOn('app:quit-requested', () => {
      setShowQuitModal(prev => {
        if (prev) return prev  // Ignore if modal already showing (double-fire guard)
        ListSessions().then(sessions => {
          const activeSessions = sessions
            .filter((s: SessionInfo) => s.state !== 'stopped')
            .map((s: SessionInfo) => ({ id: s.id, name: s.name || 'shell' }))
          setQuitSessions(activeSessions)
          setShowQuitModal(true)
        }).catch(() => {
          setQuitSessions([])
          setShowQuitModal(true)
        })
        return prev
      })
    })

    return () => {
      offHealth()
      offDaemonError()
      offPlugins()
      cancelTrayFocus()
      offExit()
      offQuit()
      offMenuNewSession()
      offMenuCloseTab()
      offMenuSettings()
      offMenuToggleSidebar()
      offWake()
    }
  }, [])

  const createTab = useCallback(async (workDir: string) => {
    const defaultName = `shell ${tabCounter}`
    setTabCounter((n) => n + 1)

    // Quick 261116-ku9 R2: spawn the PTY at the grid the new tab's xterm will
    // fit to, so zsh's first prompt (PROMPT_SP) is padded for the same width.
    const container = document.querySelector('.terminal-container') as HTMLElement | null
    const grid = container ? measureInitialGrid(container, terminalFontStack, terminalFont.size) : null
    if (!grid) console.warn('[App] measureInitialGrid: no layout; the daemon spawns at 80×24')

    try {
      const sessionId = await CreateSession(defaultName, workDir, grid?.cols ?? 0, grid?.rows ?? 0)
      if (exitedIdsRef.current.has(sessionId)) return
      if (grid) spawnGridRef.current[sessionId] = grid
      const tab: Tab = {
        id: sessionId,
        name: defaultName,
        sessionId,
      }
      setTabs((prev) => [...prev, tab])
      setActiveId(sessionId)

      // CR-01 fix: seed sessions synchronously so the sidebar row and the
      // toolbar Share button work for a just-created session before
      // the next 3 s poll; that poll reconciles the fields we guess here.
      setSessions((prev) => [
        ...prev,
        {
          id: sessionId,
          name: defaultName,
          state: 'running',
          status: 'running',
          createdAt: new Date().toISOString(),
          hostname: '',
          webEnabled: false,
          viewerCount: 0,
          workDir,
        } as SessionInfo,
      ])

      // SEC-01 / D-06: new sessions start with web-sharing OFF. The user must
      // explicitly toggle web on to share. The daemon enforces this at the
      // handleCreateSession layer (TestHandleCreateSession_NoAutoEnable); the
      // previous auto-seed here created a UI-daemon state mismatch that made
      // the removed Sessions page show bogus "WEB ON" state with broken URLs.
    } catch (err) {
      console.error('[App] CreateSession failed:', err)
      throw err // the New Session dialog stays open and shows it
    }
  }, [tabCounter, terminalFontStack, terminalFont.size])

  // UI-only removal of a session's tab state. session:exit calls it directly
  // (the daemon already removed the session); closeTab is nothing more.
  const removeTabState = useCallback((id: string) => {
    setTabs((prev) => {
      // Right neighbour, else left, else the empty state (TERM-03).
      setActiveId((cur) => nextActiveTabId(prev, id, cur))
      return prev.filter((t) => t.id !== id)
    })
    // Clean up font size for the closed session.
    setFontSizes((prev) => { const n = { ...prev }; delete n[id]; return n })
    delete spawnGridRef.current[id]
  }, [])
  // Keep the ref in sync so the session:exit event handler ([] deps) always calls the latest version
  removeTabStateRef.current = removeTabState

  // Closing a tab only detaches (TERM-03): no session RPC, the shell keeps
  // running in the daemon and stays in the sidebar.
  const closeTab = useCallback((id: string) => {
    removeTabState(id)
  }, [removeTabState])

  // End Session… after its confirm (TERM-03). The daemon's kill path drops
  // the session's share and disconnects its viewers (Portty 04-02). Errors
  // reach the confirm dialog. The exit watcher and the poll agree with the
  // instant removal below.
  const endSession = useCallback(async (id: string) => {
    await KillSession(id)
    exitedIdsRef.current.add(id) // an in-flight poll must not bring it back
    removeTabState(id)
    setSessions((prev) => prev.filter((s) => s.id !== id))
  }, [removeTabState])

  // Rename from the tab or the sidebar row. Tab labels derive from sessions
  // (tabLabel), so the optimistic rename shows in both places at once; a
  // failed RPC puts the old name back in both (RESEARCH Pitfall 6).
  const renameSession = useCallback(async (id: string, name: string) => {
    const previous = sessions.find((s) => s.id === id)?.name ?? ''
    setSessions((prev) => renameInList(prev, id, name))
    try {
      await RenameSession(id, name)
    } catch (err) {
      console.warn('[App] RenameSession failed:', err)
      setSessions((prev) => renameInList(prev, id, previous))
    }
  }, [sessions])

  // Share state lives on the session list; the Share popover reports a toggle
  // here so the toolbar and sidebar show it before the next poll.
  const setSessionWebEnabled = useCallback((id: string, enabled: boolean) => {
    setSessions((prev) => prev.map((s) => (s.id === id ? { ...s, webEnabled: enabled } : s)))
  }, [])
  const closeSharePopover = useCallback(() => setSharePopoverOpen(false), [])

  // Portty 04-05: the sidebar menu's Share / Stop Sharing, on the same binding as
  // the popover. Success shows only through the row icon and the toolbar; a
  // failure shows one warning banner (copy picked by code, detail from the local binding).
  const toggleShareFromMenu = useCallback(async (id: string, shared: boolean) => {
    const name = sessions.find((s) => s.id === id)?.name || 'shell'
    let r: { shared: boolean; errorCode: string; errorDetail: string }
    try {
      r = await SetSessionShared(id, shared)
    } catch (err) {
      r = { shared: !shared, errorCode: shared ? 'start_failed' : 'stop_failed', errorDetail: String(err) }
    }
    if (r.errorCode) {
      setShareFailure({ sessionId: id, text: shared ? menuShareError(name, r.errorDetail) : menuStopError(name, r.errorDetail) })
      return
    }
    setSessionWebEnabled(id, r.shared)
    setShareFailure((f) => (f?.sessionId === id ? null : f))
  }, [sessions, setSessionWebEnabled])

  // UI-SPEC "Open, close and focus": a tab switch (local or peer), ending the
  // session and opening Settings close the popover. Esc, an outside click and
  // the window losing key status are handled inside SharePopover.
  useEffect(() => { setSharePopoverOpen(false) }, [activeId])
  useEffect(() => { if (showSettings) setSharePopoverOpen(false) }, [showSettings])

  const handleFontSizeChange = useCallback((sessionId: string, delta: number) => {
    setFontSizes((prev) => {
      const current = prev[sessionId] ?? terminalFont.size
      const next = Math.max(6, Math.min(32, current + delta))
      return { ...prev, [sessionId]: next }
    })
  }, [terminalFont.size])

  // NAV-01: always-on session poll, so the sidebar lists every daemon session
  // (including ones created from the CLI) and the open Share popover stays live.
  useEffect(() => {
    if (mode === 'web') return // Phase 120-06: no Wails RPC in browser mode.
    let cancelled = false
    async function refresh() {
      try {
        const list = await ListSessions()
        // A poll in flight across session:exit or End Session may still list it.
        if (!cancelled) setSessions(list.filter((s) => !exitedIdsRef.current.has(s.id)))
        // 05-16 (R6): a respawned daemon listens on a new relay port, and new
        // local tabs dial it. init owns the first set (no empty-state flash).
        const port = await GetRelayPort()
        if (!cancelled) setRelayPort((prev) => (prev !== null && port > 0 ? port : prev))
      } catch {
        // Keep the last list until the next tick succeeds.
      }
    }
    void refresh()
    const interval = setInterval(() => void refresh(), 3000)
    return () => { cancelled = true; clearInterval(interval) }
  }, [])

  // NAV-02: poll remote peers whenever the desktop app is open and Tailscale is
  // connected; not gated on any tab. Errors keep the last list. 05-13: every 5 s
  // while visible (a new share shows up fast), 30 s while hidden, at once on
  // focus, visibility and wake. Each poll is a fan-out to every peer, so never two at once.
  useEffect(() => {
    if (mode === 'web') return // Phase 120-06: no Wails RPC in browser mode.
    // NET-01: while Tailscale is off, list no peers; tailscaleOff is a dep so
    // polling resumes (and "Looking for other Macs…" shows again) on reconnect.
    if (tailscaleOff) {
      setRemotePeers([])
      setRemoteLoaded(false)
      return
    }
    let cancelled = false
    let inFlight = false
    let lastAt = 0
    async function refresh() {
      if (inFlight) return
      inFlight = true
      lastAt = Date.now()
      try {
        const peers = await GetRemoteSessionsWithMeta()
        if (!cancelled) {
          setRemotePeers(peers ?? [])
          setRemoteLoaded(true)
        }
      } catch {
        // Absorbed: the sidebar keeps its last state until the next poll succeeds.
      } finally {
        inFlight = false
      }
    }
    const visible = () => document.visibilityState === 'visible'
    const now = () => void refresh()
    const onVisibility = () => { if (visible()) now() }
    void refresh()
    const interval = setInterval(() => {
      if (visible() || Date.now() - lastAt >= 30_000) now()
    }, 5_000)
    window.addEventListener('focus', now)
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('portty:wake', now)
    return () => {
      cancelled = true
      clearInterval(interval)
      window.removeEventListener('focus', now)
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('portty:wake', now)
    }
  }, [tailscaleOff])

  // NET-01: recheck Tailscale when the window gains focus or becomes visible
  // (launch and the 10 s poller cover the other triggers). Desktop only.
  useEffect(() => {
    if (mode === 'web') return
    // The check emits tailscale:health on change; the response is not applied,
    // so a late reply cannot overwrite a newer event.
    const recheck = () => {
      GetTailscaleStatus().catch(() => { /* keep the last value */ })
    }
    const onVisibility = () => {
      if (document.visibilityState === 'visible') recheck()
    }
    window.addEventListener('focus', recheck)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      window.removeEventListener('focus', recheck)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [mode])

  // UI-SPEC: while the window is not key, the selected sidebar row dims
  // (html.is-inactive in style.css). Desktop only.
  useEffect(() => {
    if (mode === 'web') return
    const root = document.documentElement
    const onBlur = () => root.classList.add('is-inactive')
    const onFocus = () => root.classList.remove('is-inactive')
    window.addEventListener('blur', onBlur)
    window.addEventListener('focus', onFocus)
    return () => {
      window.removeEventListener('blur', onBlur)
      window.removeEventListener('focus', onFocus)
    }
  }, [mode])

  // update:available subscription, lifted to App (Phase 81 D-06)
  useEffect(() => {
    if (mode === 'web') return // Phase 120-06: update poller is Wails-only.
    GetLastUpdateInfo()
      .then((info) => { if (info) setUpdate(info) })
      .catch(() => {})
    const offUpdate = EventsOn('update:available', (info: UpdateInfo) => {
      setUpdate(info)
    })
    return () => { offUpdate() }
  }, [])

  // Phase 155-03 — stable tab id for the web-session view. One view per
  // session (find-or-focus semantics).
  const webSessionTabId = (sessionId: string) => `__websession__${sessionId}`

  // Phase 155-03 — open (or focus) the WebShareSessionView tab for a web-share
  // session.
  //
  // Phase 168-03 (FIX-03) — extended to (sessionId, baseURL?, capToken?). The
  // app's own web-share bootstrap tab (mode==='web') still omits baseURL/
  // capToken and is resolved from the mount-stable webParams at render time
  // (unchanged behavior). Remote-peer tabs (opened via the remote-open action)
  // pass a baseURL + capToken that are carried PER-TAB on the Tab object, so
  // two different remote sessions never share params (RESEARCH Pitfall 3).
  // Portty 02-04: name is the tab label; peer tabs pass '<host> › <session>'.
  // Portty 04-06: peer tabs also pass peerHost and connectFailed; reopening an
  // existing tab with peer fields (a retry) replaces them.
  const openWebSessionTab = useCallback((sessionId: string, baseURL?: string, capToken?: string, name = 'Session', peer?: { peerHost: string; connectHost?: string; connectFailed?: boolean }) => {
    const tabId = webSessionTabId(sessionId)
    const newTab: Tab = {
      id: tabId,
      name,
      sessionId,
      type: 'web-session',
      baseURL,
      capToken,
      ...peer,
    }
    // Portty 05-09: decide add-or-update on the latest state, never a stale
    // snapshot; two appends of one id made duplicate tabs.
    setTabs((prev) => {
      if (!prev.some((t) => t.id === tabId)) return [...prev, newTab]
      return peer ? prev.map((t) => (t.id === tabId ? { ...newTab, name: t.name } : t)) : prev
    })
    setActiveId(tabId)
  }, [])

  // Phase 131 UAT follow-up — re-attach to an already-running session.
  // Opens (or focuses) a terminal tab for an existing session id. Mirrors the
  // startup-restore path (SESS-02): a terminal tab is { id: sessionId, name,
  // sessionId } with no `type`, and the Terminal component attaches to the
  // live PTY by sessionId. Wired to both the Hub cards and the Sessions panel so
  // a session created in another window (or whose tab was closed) can be reopened.
  // NOTE: the Hub surfaces this via an explicit "Open" button rather than a
  // whole-card click — card-click is reserved for the Phase 134 modal gesture.
  const handleOpenSessionTab = useCallback((sessionId: string, name: string) => {
    if (exitedIdsRef.current.has(sessionId)) return // no exit event would ever close it
    const existing = tabs.find((t) => t.id === sessionId)
    if (existing) {
      setActiveId(existing.id)
      return
    }
    const newTab: Tab = {
      id: sessionId,
      name: name || 'shell',
      sessionId,
    }
    setTabs((prev) => [...prev, newTab])
    setActiveId(newTab.id)
  }, [tabs])

  // Phase 120-06 / 155-03 — web-mode bootstrap: when the SPA loads under /app/
  // with a ?session=<id> param, open the session view (WebShareSessionView:
  // terminal) as the active tab.
  // Effect runs exactly once because mode + webParams are mount-stable.
  // Portty 04-06: check the cap first. 401/403 is a dead link; any other answer
  // (or a network error) opens the tab, and the WebSocket reports the real state.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    const { sessionId, capToken } = webParams
    if (mode !== 'web' || !sessionId || !capToken) return
    let cancelled = false
    fetch(`/api/sessions/${encodeURIComponent(sessionId)}/info?cap=${encodeURIComponent(capToken)}`)
      .then(async (res) => {
        if (cancelled) return
        if (res.status === 401 || res.status === 403) {
          setWebLinkDead(true)
          return
        }
        if (res.ok) {
          const info = await res.json().catch(() => null) as { hostname?: unknown } | null
          if (!cancelled && typeof info?.hostname === 'string') setWebHost(info.hostname)
        }
        if (!cancelled) openWebSessionTab(sessionId)
      })
      .catch(() => { if (!cancelled) openWebSessionTab(sessionId) })
    return () => { cancelled = true }
  }, [])

  // Portty 04-06: a peer row opens (or activates) its tab at once. The daemon
  // asks the peer through the same-account routes (no code) and keeps the cap;
  // the returned URL gives the tab's origin and cap. A failure opens the tab
  // with the connect-failed banner, and clicking the row again retries.
  // Portty 05-09: session ids whose connect is in flight. A repeat click
  // during a slow connect only activates an existing tab.
  const connectingRef = useRef(new Set<string>())
  const handleOpenRemoteSession = useCallback(
    async (session: AdaptedRemoteSessionInfo): Promise<void> => {
      const existing = tabs.find((t) => t.id === webSessionTabId(session.id))
      if (connectingRef.current.has(session.id)) {
        if (existing) setActiveId(existing.id)
        return
      }
      if (existing && !existing.connectFailed && !existing.shareStopped) {
        setActiveId(existing.id)
        return
      }
      connectingRef.current.add(session.id)
      const peerHost = session.hostname
      const connectHost = session.peerDNSName || peerHost
      let label = `${shortHost(peerHost)} › ${session.name}`
      try {
        const result = await ConnectPeerSession(connectHost, session.id)
        label = `${shortHost(result.host || peerHost)} › ${result.name || session.name}`
        if (!result.errorCode) {
          const parsed = new URL(result.url)
          openWebSessionTab(session.id, parsed.origin, parsed.searchParams.get('cap') ?? '', label, { peerHost, connectHost })
          return
        }
      } catch {
        // A rejected binding or a malformed URL is a failed connect, shown the same way.
      } finally {
        connectingRef.current.delete(session.id)
      }
      openWebSessionTab(session.id, undefined, undefined, label, { peerHost, connectFailed: true })
    },
    [tabs, openWebSessionTab],
  )

  // Portty 05-05 (SHAR-07): run before each peer-tab redial. Re-running the
  // connect re-checks same-account access and Share, refills this daemon's cap
  // store, and the relay port may be new after a daemon restart. It must not
  // touch tabs: changing the view's key would remount it and drop the buffer.
  const reconnectPeerTab = useCallback(async (host: string, sessionId: string): Promise<ReconnectVerdict> => {
    try {
      const port = await GetRelayPort()
      const result = await ConnectPeerSession(host, sessionId)
      return result.errorCode ? peerConnectVerdict(result.errorCode) : { kind: 'ok', port }
    } catch {
      return { kind: 'retry' }
    }
  }, [])

  // Portty 04-06: the host stopped sharing a session this Mac has a tab on. Drop
  // its row at once (the tab stays); the next peer poll reconciles. The tab is
  // marked so clicking the row again, once it is re-shared, reconnects (WR-06).
  const dropPeerSession = useCallback((peerHost: string, sessionId: string) => {
    setRemotePeers((prev) => prev.map((p) => (
      p.hostname === peerHost ? { ...p, sessions: p.sessions.filter((x) => x.id !== sessionId) } : p
    )))
    setTabs((prev) => prev.map((t) => (t.id === webSessionTabId(sessionId) ? { ...t, shareStopped: true } : t)))
  }, [])

  const retryInit = useCallback(async () => {
    setDaemonError(null)
    // Phase 120-06 — web mode has no daemon to retry against. The web-share
    // shell never sets daemonError (init early-returns before any RPC), so
    // this is defensive — but keeping the guard documents the contract that
    // every Wails-bound effect must consult `mode` first.
    if (mode === 'web') return
    try {
      await RetryDaemon()
    } catch (err) {
      setDaemonError(String(err))
      return
    }
    try {
      const healthSeq = healthEventSeqRef.current
      const [port, allSessions, health] = await Promise.all([
        GetRelayPort(),
        ListSessions(),
        GetTailscaleStatus(),
      ])
      // Mirror init: stopped sessions are removed, not restored (TERM-05).
      const sessions = allSessions.filter((s) => s.state !== 'stopped' && !exitedIdsRef.current.has(s.id))
      allSessions.filter((s) => s.state === 'stopped').forEach((s) => {
        KillSession(s.id).catch((err) => console.warn('[App] KillSession of stopped session failed:', err))
      })
      setRelayPort(port)
      if (healthEventSeqRef.current === healthSeq) setTailscaleHealth(health)
      // Mirror the mount-time sessions seed (see init above).
      setSessions(sessions)

      // Mirror init: the stored Appearance override (keep 'system' on failure).
      GetAppearance()
        .then((v) => { if (v === 'light' || v === 'dark') setAppearance(v) })
        .catch(() => { /* stay 'system' */ })
      GetTerminalFont().then(setTerminalFont).catch(() => { /* keep default */ })

      if (sessions.length > 0) {
        const restoredTabs: Tab[] = sessions.map((s) => ({
          id: s.id,
          name: s.name || 'shell',
          sessionId: s.id,
        }))
        setTabs(restoredTabs)
        setActiveId(restoredTabs[0].id)
      }
    } catch (err) {
      console.error('[App] retry init failed:', err)
      setDaemonError(String(err))
    }
  }, [])

  // Review IN-01: the poller brought the daemon back after an error. With no tabs, reload
  // as Retry would; otherwise only clear the error, since retryInit replaces the tab list
  // and would drop peer tabs and move focus (review it3 WR-02).
  useEffect(() => {
    if (mode === 'web') return
    return EventsOn('daemon:recovered', () => {
      if (tabsRef.current.length === 0) { void retryInit(); return }
      setDaemonError(null)
      GetRelayPort().then((p) => { if (p > 0) setRelayPort(p) }).catch(() => { /* keep the port */ })
    })
  }, [retryInit])

  // relayPort is set with the restored tabs, so launch never flashes the empty state.
  // R4 (quick 261116-ku9): one boolean gates the empty state and hides the
  // sidebar's New Session row, so the empty state's button is the only labelled CTA.
  const showEmptyState = mode !== 'web' && relayPort !== null && tabs.length === 0

  return (
    <div className="app">
      <div className="app__row">
      {/* Phase 159-02 (WEBCHAT-03) — web-share guest scope. In web mode the SPA
          is served to a remote guest holding a capability for ONE session; the
          desktop navigation chrome (the Sessions sidebar and its peer list) must
          NOT be rendered, or the guest could navigate away from the scoped
          session and reach the open /api/sessions/meta enumeration surface.
          Mirrors the existing `mode !== 'web'` gates on the Settings surface. */}
      {mode !== 'web' && !sidebarHidden && (
      <Sidebar
        sessions={sessions}
        activeTab={tabs.find((t) => t.id === activeId) ?? null}
        onOpenLocal={handleOpenSessionTab}
        peerView={peerView(tailscaleOff, remoteLoaded, remotePeers)}
        onOpenPeer={(peer, s) => void handleOpenRemoteSession(adaptRemoteSession(peer, s))}
        tailscaleHealth={tailscaleHealth}
        onHide={() => setSidebarHidden(true)}
        onRename={(id, name) => void renameSession(id, name)}
        onEndSession={setEndSessionTarget}
        onShareToggle={(id, shared) => void toggleShareFromMenu(id, shared)}
        onNewSession={() => setShowNewSessionModal(true)}
        hideNewSessionRow={showEmptyState}
      />
      )}
      <div className="app__content">
        <TabBar
          tabs={tabs.map((t) => ({ ...t, name: tabLabel(t, sessions) }))}
          activeId={activeId}
          onSelect={setActiveId}
          onClose={closeTab}
          onRename={(id, name) => void renameSession(id, name)}
          onEndSession={setEndSessionTarget}
          onRequestSave={pluginConfig?.serialize ? handleRequestSave : undefined}
          tabProgress={tabProgress}
          webMode={mode === 'web'}
          sidebarHidden={mode !== 'web' && sidebarHidden}
          onShowSidebar={() => setSidebarHidden(false)}
          onNewSession={() => setShowNewSessionModal(true)}
          onShare={() => setSharePopoverOpen((open) => !open)}
          shareOpen={sharePopoverOpen}
          sharePopoverId={SHARE_POPOVER_ID}
          shareButtonRef={shareButtonRef}
          shared={activeShared}
          shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}
        />

        {/* INVARIANT: this gating expression must include the trigger of EVERY banner
            rendered inside .banner-stack below. A banner whose own render condition is
            true but whose trigger is missing here will never mount (the whole stack is
            conditionally rendered). Phase 129 DNS-03 regressed exactly this way. */}
        {(tailscaleOff ||
          update ||
          ((webglContextLost || webglSoftwareDetected) && !webglBannerDismissed) ||
          saveBanner !== null ||
          shareFailure !== null ||
          pluginToggleBanners.length > 0) && (
          <div className="banner-stack">
            {tailscaleOff && <TailscaleBanner />}
            {shareFailure && (
              <div className="banner banner--warning" role="status">
                <ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />
                <span className="banner__message">{shareFailure.text}</span>
                <button type="button" className="banner__dismiss" onClick={() => setShareFailure(null)} aria-label="Dismiss">
                  <XMarkIcon aria-hidden="true" />
                </button>
              </div>
            )}
            {(webglContextLost || webglSoftwareDetected) && !webglBannerDismissed && (
              <WebGLRecoveryBanner
                reason={webglSoftwareDetected ? 'software-rasterized' : 'context-loss'}
                onDismiss={() => setWebglBannerDismissed(true)}
              />
            )}
            {update && (
              <UpdateBanner
                update={update}
                onDismiss={handleDismissUpdate}
                className={updateExiting ? 'banner-exit' : undefined}
              />
            )}
            {/* Phase 99 PUI-02: one-shot toggle-change banners for unicode11 / image.
                Both auto-dismiss after 6000ms; user can also dismiss via the Dismiss button. */}
            {pluginToggleBanners.map((kind) => (
              <PluginToggleBanner
                key={kind}
                kind={kind}
                onDismiss={() =>
                  setPluginToggleBanners((prev) => prev.filter((k) => k !== kind))
                }
              />
            ))}
            {/* Phase 97 SER-01: save-feedback banner (info = Serialize disabled,
                warning = write/dialog failure). Mirrors localBanner one-shot pattern. */}
            {saveBanner && (
              <div
                className={saveBanner.kind === 'error' ? 'banner banner--warning' : 'banner banner--info'}
                role="status"
              >
                {saveBanner.kind === 'error'
                  ? <ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />
                  : <InformationCircleIcon className="banner__icon" aria-hidden="true" />}
                <span className="banner__message">{saveBanner.text}</span>
                <button type="button" className="banner__dismiss" onClick={() => setSaveBanner(null)} aria-label="Dismiss">
                  <XMarkIcon aria-hidden="true" />
                </button>
              </div>
            )}
          </div>
        )}
        <div className="terminal-container">
        {webLinkDead && <LinkNoLongerWorks />}
        {/* Phase 155-03 — WebShareSessionView render branch. Activated when
            the active tab id starts with __websession__. This is the primary
            surface for web-share viewers: TerminalPanel
            connected to the webserver WS (not the loopback relay). relayPort
            is 0 on web-share; wsURL inside WebShareSessionView overrides the
            loopback construction so the sentinel 0 is never used for socket
            connects.

            Phase 168-03 (FIX-03) — per-tab param resolution. A remote-peer
            tab (opened via handleOpenRemoteSession) carries its OWN
            baseURL/capToken on the Tab object; those are read here instead
            of the single mount-stable webParams (RESEARCH Pitfall 3 — reading
            the global webParams for a remote tab would silently reuse the
            FIRST remote session's cap/host for a SECOND one). The app's own
            web-share bootstrap tab (mode==='web', no per-tab baseURL) still
            falls back to webParams, preserving prior behavior. pluginConfig
            is likewise only meaningful for the local bootstrap tab — App's
            own pluginConfig state describes THIS daemon's plugins, which is
            irrelevant to a different peer's session; passing it through for
            a remote tab would suppress WebShareSessionView's web-guest
            self-fetch (isWebGuest) and apply the wrong peer's config. */}
        {activeId !== null && activeId.startsWith('__websession__') && (() => {
          const activeWebTab = tabs.find((t) => t.id === activeId)
          // Portty 05-09: never fall back to window.location.origin on the desktop (wails://, the WebSocket throws).
          if (!activeWebTab && mode !== 'web') return null
          const isRemoteWebTab = activeWebTab?.baseURL !== undefined
          const wsSessionId = activeWebTab?.sessionId ?? webParams.sessionId ?? activeId.slice('__websession__'.length)
          const wsCapToken = isRemoteWebTab ? (activeWebTab?.capToken ?? '') : (webParams.capToken ?? '')
          const peerHost = activeWebTab?.peerHost
          // Portty 04-06: a peer tab whose connect failed shows only the warning banner.
          if (activeWebTab?.connectFailed) {
            return (
              <div className="terminal-wrapper" id={`tabpanel-${activeId}`} role="tabpanel" style={{ display: 'flex' }}>
                <div className="banner banner--warning" role="status">
                  <ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />
                  <span className="banner__message">
                    {`Couldn't connect to ${shortHost(peerHost ?? '')}. Check that it's awake and signed in to your Tailscale account.`}
                  </span>
                </div>
              </div>
            )
          }
          return (
            // FIX-03 (plan 09) RC-B: WebShareSessionView's flex:1 root (its
            // interactive Hub-modal body class) is inert under the block-context
            // .terminal-container, collapsing to content height (--hub-bg dead
            // band). Wrap in .terminal-wrapper (height:100%; display:flex;
            // flex-direction:column) — the same full-height chain normal
            // terminal tabs use — so flex:1 fills the pane. Fixes both the remote
            // tab and the native web-guest bootstrap tab without touching the
            // shared Hub-modal classes (still used by HubInteractiveModal).
            <div className="terminal-wrapper" id={`tabpanel-${activeId}`} role="tabpanel" style={{ display: 'flex' }}>
              {/* CR-03: key on wsSessionId so React remounts (drops stale
                  plugin-config useState) when the active tab switches
                  to a different remote session; the cap too, so a
                  reconnect after a re-share dials afresh (WR-06). */}
              <ErrorBoundary label="web-session" resetKey={activeId}>
              <WebShareSessionView
                key={`${wsSessionId}:${wsCapToken}`}
                sessionId={wsSessionId}
                capToken={wsCapToken}
                baseURL={activeWebTab?.baseURL}
                relayPort={relayPort ?? 0}
                theme={terminalTheme}
                pluginConfig={isRemoteWebTab ? undefined : (pluginConfig ?? undefined)}
                remote={isRemoteWebTab}
                // Desktop: the viewer's own font (it must be installed here). Web: the host's, self-fetched.
                fontFamily={mode !== 'web' ? terminalFontStack : undefined}
                fontSize={mode !== 'web' ? terminalFont.size : undefined}
                hostLabel={peerHost ? shortHost(peerHost) : (mode === 'web' ? webHost : undefined)}
                onShareStopped={peerHost ? () => dropPeerSession(peerHost, wsSessionId) : undefined}
                beforeReconnect={peerHost ? () => reconnectPeerTab(activeWebTab?.connectHost ?? peerHost, wsSessionId) : undefined}
              />
              </ErrorBoundary>
            </div>
          )
        })()}

        {/* Phase 120-06 — SettingsSheet is Wails-bound (calls
            HasCTDisclosure, GetStartMinimized, etc. on open). In web mode none
            of those RPCs are reachable, so it is never mounted there. The
            Settings surface has no meaning for a web-share viewer (no daemon
            to configure). */}
        {mode !== 'web' && (
          <SettingsSheet
            isOpen={showSettings}
            onClose={() => setShowSettings(false)}
            tailscaleHealth={tailscaleHealth}
            onPluginToggleSideEffect={handlePluginToggleSideEffect}
            onAppearanceChange={setAppearance}
            onTerminalFontChange={setTerminalFont}
          />
        )}
        {showEmptyState && (
          <EmptyState onNewSession={() => setShowNewSessionModal(true)} />
        )}
        {daemonError && tabs.length === 0 && (
          <div className="banner banner--warning" role="alert">
            <ExclamationTriangleIcon className="banner__icon" aria-hidden="true" />
            <span className="banner__message">
              <strong>Unable to connect to session daemon</strong>
              <br />
              {daemonError}
            </span>
            <button type="button" className="banner__action" onClick={retryInit}>
              Retry Connection
            </button>
          </div>
        )}
        {relayPort != null && relayPort > 0 &&
          tabs.map((tab) => {
            if (tab.type === 'web-session') return null
            const isActive = tab.id === activeId
            return (
              <div
                key={tab.sessionId}
                id={`tabpanel-${tab.id}`}
                role="tabpanel"
                className="terminal-wrapper"
                style={{ display: isActive ? 'flex' : 'none' }}
              >
                <ErrorBoundary label="terminal">
                <TerminalPanel
                  sessionId={tab.sessionId}
                  isActive={isActive}
                  relayPort={relayPort}
                  fontFamily={terminalFontStack}
                  initialCols={spawnGridRef.current[tab.sessionId]?.cols}
                  initialRows={spawnGridRef.current[tab.sessionId]?.rows}
                  fontSize={fontSizes[tab.sessionId] ?? terminalFont.size}
                  onFontSizeChange={(delta) => handleFontSizeChange(tab.sessionId, delta)}
                  theme={terminalTheme}
                  pluginConfig={pluginConfig}
                  onWebGLContextLost={handleWebGLContextLost}
                  onRegisterSaver={handleRegisterSaver}
                  onProgressChange={handleProgressChange}
                />
                </ErrorBoundary>
              </div>
            )
          })}
        </div>
      </div>
      </div>{/* end app__row */}

      {/* Portty 04-05: the one Share popover, for the active local session only
          (a peer tab's id matches no local session). */}
      {(() => {
        const session = sharePopoverOpen ? sessions.find((s) => s.id === activeId) : undefined
        return session ? (
          <SharePopover
            sessionId={session.id}
            sessionName={session.name || 'shell'}
            shared={!!session.webEnabled}
            anchor={shareButtonRef.current}
            popoverId={SHARE_POPOVER_ID}
            tailscaleOff={tailscaleOff}
            tailscaleDomain={tailscaleHealth?.domain ?? ''}
            onClose={closeSharePopover}
            onSharedChange={setSessionWebEnabled}
          />
        ) : null
      })()}

      {(() => {
        const target = sessions.find((s) => s.id === endSessionTarget)
        return (
          <RegenerateKeyModal
            isOpen={!!target}
            titleId="end-session-title"
            title={`End “${target?.name || 'shell'}”?`}
            body={
              <>
                {"The shell and everything running in it will stop. This can't be undone."}
                {target?.webEnabled && (
                  <span className="end-session__note">Anyone attached from another Mac or a browser will be disconnected.</span>
                )}
              </>
            }
            confirmLabel="End Session"
            actingLabel="Ending…"
            cancelLabel="Cancel"
            destructive
            onConfirm={async () => {
              await endSession(endSessionTarget!)
              setEndSessionTarget(null)
            }}
            onCancel={() => setEndSessionTarget(null)}
          />
        )
      })()}

      {showNewSessionModal && (
        <NewSessionModal
          isOpen={showNewSessionModal}
          onConfirm={async (workDir) => {
            await createTab(workDir)
            setShowNewSessionModal(false)
          }}
          onClose={() => setShowNewSessionModal(false)}
        />
      )}

      {showQuitModal && (
        <QuitConfirmModal
          isOpen={showQuitModal}
          sessions={quitSessions}
          onQuitGUI={() => { setShowQuitModal(false); void QuitGUIOnly() }}
          onQuitAll={() => { setShowQuitModal(false); void QuitAll() }}
          onCancel={() => setShowQuitModal(false)}
        />
      )}

    </div>
  )
}

export default App
