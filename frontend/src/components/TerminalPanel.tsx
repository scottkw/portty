import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import type { ITheme, IDisposable } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { ImageAddon } from '@xterm/addon-image'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import { WebglAddon } from '@xterm/addon-webgl'
import { ClipboardAddon } from '@xterm/addon-clipboard'
import { SearchAddon } from '@xterm/addon-search'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SerializeAddon } from '@xterm/addon-serialize'
import { ProgressAddon, type IProgressState } from '@xterm/addon-progress'
import { RelayClient, RECONNECT, type ReconnectVerdict } from '../lib/relayClient'
import { createPreviewAssembler, type PreviewEvent } from '../lib/previewAssembler'
import { compensatePointerScale, computeGuestViewport, DEFAULT_GUEST_MIN_SCALE } from '../lib/terminalScale'
import { applyHostGrid, resetForFullReplay } from '../lib/guestTerminal'
import { PEER_FLUSH_TIMEOUT_MS, awaitPeerSave, beginPeerSave, peerSnapshotGeneration, peerSnapshotKey, putPeerSnapshot, takePeerSnapshot, watchPeerTerm } from '../lib/peerSnapshots'
import { isSoftwareWebGL } from '../lib/webglProbe'
import { isXtermFocused } from '../lib/isXtermFocused'
import { guardPasteInsert } from '../lib/pasteGuard'
import { hostReconnectVerdict } from '../lib/hostReconnect'
import { FindBar, type FindBarSearchOptions } from './FindBar/FindBar'
import { SetSearchConfig } from '../wailsjs/go/main/App'
import { daemon } from '../wailsjs/go/models'
import { isAllowedScheme, getRisk, type RiskKind } from '../lib/urlSafety'
import { openLink, isModifierPressed, type ModifierMode } from '../lib/openLink'
import { registerPathLinks } from '../lib/pathLinks'
import { LinkConfirmPopover } from './LinkConfirmPopover'
import { SessionEndedBanner } from './SessionEndedBanner'
import { CLOSE_REASON_SHARE_STOPPED } from '../lib/closeReasons'
// Phase 113 UI-03 / UI-04 — iPad single-finger scrollback. xterm.js v6 has
// no built-in touch handling; this helper attaches the missing handlers.
import { attachTouchScroll } from '../lib/touchScrollHandler'
import { DEFAULT_TERMINAL_FONT_STACK } from '../lib/terminalFont'
import { gridForContentBox } from '../lib/initialGrid'
type PluginSettings = daemon.PluginSettings

// Custom fit that uses full container width (no hardcoded scrollbar deduction).
// FitAddon.fit() always subtracts DEFAULT_SCROLL_BAR_WIDTH (14px) even when the
// scrollbar is hidden via CSS and takes 0px — causing a permanent right-side gap.
function fitTerminal(term: Terminal): void {
  const fit = measureFit(term)
  if (!fit) return
  if (term.rows !== fit.rows || term.cols !== fit.cols) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(term as any)._core._renderService.clear()
    term.resize(fit.cols, fit.rows)
  }
}

// The grid this panel's container fits at the current cell size (INPT-06: what a
// report or claim carries), or null when it cannot be measured: zero cell metrics
// (a hidden panel, or a test terminal with no render service), no parent, or an
// empty content box.
function measureFit(term: Terminal): { cols: number; rows: number } | null {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const dims = (term as any)._core?._renderService?.dimensions
  if (!dims?.css?.cell || dims.css.cell.width === 0 || dims.css.cell.height === 0) return null

  const parent = term.element?.parentElement
  if (!parent) return null

  // Use clientWidth/Height minus padding for an unambiguous content-box size.
  // getComputedStyle().width can return border-box values with box-sizing:border-box,
  // which would include the container padding and make cols/rows too large.
  const parentStyle = window.getComputedStyle(parent)
  const px = (v: string) => parseFloat(v) || 0 // jsdom leaves unset padding ''
  const parentW = parent.clientWidth - px(parentStyle.paddingLeft) - px(parentStyle.paddingRight)
  const parentH = parent.clientHeight - px(parentStyle.paddingTop) - px(parentStyle.paddingBottom)
  if (!(parentW > 0) || !(parentH > 0)) return null

  return gridForContentBox(parentW, parentH, dims.css.cell.width, dims.css.cell.height)
}

// INPT-06: trailing debounce for the window-resize claim (a drag fires many events).
const CLAIM_DEBOUNCE_MS = 150
// Phase 12 PATH-02: how long the refused-file notice stays in the clicking pane.
const PATH_NOTICE_MS = 5000

interface TerminalPanelProps {
  sessionId: string
  isActive: boolean
  relayPort: number
  fontSize: number
  onFontSizeChange: (delta: number) => void
  /** Quick 261116-ku9 R1: xterm fontFamily (built by terminalFontFamily); defaults to the stack alone. */
  fontFamily?: string
  /** Quick 261116-ku9 R2: the PTY's spawn grid, so a pre-fit write lands at the shell's column count. */
  initialCols?: number
  initialRows?: number
  theme: ITheme
  /** When true, RelayClient builds the daemon-proxy URL
   *  (ws://127.0.0.1:{port}/api/relay/remote/{id}/ws) instead of the
   *  local-direct path. The cap is looked up server-side by sessionID —
   *  it is never passed through React state (T-134-07-01 invariant). */
  remote?: boolean
  /**
   * Phase 155: explicit WebSocket URL override. When set, RelayClient uses this
   * verbatim (e.g. wss://host/sessions/{id}/ws?cap=…) instead of building
   * ws://127.0.0.1:{port}/… — required for web-share where port is 0.
   */
  wsURL?: string
  // Phase 93 PLUG-03/WGL-01/CLIP-01: pluginConfig is consumed in the hot-swap
  // useEffect to live-attach/dispose WebGL and Clipboard addons. Unicode 11
  // is honored at session init only (next-session semantics — UI-SPEC).
  pluginConfig?: PluginSettings | null
  // Phase 93 WGL-02/WGL-03: fired when WebGL falls back to DOM (context-loss)
  // or when software-rasterizer is detected at startup (preempted).
  onWebGLContextLost?: (reason: 'context-loss' | 'software-rasterized') => void
  /**
   * Phase 97 SER-01: register/unregister this panel's serialize() closure
   * with App.tsx's saver registry. Called inside the hot-swap useEffect
   * positive arm with a closure that captures the SerializeAddon ref;
   * called with null in the negative arm AND in mount-useEffect cleanup
   * to prevent registry memory leaks (Pitfall #6).
   */
  onRegisterSaver?: (sessionId: string, fn: (() => string) | null) => void
  /**
   * Phase 98 PRG-02 — onProgressChange forwards xterm.js ProgressAddon onChange
   * events up to App.tsx for cross-session aggregation. Fired with the addon's
   * IProgressState. On detach (pluginConfig.progress flipping OFF) and on
   * TerminalPanel unmount, an explicit `{state:0, value:0}` is emitted so the
   * App-side registry can clear the entry (Pitfall #7 stuck-progress).
   */
  onProgressChange?: (sessionId: string, state: IProgressState) => void
  /** Portty 04-06: host label for the stopped-sharing banner (guest path). */
  hostLabel?: string
  /** Portty 04-06: called once when a guest socket closes with the share-stopped reason. */
  onShareStopped?: () => void
  /**
   * Phase 5 (SHAR-07): guest re-auth hook run before each redial (05-05 wires
   * GetRelayPort + ConnectPeerSession or the browser /info probe). Absent means
   * redial straight away.
   */
  beforeReconnect?: () => Promise<ReconnectVerdict>
  /** B5: the browser probe has failed for minutes; the reconnecting banner says the host can't be reached. */
  hostUnreachable?: boolean
  /** The reconnecting banner showed (true) or hid (false); App ambers the peer's sidebar dot. Also false on unmount. */
  onReconnectingChange?: (reconnecting: boolean) => void
  /** DEBT-04: the peer's connect host, the first half of this panel's snapshot key (App's peerSnapshotHost). */
  snapshotHost?: string
  /** Phase 11: this panel's preview events (one assembler per RelayClient); unmount fails an in-flight one. */
  onPreview?: (event: PreviewEvent) => void
}

/**
 * TerminalPanel creates and owns one xterm.js Terminal instance.
 * All panels render simultaneously; inactive ones are hidden via display:none
 * to preserve the terminal buffer state without destroying the DOM node.
 */
export function TerminalPanel({
  sessionId,
  isActive,
  relayPort,
  fontSize,
  onFontSizeChange,
  fontFamily = DEFAULT_TERMINAL_FONT_STACK,
  initialCols,
  initialRows,
  theme,
  remote,
  wsURL,
  pluginConfig,
  onWebGLContextLost,
  onRegisterSaver,
  onProgressChange,
  hostLabel,
  onShareStopped,
  beforeReconnect,
  hostUnreachable,
  onReconnectingChange,
  snapshotHost,
  onPreview,
}: TerminalPanelProps): React.ReactElement {
  const containerRef = useRef<HTMLDivElement>(null)
  // The relay onClose closure is built once at mount; read the latest callback through a ref.
  const onShareStoppedRef = useRef(onShareStopped)
  onShareStoppedRef.current = onShareStopped
  const beforeReconnectRef = useRef(beforeReconnect)
  beforeReconnectRef.current = beforeReconnect
  const onReconnectingChangeRef = useRef(onReconnectingChange)
  onReconnectingChangeRef.current = onReconnectingChange
  const onPreviewRef = useRef(onPreview)
  onPreviewRef.current = onPreview
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  // POL-04: deferred theme ref — stashes the theme when the panel is hidden (isActive=false)
  // so clearTextureAtlas() never runs on a display:none panel with zero cell dims.
  const pendingThemeRef = useRef<ITheme | null>(null)
  const clientRef = useRef<RelayClient | null>(null)
  // Phase 93 WGL-01 / CLIP-01: addon refs for hot-swap useEffect.
  const webglAddonRef = useRef<WebglAddon | null>(null)
  const clipboardAddonRef = useRef<ClipboardAddon | null>(null)
  // Phase 94 SRC-01..04: SearchAddon lifecycle + FindBar state. The addon
  // ref is hot-swapped from the same useEffect as webgl/clipboard (specific-
  // key dep array — Pitfall #1). The onDidChangeResults subscription is
  // disposed alongside the addon. The debounce timer is canceled on close +
  // unmount (Pitfall #10 — cancel-on-close prevents zombie searches).
  const searchAddonRef = useRef<SearchAddon | null>(null)
  const searchResultsDisposableRef = useRef<IDisposable | null>(null)
  const debounceTimerRef = useRef<number | null>(null)
  // Phase 95 LNK-01..04: WebLinksAddon lifecycle. Hot-swapped from the same
  // useEffect as webgl/clipboard/search (specific-key dep array — Pitfall #1).
  // The sub-config (modifier, confirm*) flows via webLinksConfigRef so
  // changing those does NOT re-attach the addon (Pitfall #8).
  const webLinksAddonRef = useRef<WebLinksAddon | null>(null)
  const webLinksConfigRef = useRef(pluginConfig?.webLinksConfig)
  // Phase 12 PATH-02: path links (lib/pathLinks.ts) ride the web-links toggle. The provider is registered once
  // per terminal; pathClickEnabledRef says whether the host offered path clicks on the current socket
  // (replay meta pathClick:true), so a read-only viewer or an older host sees no links.
  const pathLinksRef = useRef<IDisposable | null>(null)
  const pathClickEnabledRef = useRef(false)
  // Phase 96 IMG-01/IMG-02: ImageAddon ref. Construction is NEXT-SESSION-ONLY
  // (mount useEffect, NOT hot-swap useEffect) — toggling pluginConfig.image
  // in Settings does NOT re-attach on already-open terminals. The italic
  // "Applies to new sessions you create." caption in PluginsSection is the
  // user-facing affordance for this constraint.
  const imageAddonRef = useRef<ImageAddon | null>(null)
  // Phase 97 SER-01: SerializeAddon ref. Construction is in the HOT-SWAP
  // useEffect (NOT mount) — Serialize is a pure buffer-walker with no
  // buffer-state implications, so it can be attached/detached at runtime.
  const serializeAddonRef = useRef<SerializeAddon | null>(null)
  // Phase 98 PRG-02 — ProgressAddon hot-swap ref + onChange disposable.
  // Construction is in the HOT-SWAP useEffect (NOT mount): toggling Progress
  // in Settings adds/removes the addon live without re-mounting the terminal.
  const progressAddonRef = useRef<ProgressAddon | null>(null)
  const progressOnChangeDisposable = useRef<{ dispose(): void } | null>(null)

  // Phase 157 VIEW-04/05: isGuest = true when this panel is a guest viewer
  // (remote peer or web-share). INPT-06: sizing no longer depends on it (every
  // panel follows the server grid); it gates only what is really guest-only,
  // the session-ended banner. Stored as a ref, derived from mount-time props
  // that never change while the panel is mounted.
  const isGuestRef = useRef<boolean>(false)
  isGuestRef.current = remote || !!wsURL

  // Phase 157 VIEW-05 / Phase 175-03 BUG-01: scale the server grid into the pane.
  // INPT-06: every panel that follows a server grid uses it (a local tab that
  // lost the size scales like a guest; at scale 1 it looks as before). Reads
  // cell metrics from xterm's private renderService (same source as
  // fitTerminal) to avoid transform-feedback oscillation (Pitfall 6). Called
  // from the RelayClient onResize callback and relayout. useCallback([]) is
  // correct: the function only reads from refs whose .current is always up-to-date.
  //
  // BUG-01 (#128): below DEFAULT_GUEST_MIN_SCALE the grid would keep
  // shrinking to unreadable text on a narrow phone viewport. computeGuestViewport
  // clamps at the floor and reports overflowX so the container can switch to
  // horizontal scroll instead of shrinking further.
  const recomputeScale = useCallback(() => {
    const term = termRef.current
    const container = containerRef.current
    if (!term || !container || !term.element) return
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const core = (term as any)._core
    const dims = core?._renderService?.dimensions
    if (!dims?.css?.cell) return
    const cellW: number = dims.css.cell.width
    const cellH: number = dims.css.cell.height
    if (cellW === 0 || cellH === 0) return
    // 05-12: fit the content box (inside the padding), as fitTerminal does for hosts.
    const cs = window.getComputedStyle(container)
    const px = (v: string) => parseFloat(v) || 0 // jsdom leaves unset padding ''
    const contentW = container.clientWidth - px(cs.paddingLeft) - px(cs.paddingRight)
    const contentH = container.clientHeight - px(cs.paddingTop) - px(cs.paddingBottom)
    const { scale: s, overflowX } = computeGuestViewport(
      contentW,
      contentH,
      term.cols * cellW,
      term.rows * cellH,
      DEFAULT_GUEST_MIN_SCALE,
    )
    term.element.style.transform = `scale(${s})`
    container.classList.toggle('terminal-guest--scroll-x', overflowX)
    // 05-12: xterm 6 puts its vertical scrollbar at the right edge of a block
    // child of term.element, which the scale shrinks with it. Stretch the
    // element to contentW / s so the scrollbar meets the pane edge, or to the
    // grid's width when the grid overflows the pane (overflow-x branch). The
    // screen stays the server grid's cols wide either way.
    term.element.style.width = `${Math.max(contentW / s, term.cols * cellW)}px`
  }, [])

  // INPT-06 (Research Pattern 6): the last 0x02 grid (null before any) and the fit
  // last reported on this socket. Reports and claims carry measureFit's result,
  // never term.cols/rows: while following, those are another viewer's grid.
  const serverGridRef = useRef<{ cols: number; rows: number } | null>(null)
  const lastReportRef = useRef<string | null>(null)

  // Tell the hub this panel's fit (a report never takes the grid). Deduped per socket.
  const reportFit = useCallback(() => {
    const term = termRef.current
    const fit = term && measureFit(term)
    if (!fit || !clientRef.current) return
    const key = `${fit.cols}x${fit.rows}`
    if (key === lastReportRef.current) return
    lastReportRef.current = key
    clientRef.current.sendResize(fit.cols, fit.rows)
  }, [])

  // Every container change (activation, ResizeObserver, theme). A never-sized
  // session (no grid frame yet: the local relay sends none then) fits itself and
  // the hub adopts this report; otherwise the panel keeps the server grid and
  // rescales.
  const relayout = useCallback(() => {
    if (!termRef.current) return
    if (serverGridRef.current) recomputeScale()
    else fitTerminal(termRef.current!)
    reportFit()
  }, [recomputeScale, reportFit])

  // A user-initiated size change (window resize, zoom) claims the grid at this
  // panel's fit. xterm is not resized here: the claim's 0x02 echo makes the panel
  // follow, and an old host that ignores claims leaves the view consistent.
  const claimFit = useCallback(() => {
    const term = termRef.current
    const fit = term && measureFit(term)
    if (!fit || !clientRef.current) return
    lastReportRef.current = `${fit.cols}x${fit.rows}`
    clientRef.current.sendClaim(fit.cols, fit.rows)
  }, [])

  // Phase 94 SRC-01/02: FindBar UI state. Owned at TerminalPanel level so
  // SearchAddon (also at this level) and FindBar share a single source of
  // truth. searchOptions are seeded from pluginConfig?.searchConfig at
  // mount only (Pitfall #2 — mid-open re-sync would surprise the user).
  // Phase 175-06 (BUG-02 / #125): guest-path end notice. Set when the
  // RelayClient onClose fires on the guest (remote viewer) path, which since
  // Phase 5 (SHAR-07) happens only for a final close (share stopped, session
  // ended) or a stop verdict from beforeReconnect: the view ends there. A
  // network drop does not set it; the client reconnects and `reconnecting`
  // shows the delayed banner instead. Never set on the host path.
  const [sessionEnded, setSessionEnded] = useState<{ ended: boolean; reason?: string; shareStopped?: boolean }>({
    ended: false,
  })
  const [reconnecting, setReconnecting] = useState(false)
  // DEBT-05: the last paste a socket drop cut short, in KB (sent absent when unconfirmed); kept until dismissed.
  const [pasteNotice, setPasteNotice] = useState<{ sentKB?: number; totalKB: number } | null>(null)
  // Phase 12 PATH-02: the host refused a clicked file (unsupported type or too large). One notice at a time,
  // auto-dismissed after PATH_NOTICE_MS; a newer one replaces it and restarts the timer. It yields to every
  // other banner: dropped (not queued) if one holds the slot, removed if one takes it, never moves focus.
  const [pathNotice, setPathNotice] = useState<'path-unsupported' | 'path-too-large' | null>(null)
  const pathNoticeTimerRef = useRef<number | null>(null)
  const otherBannerRef = useRef(false)
  otherBannerRef.current = !!pasteNotice || reconnecting || (isGuestRef.current && sessionEnded.ended)
  const clearPathNotice = useCallback(() => {
    if (pathNoticeTimerRef.current !== null) window.clearTimeout(pathNoticeTimerRef.current)
    pathNoticeTimerRef.current = null
    setPathNotice(null)
  }, [])
  const showPathNotice = useCallback((kind: 'path-unsupported' | 'path-too-large') => {
    if (otherBannerRef.current) return
    if (pathNoticeTimerRef.current !== null) window.clearTimeout(pathNoticeTimerRef.current)
    setPathNotice(kind)
    pathNoticeTimerRef.current = window.setTimeout(clearPathNotice, PATH_NOTICE_MS)
  }, [clearPathNotice])
  useEffect(() => { if (!isActive || otherBannerRef.current) clearPathNotice() }, [isActive, pasteNotice, reconnecting, sessionEnded.ended, clearPathNotice])
  useEffect(() => () => { if (pathNoticeTimerRef.current !== null) window.clearTimeout(pathNoticeTimerRef.current) }, [])
  // The cleanup reports false when the banner hides and when the panel unmounts mid-reconnect.
  useEffect(() => {
    if (!reconnecting) return
    onReconnectingChangeRef.current?.(true)
    return () => onReconnectingChangeRef.current?.(false)
  }, [reconnecting])
  const [findBarOpen, setFindBarOpen] = useState(false)
  // Phase 94 WR-01 / SC-4 — exit animation. While `findBarExiting` is true,
  // the FindBar receives an `exiting` prop that toggles the .find-bar--exiting
  // CSS modifier (translateY(-8px) over 200ms + opacity over 150ms — UI-SPEC
  // §Animation line 200). The actual unmount is delayed by 200ms via
  // findBarExitTimerRef so the transition completes. Re-opening during exit
  // cancels the pending timer (Pitfall #10 — no zombie state).
  const [findBarExiting, setFindBarExiting] = useState(false)
  const findBarExitTimerRef = useRef<number | null>(null)
  const [findBarFocusSeq, setFindBarFocusSeq] = useState(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchOptions, setSearchOptions] = useState<FindBarSearchOptions>(() => ({
    regex: pluginConfig?.searchConfig?.regex ?? false,
    caseSensitive: pluginConfig?.searchConfig?.caseSensitive ?? false,
    wholeWord: pluginConfig?.searchConfig?.wholeWord ?? false,
  }))
  // Phase 94-07 WR-02 / SC-2 (gap closure) — first-load seed for searchOptions.
  // The lazy initializer above runs ONLY on first render, when pluginConfig
  // is typically still null (App.tsx loads it async via GetPluginSettings).
  // This ref guards a one-shot useEffect that seeds searchOptions exactly
  // once when pluginConfig.searchConfig FIRST becomes non-null AND the
  // find bar is NOT currently open. Pitfall #2 invariant preserved:
  // mid-open re-seeds would surprise the user (e.g. SSE-pushed change
  // from another window clobbers a toggle they just clicked).
  const seededRef = useRef(false)
  useEffect(() => {
    if (seededRef.current) return
    if (!pluginConfig?.searchConfig) return
    if (findBarOpen) return // Pitfall #2 — never re-seed mid-open.
    setSearchOptions({
      regex: !!pluginConfig.searchConfig.regex,
      caseSensitive: !!pluginConfig.searchConfig.caseSensitive,
      wholeWord: !!pluginConfig.searchConfig.wholeWord,
    })
    seededRef.current = true
  }, [pluginConfig?.searchConfig, findBarOpen])
  const [matchInfo, setMatchInfo] = useState<{ index: number; count: number }>({ index: -1, count: 0 })

  // Phase 95 LNK-03: link-confirmation popover state. Set by the
  // WebLinksAddon click handler when getRisk(displayText, href) returns a
  // non-null kind AND the matching pluginConfig.webLinksConfig.confirm*
  // flag is true. Continue → openLink + clear; Cancel → clear.
  const [linkConfirmState, setLinkConfirmState] = useState<
    { url: string; risk: RiskKind; x: number; y: number } | null
  >(null)

  // Keep webLinksConfigRef in sync with sub-config changes WITHOUT triggering
  // an addon re-attach (Pitfall #8 — sub-config is read at click time).
  useEffect(() => {
    webLinksConfigRef.current = pluginConfig?.webLinksConfig
  }, [pluginConfig?.webLinksConfig])

  // Create the terminal and relay client once per sessionId.
  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      scrollback: 10000,         // TERM-04
      allowProposedApi: true,    // required for unicode11
      cursorBlink: true,
      fontFamily,
      fontSize,
      theme,
      // Option-drag selects even when a TUI (Claude Code fullscreen) has the mouse, so Cmd-C has text to copy.
      macOptionClickForcesSelection: true,
      // Start at the PTY's spawn grid (else xterm's 80×24) until the first fit.
      ...(initialCols && initialCols > 0 && initialRows && initialRows > 0 ? { cols: initialCols, rows: initialRows } : {}),
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)

    // Phase 93 U11-01: Unicode 11 honors next-session-only semantics. The
    // pluginConfig?.unicode11 flag is read at session init; toggling it in
    // Settings does NOT mutate already-open terminals (UI-SPEC § Interaction
    // Contract — italic caption "Applies to new sessions you create."
    // explains this affordance to users). Default true if pluginConfig
    // hasn't loaded yet (preserves Phase 92 always-on behavior).
    if (pluginConfig?.unicode11 !== false) {
      const unicode11 = new Unicode11Addon()
      term.loadAddon(unicode11)
      term.unicode.activeVersion = '11'   // TERM-03: emoji + CJK + box-drawing
    }

    // Phase 96 IMG-01/IMG-02: ImageAddon is NEXT-SESSION-ONLY.
    // Lives in the MOUNT useEffect (alongside Unicode 11), NOT the
    // hot-swap useEffect — toggling pluginConfig.image in Settings
    // does NOT re-attach on already-open terminals. The italic
    // "Applies to new sessions you create." caption in PluginsSection
    // is the user-facing affordance for this constraint.
    //
    // enableSizeReports: false is MANDATORY (96-RESEARCH §"Pitfall 8":
    // CSI 14/16/18 t reports would leak terminal pixel dimensions to
    // the running CLI as keyboard input — privacy + security regression).
    if (pluginConfig?.image !== false) {
      try {
        const storageLimit = pluginConfig?.imageConfig?.storageLimit ?? 16
        const imageAddon = new ImageAddon({
          storageLimit,
          enableSizeReports: false,
        })
        term.loadAddon(imageAddon)
        imageAddonRef.current = imageAddon
      } catch (e) {
        // WASM instantiation failure is non-critical; sixel/IIP escapes
        // pass through harmlessly as printable garbage. No banner.
        // 96-RESEARCH §"Claude's Discretion / fall back gracefully".
        console.warn('Phase 96 IMG-01: ImageAddon construction failed', e)
      }
    }

    // WebGL + Clipboard addons load via the hot-swap useEffect below
    // (Phase 93 WGL-01/CLIP-01). Initial load is the same code path as
    // hot-swap because pluginConfig?.webgl/clipboard are dep-array keys.

    term.open(containerRef.current)
    const unguardPaste = guardPasteInsert(term.textarea) // WR-01: no paste left in the helper textarea
    compensatePointerScale(term) // selection and mouse reports follow the pointer under recomputeScale's scale()
    // Don't fit here — hidden panels have zero dimensions.
    // The isActive effect handles fitting when a panel becomes visible.

    // Intercept SHIFT+= and SHIFT+- for font size control; return false to suppress PTY injection.
    term.attachCustomKeyEventHandler((ev: KeyboardEvent): boolean => {
      if (ev.type !== 'keydown') return true
      if (ev.shiftKey && ev.key === '=') { onFontSizeChange(+1); return false }
      if (ev.shiftKey && ev.key === '-') { onFontSizeChange(-1); return false }
      return true
    })

    termRef.current = term
    fitAddonRef.current = fitAddon

    // Phase 157 VIEW-04: compute isGuest from props captured in this effect
    // closure (same lifetime as the relay connection — one RelayClient per session).
    const isGuest = remote || !!wsURL

    // HARD-05: a desktop peer tab is unmounted when hidden, so it keeps its content
    // and stream offset across the switch and rejoins with a delta. Its own serializer:
    // the Phase 97 serializeAddonRef is plugin-controlled (off for peer tabs).
    const snapSerializer = remote && !wsURL ? new SerializeAddon() : null
    if (snapSerializer) term.loadAddon(snapSerializer)
    const peerWatch = snapSerializer ? watchPeerTerm(term) : null // WR-01: before the snapshot write
    const snapKey = peerSnapshotKey(snapshotHost ?? '', sessionId)
    const snapGeneration = peerSnapshotGeneration(snapKey)

    // SHAR-07: the reconnecting banner waits RECONNECT.bannerDelayMs so a blip stays silent.
    let bannerTimer: ReturnType<typeof setTimeout> | null = null
    const hideReconnecting = () => {
      if (bannerTimer !== null) { clearTimeout(bannerTimer); bannerTimer = null }
      setReconnecting(false)
    }

    // DEBT-04 IN-03: a quick switch back can land while the hidden panel's save is
    // still in flight; wait for it (capped) so the restore gets a delta, not a full
    // replay. start() runs at once when nothing is pending.
    let live: { client: RelayClient; disposeData: IDisposable; preview: ReturnType<typeof createPreviewAssembler> } | null = null
    let cancelled = false
    const start = () => {
      if (cancelled) return
      const snap = snapSerializer ? takePeerSnapshot(snapKey) : undefined
      if (snap) {
        serverGridRef.current = { cols: snap.cols, rows: snap.rows }
        applyHostGrid(term, snap.cols, snap.rows, recomputeScale)
        term.write(snap.data)
      }

      // Connect relay client — one per terminal (TERM-01 independent sessions).
      // INPT-06: every panel reports its own fit once per (re)connect; in
      // reconnect mode onOpen fires after the join's grid frame and the replay.
      // The fit measured before the socket opened was never delivered.
      const preview = createPreviewAssembler((e) => onPreviewRef.current?.(e))
      const client = new RelayClient(relayPort, sessionId, {
        onPreview: preview.feed,
        onPreviewReset: preview.discard,
        onPathClickCapability: (enabled) => { pathClickEnabledRef.current = enabled },
        onPathNotice: showPathNotice,
        onOutput: (data) => { peerWatch?.feed(data); term.write(data) },
        onOpen: () => { lastReportRef.current = null; reportFit() },
        // Phase 175-06 (BUG-02 / #125): show the end banner on the guest path
        // only — the host is the owner and does not need a "session ended"
        // notice about its own session (the App exit flow reports it). Both
        // paths reconnect after a drop (SHAR-07; host tabs since 09-06), so this
        // fires only for a final close or a stop verdict.
        onClose: (code, reason) => {
          console.debug(`[RelayClient] disconnected session=${sessionId}`, code, reason)
          if (!isGuest) return
          // Portty 04-06: the view keeps its scrollback but takes no more input.
          term.options.disableStdin = true
          const shareStopped = reason === CLOSE_REASON_SHARE_STOPPED
          setSessionEnded({ ended: true, reason, shareStopped })
          if (shareStopped) onShareStoppedRef.current?.()
        },
        // INPT-06: every panel shows the session at the server's grid (0x02) →
        // term.resize + scale recompute. 05-12: applyHostGrid queues the resize
        // behind output already written, so old-width bytes parse at the old
        // width. Pitfall 5: it resizes BEFORE calling recomputeScale, so scale math
        // reads the new term.cols/rows. Clamp to ≥1 (T-157-03) lives in the helper.
        onResize: (cols, rows) => {
          serverGridRef.current = { cols, rows }
          applyHostGrid(term, cols, rows, recomputeScale)
        },
        // 05-12: an in-band reset, so output still queued is not drawn after it.
        // IN-09: the RIS goes through term.write, not feed, so re-sync the ground
        // tracker too (CAN aborts any sequence a dropped socket left open).
        onReset: () => { peerWatch?.feed(new Uint8Array([0x18])); resetForFullReplay(term) },
        // DEBT-05: N rounds down and M up, so the notice never overstates what was sent.
        onPasteInterrupted: ({ sentBytes, totalBytes }) => setPasteNotice({
          sentKB: sentBytes === null ? undefined : Math.floor(sentBytes / 1024),
          totalKB: Math.ceil(totalBytes / 1024),
        }),
        // SHAR-07: while not open, stdin is disabled and RelayClient drops input;
        // keystrokes are never queued into the shell (T-05-14).
        onStateChange: (state) => {
          if (state === 'reconnecting') {
            term.options.disableStdin = true
            hideReconnecting()
            // RECONNECT is read here only: other TerminalPanel tests mock relayClient without it.
            bannerTimer = setTimeout(() => { bannerTimer = null; setReconnecting(true) }, RECONNECT.bannerDelayMs)
            return
          }
          hideReconnecting()
          if (state === 'open') term.options.disableStdin = false
        },
      }, {
        remote,
        wsURL,
        // 09-06 (UPGR-02): host tabs reconnect too. A local relay drop used to
        // mean the daemon and session died; since the upgrade handoff it can be
        // the old daemon leaving with 1012 while the session lives on under the
        // new one. hostReconnectVerdict redials only while the daemon still lists
        // the session, so a real crash (the respawned daemon has no such
        // session) still ends the tab.
        reconnect: isGuest
          ? { beforeReconnect: () => beforeReconnectRef.current?.() ?? Promise.resolve({ kind: 'ok' }) }
          : { beforeReconnect: () => hostReconnectVerdict(sessionId) },
        resume: snap?.offset,
      })
      clientRef.current = client

      // Wire terminal input to relay (TERM-05: paste support via terminal.onData).
      // INPT-06: xterm's own resizes are not sent: in follow mode they are the
      // server's grid, and reports carry measureFit's result (reportFit).
      const disposeData = term.onData((data) => client.sendInput(data))
      live = { client, disposeData, preview }
    }
    const pendingSave = snapSerializer ? awaitPeerSave(snapKey) : null
    if (pendingSave) void pendingSave.then(start)
    else start()

    return () => {
      cancelled = true // start() may not have run: no client, nothing to save
      unguardPaste()
      live?.disposeData.dispose()
      const resumePoint = snapSerializer && live ? live.client.resumePoint() : null // read before close
      live?.client.close()
      live?.preview.discard()
      if (bannerTimer !== null) clearTimeout(bannerTimer)
      // Phase 93: dispose hot-swap addons before disposing the terminal
      // itself to avoid orphaned references. dispose() on the addon detaches
      // the render backend / clipboard handler — Terminal.dispose() then
      // tears down the buffer. Order matters only for cleanliness.
      if (webglAddonRef.current) {
        webglAddonRef.current.dispose()
        webglAddonRef.current = null
      }
      if (clipboardAddonRef.current) {
        clipboardAddonRef.current.dispose()
        clipboardAddonRef.current = null
      }
      // Phase 94 SRC-01..04: tear down SearchAddon + onDidChangeResults
      // subscription + pending debounce (HMR + unmount safety —
      // RESEARCH §"Pitfall #4" + §"Pitfall #10").
      if (searchResultsDisposableRef.current) {
        searchResultsDisposableRef.current.dispose()
        searchResultsDisposableRef.current = null
      }
      if (searchAddonRef.current) {
        searchAddonRef.current.dispose()
        searchAddonRef.current = null
      }
      if (pathLinksRef.current) {
        pathLinksRef.current.dispose()
        pathLinksRef.current = null
      }
      // Phase 95 LNK-01..04: dispose WebLinksAddon on unmount so the
      // addon's link provider releases the buffer-scan subscription.
      if (webLinksAddonRef.current) {
        try { webLinksAddonRef.current.dispose() } catch { /* ignore */ }
        webLinksAddonRef.current = null
      }
      // Phase 96 IMG-01/IMG-02: dispose ImageAddon on unmount. Mirror
      // Phase 95 web-links cleanup style (try/catch + null the ref).
      if (imageAddonRef.current) {
        try { imageAddonRef.current.dispose() } catch { /* ignore */ }
        imageAddonRef.current = null
      }
      // Phase 97 SER-01: dispose serializeAddon AND flush the saver registry
      // entry on unmount (Pitfall #6 — leaving a stale closure behind would
      // mean handleRequestSave invokes a disposed addon).
      if (serializeAddonRef.current) {
        serializeAddonRef.current.dispose()
        serializeAddonRef.current = null
      }
      onRegisterSaver?.(sessionId, null)
      // Phase 98 PRG-02 — dispose progressAddon AND emit {state:0, value:0} so the
      // App-side registry clears the entry (Pitfall #7 — leaving a stale entry
      // behind would mean the cross-session mean keeps a dead 50% reading forever).
      if (progressAddonRef.current) {
        progressOnChangeDisposable.current?.dispose()
        progressOnChangeDisposable.current = null
        progressAddonRef.current.dispose()
        progressAddonRef.current = null
      }
      onProgressChange?.(sessionId, { state: 0, value: 0 })
      if (debounceTimerRef.current !== null) {
        window.clearTimeout(debounceTimerRef.current)
        debounceTimerRef.current = null
      }
      // Phase 94 WR-01: clear any pending exit-animation unmount so the
      // timer doesn't fire after the panel is gone (would no-op safely
      // because state setters on unmounted components are silent in React
      // 18+, but the ref state machine wants a clean teardown).
      if (findBarExitTimerRef.current !== null) {
        window.clearTimeout(findBarExitTimerRef.current)
        findBarExitTimerRef.current = null
      }
      if (snapSerializer && resumePoint !== null) {
        // xterm parses writes asynchronously: serialize only once received output is
        // in the buffer, so the snapshot never lags its offset. W2: if the callback
        // never comes, the timeout still disposes the terminal and stores nothing
        // (the buffer may lag the offset). A remount waits on settleSave (IN-03).
        const settleSave = beginPeerSave(snapKey)
        let finished = false
        const finish = (save: boolean) => {
          if (finished) return
          finished = true
          clearTimeout(flushTimer)
          // WR-01: no snapshot when the delta would land in a torn or unrepresentable state.
          const extra = save ? (peerWatch?.suffix() ?? null) : null
          if (extra !== null) {
            putPeerSnapshot(snapKey, { data: snapSerializer.serialize() + extra, cols: term.cols, rows: term.rows, offset: resumePoint }, snapGeneration)
          }
          peerWatch?.dispose()
          snapSerializer.dispose()
          term.dispose()
          settleSave()
        }
        const flushTimer = setTimeout(() => finish(false), PEER_FLUSH_TIMEOUT_MS)
        term.write('', () => finish(true))
      } else {
        peerWatch?.dispose()
        snapSerializer?.dispose()
        term.dispose()
      }
      termRef.current = null
      fitAddonRef.current = null
      clientRef.current = null
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  // onFontSizeChange + pluginConfig?.unicode11 intentionally omitted: mount
  // effect runs once per session; unicode11 is read at init (next-session
  // semantics, UI-SPEC § Interaction Contract). wsURL added (Phase 155) so
  // a web-share remount reconnects with the correct override URL.
  }, [sessionId, wsURL])

  // Phase 113 UI-03 / UI-04 — iPad single-finger scrollback.
  // Lives AFTER the mount useEffect so termRef.current is populated before
  // this runs. Listeners are attached to the React-owned outer container
  // <div> (not anything inside .xterm), so they survive term.dispose() in
  // the mount-effect cleanup. Both effects key off sessionId — cleanup runs
  // in reverse order (this effect's cleanup first → mount cleanup), but the
  // container ref itself is React-owned and persists across xterm re-mount.
  useEffect(() => {
    const container = containerRef.current
    const term = termRef.current
    if (!container || !term) return
    const cleanup = attachTouchScroll(container, term)
    return cleanup
  }, [sessionId])

  // Phase 93 hot-swap useEffect (WGL-01 + CLIP-01 + WGL-02 + WGL-03).
  // Lives AFTER the mount useEffect so termRef.current is set before this
  // first runs. Dep array keys are SPECIFIC fields (Pitfall #1 — putting
  // the whole pluginConfig object would re-run on every save even if the
  // relevant flags didn't change). Unicode 11 is intentionally NOT in this
  // dep array — it's next-session-only.
  useEffect(() => {
    const term = termRef.current
    if (!term) return

    // WebGL hot-swap (WGL-01) + software-rasterizer preemption (WGL-03)
    //                       + context-loss recovery (WGL-02)
    if (pluginConfig?.webgl) {
      if (!webglAddonRef.current) {
        if (isSoftwareWebGL()) {
          // Software-rasterized WebGL detected at startup — DOM renderer
          // is preemptively used; one-shot persistent toast informs user.
          onWebGLContextLost?.('software-rasterized')
        } else {
          try {
            const webglAddon = new WebglAddon()
            webglAddon.onContextLoss(() => {
              // Phase 112 UI-01 / Issue #55: notify React FIRST while the
              // emitter chain is still alive. The previous order disposed the
              // addon synchronously inside its own onContextLoss fire, which
              // tore down the Event.forward + renderer-reset disposables
              // registered in WebglAddon.activate() (see
              // node_modules/@xterm/addon-webgl/src/WebglAddon.ts:84-97) and
              // aborted the continuation that would have invoked this React
              // notify call — leaving webglContextLost stuck at false and the
              // banner unrendered.
              onWebGLContextLost?.('context-loss')
              // Defer the addon teardown to a microtask so the emitter's
              // .fire() returns cleanly before we dispose. try/catch matches
              // the established defensive pattern at line ~320 (WebLinksAddon)
              // — dispose() may throw post-loss when the render service has
              // already torn itself down. The ref-identity guard prevents
              // clobbering a fresh addon if hot-swap re-registered one
              // between fire and microtask flush.
              queueMicrotask(() => {
                try { webglAddon.dispose() } catch { /* ignore — addon may already be torn down */ }
                if (webglAddonRef.current === webglAddon) {
                  webglAddonRef.current = null
                }
              })
            })
            term.loadAddon(webglAddon)
            webglAddonRef.current = webglAddon
          } catch (err) {
            // WebGL context creation failed — silent (no toast); user
            // explicitly enabled WebGL. Browser console still surfaces err.
            console.warn(`[TerminalPanel] WebGL unavailable for session ${sessionId}:`, err)
          }
        }
      }
    } else {
      // Toggle OFF — dispose addon if loaded. The Terminal's buffer
      // (scrollback) survives; only the render backend detaches.
      if (webglAddonRef.current) {
        webglAddonRef.current.dispose()
        webglAddonRef.current = null
      }
    }

    // Clipboard hot-swap (CLIP-01)
    if (pluginConfig?.clipboard) {
      if (!clipboardAddonRef.current) {
        const clipAddon = new ClipboardAddon()
        term.loadAddon(clipAddon)
        clipboardAddonRef.current = clipAddon
      }
    } else {
      if (clipboardAddonRef.current) {
        clipboardAddonRef.current.dispose()
        clipboardAddonRef.current = null
      }
    }

    // Phase 94 SRC-01..04: SearchAddon hot-swap. Loading is symmetric with
    // webgl/clipboard — single useEffect coordinates all three. The
    // onDidChangeResults subscription is created once when the addon
    // attaches and disposed when the addon detaches. Default highlight
    // limit (1000) — RESEARCH §"SearchAddon API Contract".
    if (pluginConfig?.search) {
      if (!searchAddonRef.current) {
        const searchAddon = new SearchAddon()
        term.loadAddon(searchAddon)
        searchAddonRef.current = searchAddon
        searchResultsDisposableRef.current = searchAddon.onDidChangeResults((e) => {
          setMatchInfo({ index: e.resultIndex, count: e.resultCount })
        })
      }
    } else {
      if (searchResultsDisposableRef.current) {
        searchResultsDisposableRef.current.dispose()
        searchResultsDisposableRef.current = null
      }
      if (searchAddonRef.current) {
        searchAddonRef.current.dispose()
        searchAddonRef.current = null
      }
      // Search disabled mid-session: close the bar + clear local state.
      setFindBarOpen(false)
      setSearchQuery('')
      setMatchInfo({ index: -1, count: 0 })
    }

    // Phase 95 LNK-01..04: WebLinksAddon hot-swap arm.
    // Sub-config (modifier, confirm*) is read at click time via
    // webLinksConfigRef.current so toggling sub-config does NOT re-attach
    // the addon (Pitfall #8). Only the boolean main toggle
    // (pluginConfig?.webLinks) drives load/dispose.
    //
    // Plan B (Wave 0 spike outcome — 95-RESEARCH §"Wave 0 Spike Outcome"):
    // OSC 8 mismatch detection deferred to v3.3 because the public
    // hyperlink-id accessor is absent from @xterm/xterm@6.0.0 typings, so no
    // OSC 8 secondary provider exists; v3.2 ships IDN + typosquat detectors
    // only. The osc8 branch in LinkConfirmPopover ships dormant for the v3.3
    // wiring slice. Phase 12 path links come from lib/pathLinks.ts (registered
    // below, same toggle); this file still holds no provider code of its own.
    if (pluginConfig?.webLinks) {
      if (!webLinksAddonRef.current) {
        const handler = (event: MouseEvent, uri: string) => {
          // LNK-01 defense-in-depth: handler re-validates the scheme even
          // though the addon's default regex already rejects non-allowlisted
          // schemes. A buggy upstream regex must NEVER punch through.
          if (!isAllowedScheme(uri)) return
          const cfg = webLinksConfigRef.current
          // LNK-02: modifier-click gate. Default 'platform' = Cmd on darwin
          // / Ctrl elsewhere. Modifier='none' bypass is intentional escape
          // hatch (Pitfall #9 — risk gates still fire below).
          // The daemon-typed `modifier` is `string`; narrow to ModifierMode
          // (the daemon's defaultPluginSettings + SetWebLinksConfig RPC
          // validation pin the value to one of these four literals — Plan
          // 95-05 enforces, here we accept and fall back defensively).
          //
          // WR-03: use truthy fallback (||) — matching web/assets/terminal.js
          // — so an empty-string `modifier` (corrupted settings.json edge
          // case) falls back to 'platform' instead of breaking the click
          // gate silently. Desktop and web must behave identically here
          // (UI-SPEC web parity mandate). WR-02's API-boundary validation
          // is the primary defense; this is the secondary belt-and-braces.
          const modifier = (cfg?.modifier || 'platform') as ModifierMode
          if (!isModifierPressed(event, modifier)) return
          // LNK-03: risk detection. For plain-text URL matches the addon
          // emits, displayText === uri, so osc8Mismatch never fires here —
          // OSC 8 mismatch is a v3.3 follow-up (Plan B; see header).
          const risk = getRisk(uri, uri)
          const shouldConfirm =
            (risk === 'osc8' && (cfg?.confirmOSC8 ?? true)) ||
            (risk === 'idn' && (cfg?.confirmIDN ?? true)) ||
            (risk === 'typosquat' && (cfg?.confirmTyposquat ?? true))
          if (risk && shouldConfirm) {
            setLinkConfirmState({ url: uri, risk, x: event.clientX, y: event.clientY })
            return
          }
          // LNK-04: platform-aware opener (Wails BrowserOpenURL on desktop;
          // window.open with noopener,noreferrer on web).
          openLink(uri)
        }
        const addon = new WebLinksAddon(handler, {
          // POLISH-01 (Phase 102): explicit urlRegex that matches http(s)
          // AND mailto: URLs. The addon's default regex matches only http(s)
          // — passing mailto through here lets Cmd/Ctrl-click on mailto links
          // in terminal output route to the system mail client.
          // POLISH-02 (Phase 102): IDN/Cyrillic-homograph URLs are already
          // detected at the handler level via getRisk → 'idn' → popover
          // (urlSafety.hasIDN handles non-ASCII + xn-- forms, including
          // mailto: addresses with IDN domains).
          // Defense-in-depth: handler still re-checks scheme via
          // isAllowedScheme (LNK-01).
          urlRegex: /(https?:\/\/[^\s$.?#].[^\s]*|mailto:[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,})/,
          hover: (event: MouseEvent, uri: string) => {
            // Native title attribute is the simplest accessible tooltip;
            // xterm exposes the link element via this callback's event.target.
            if (event.target instanceof HTMLElement) {
              event.target.setAttribute('title', uri)
            }
          },
          leave: (event: MouseEvent) => {
            // Pitfall #10: BOTH hover AND leave callbacks are required.
            // Without leave, a stale tooltip persists over non-link cells
            // after the mouse moves off the link.
            if (event.target instanceof HTMLElement) {
              event.target.removeAttribute('title')
            }
          },
        })
        term.loadAddon(addon)
        webLinksAddonRef.current = addon
      }
      if (!pathLinksRef.current) {
        pathLinksRef.current = registerPathLinks(term, {
          enabled: () => pathClickEnabledRef.current,
          modifier: () => (webLinksConfigRef.current?.modifier || 'platform') as ModifierMode,
          onClick: (request) => clientRef.current?.sendPathClick(request),
        })
      }
    } else {
      if (pathLinksRef.current) {
        pathLinksRef.current.dispose()
        pathLinksRef.current = null
      }
      // Toggle OFF — dispose addon. Terminal buffer (scrollback) survives;
      // only the link provider detaches.
      if (webLinksAddonRef.current) {
        try { webLinksAddonRef.current.dispose() } catch { /* ignore */ }
        webLinksAddonRef.current = null
      }
      // Clear any in-flight confirm popover so users don't get a stuck
      // dialog after disabling the feature.
      setLinkConfirmState(null)
    }

    // Phase 97 SER-01 hot-swap arm. Mirrors clipboard/webgl shape. Serialize is
    // hot-swap-friendly (pure buffer-walker, no buffer-state implications) —
    // distinct from Image and Unicode 11 which are mount-only per 97-PATTERNS.md
    // §"Hot-swap addon arm contract". When attaching, register the addon's
    // serialize() closure with App.tsx's saver registry so TabBar's right-click
    // "Save Terminal As…" can reach it.
    if (pluginConfig?.serialize) {
      if (!serializeAddonRef.current) {
        const serializeAddon = new SerializeAddon()
        term.loadAddon(serializeAddon)
        serializeAddonRef.current = serializeAddon
        onRegisterSaver?.(sessionId, () => serializeAddon.serialize({ excludeModes: true }))
      }
    } else {
      if (serializeAddonRef.current) {
        serializeAddonRef.current.dispose()
        serializeAddonRef.current = null
        onRegisterSaver?.(sessionId, null) // Pitfall #6 — flush stale closure
      }
    }

    // Phase 98 PRG-02 — ProgressAddon hot-swap arm (mirror of serialize/web-links/search arms).
    // Gates on pluginConfig?.progress (default OFF in v3.2; the entire codepath is dead
    // when Progress is OFF — this is the cuttability invariant enforced by the OFF-path
    // negative regression test internal/release/no_progress_when_off_test.go).
    if (pluginConfig?.progress) {
      if (!progressAddonRef.current) {
        const progressAddon = new ProgressAddon()
        term.loadAddon(progressAddon)
        progressAddonRef.current = progressAddon
        // Subscribe to onChange; capture disposable for clean teardown on detach/unmount.
        // Phase 98 v3.2 scope: state:1 (set) + state:0 (remove); state:2/3/4 deferred.
        // App.tsx maps state:2/3/4 → state:0 by convention (RESEARCH "Cuttable Inside Cuttable").
        progressOnChangeDisposable.current = progressAddon.onChange((state) => {
          onProgressChange?.(sessionId, state)
        })
      }
    } else {
      if (progressAddonRef.current) {
        progressOnChangeDisposable.current?.dispose()
        progressOnChangeDisposable.current = null
        progressAddonRef.current.dispose()
        progressAddonRef.current = null
        // Pitfall #7: explicitly clear the registry entry on detach so a closed-but-stuck
        // session does not contribute to the cross-session mean forever.
        onProgressChange?.(sessionId, { state: 0, value: 0 })
      }
    }
  }, [pluginConfig?.webgl, pluginConfig?.clipboard, pluginConfig?.search, pluginConfig?.webLinks, pluginConfig?.serialize, pluginConfig?.progress, onWebGLContextLost, onRegisterSaver, onProgressChange, sessionId])

  // INPT-06: when this panel becomes active, lay it out (fit a never-sized
  // session, else rescale to the server grid) and report its fit; track
  // container size changes via ResizeObserver, the same way. While active, a
  // window resize (also fired by browser zoom and fullscreen) claims. One path
  // for local tabs, peer tabs and the browser viewer.
  useEffect(() => {
    if (!isActive || !containerRef.current) return

    // POL-04: drain any theme that arrived while this panel was hidden.
    // Must happen synchronously before the rAF loop so the first fit uses the
    // correct theme. No extra relayout call needed here — the rAF loop below runs it.
    if (pendingThemeRef.current && termRef.current) {
      termRef.current.options.theme = pendingThemeRef.current
      termRef.current.clearTextureAtlas()
      pendingThemeRef.current = null
    }

    const container = containerRef.current
    let rafId: number | undefined
    let cancelled = false
    const MAX_ATTEMPTS = 20  // ~333ms at 60fps; covers slow CLI startup delays

    const tryFit = (attempt: number) => {
      if (cancelled) return

      // proposeDimensions() returns undefined when css.cell.width === 0
      // (CharSizeService hasn't measured font yet — zero cell dims from display:none open())
      const dims = fitAddonRef.current?.proposeDimensions()
      if (dims !== undefined) {
        relayout()
        return
      }

      // Cell dimensions not ready — schedule next rAF attempt
      if (attempt < MAX_ATTEMPTS) {
        rafId = requestAnimationFrame(() => tryFit(attempt + 1))
      } else {
        // Best-effort fallback after max attempts
        relayout()
      }
    }

    // Initial rAF: ensure display:none -> flex layout change is committed
    rafId = requestAnimationFrame(() => tryFit(0))

    // ResizeObserver handles all subsequent container changes. They only report:
    // the reconnecting banner, the paste notice and the sidebar change the
    // container too, and none of those is the user taking the size (INPT-06).
    const ro = new ResizeObserver(() => { relayout() })
    ro.observe(container)

    // A window resize is the user taking the size: claim once the drag settles,
    // but only if a person can have caused it (WR-03). macOS also resizes an
    // unattended window (display unplugged or asleep, wake, Stage Manager); that
    // only reports, so it cannot take the grid from the viewer in use. Visible
    // and focused is a proxy for "a person is here", not proof (IN-08).
    // shortcut: a key window on an unlocked, unattended Mac still claims on a
    // display change (monitor power-off, dock or KVM switch, resolution change),
    // and a user dragging the edge of an inactive window only reports; gate on
    // recent pointerdown/keydown instead if either shows up for real.
    let claimTimer: ReturnType<typeof setTimeout> | undefined
    const onWindowResize = () => {
      clearTimeout(claimTimer)
      claimTimer = setTimeout(() => {
        if (document.visibilityState === 'visible' && document.hasFocus()) claimFit()
        else reportFit()
      }, CLAIM_DEBOUNCE_MS)
    }
    window.addEventListener('resize', onWindowResize)

    return () => {
      cancelled = true
      if (rafId !== undefined) cancelAnimationFrame(rafId)
      ro.disconnect()
      window.removeEventListener('resize', onWindowResize)
      clearTimeout(claimTimer)
    }
  // relayout, claimFit and reportFit are stable useCallbacks that read only refs, so they are
  // safely closure-captured and do not belong in the dep array. Keeping [isActive]
  // as the sole dependency preserves the Phase 35 FILL-01..06 rAF retry-loop invariant
  // (the fit/activation effect must re-run only when the panel becomes active).
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isActive])

  // Apply Terminal Font family/size changes live (quick 261116-ku9 R1).
  // POL-04: never touch render options on a hidden panel; isActive re-runs this
  // on activation. INPT-06: a change made while the panel is shown (Cmd+/Cmd−,
  // Settings) is the user zooming, so it lays out and claims the grid at the new
  // fit (A8: xterm updates its cell metrics synchronously on the option set). A
  // change applied on activation (made while hidden) is a tab switch: report only.
  const fontShownRef = useRef(false)
  useEffect(() => {
    const wasShown = fontShownRef.current
    fontShownRef.current = isActive
    const term = termRef.current
    if (!term || !isActive) return
    if (term.options.fontFamily === fontFamily && term.options.fontSize === fontSize) return
    term.options.fontFamily = fontFamily
    term.options.fontSize = fontSize
    relayout()
    if (wasShown) claimFit()
  // relayout and claimFit are stable useCallbacks that read only refs.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fontFamily, fontSize, isActive])

  // Apply theme changes from the controlled prop (THM-03).
  // POL-04: isActive-guarded — a display:none panel has rows=0; running
  // clearTextureAtlas() + refresh on zero cell dims corrupts the WebGL atlas
  // for when the panel next becomes visible.  Instead, stash the theme in
  // pendingThemeRef and apply it synchronously at the top of the isActive
  // effect when the panel next becomes active (before the rAF fit loop runs).
  // On the active path: clearTextureAtlas() then relayout() — NOT refresh() —
  // so the layout uses the cell dims recalculated after the atlas rebuild.
  // INPT-06: relayout, not fitTerminal, so a panel following the server grid
  // keeps it (it only rescales) and the theme never resizes the shell.
  useEffect(() => {
    if (!termRef.current) return
    if (!isActive) {
      // Stash for application when this panel next becomes active.
      pendingThemeRef.current = theme
      return
    }
    termRef.current.options.theme = theme
    termRef.current.clearTextureAtlas()
    relayout()   // NOT refresh() — recalculates cell dims after atlas clear
  // relayout is a stable useCallback that reads only refs.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [theme, isActive])

  // Phase 94 SRC-01: focus-conditioned Cmd-F (Mac) / Ctrl-F (Win/Linux)
  // window keydown listener. The isXtermFocused() guard mitigates T-94-03
  // by letting browser-native find pass through when focus is on a sibling
  // (sidebar / modal / settings). When the bar is already open, re-pressing
  // Cmd-F bumps focusSeq to re-focus the search input (UI-SPEC §"Opening
  // the Find Bar — when already open").
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent): void {
      if (!pluginConfig?.search) return
      const isMac = navigator.platform.toUpperCase().includes('MAC')
      const modifier = isMac ? e.metaKey : e.ctrlKey
      if (!modifier || e.key.toLowerCase() !== 'f') return
      if (!isXtermFocused(containerRef.current)) return
      e.preventDefault()
      // Phase 94 WR-01: re-opening during exit cancels the pending unmount
      // timer (Pitfall #10 — no zombie state). Drop the .find-bar--exiting
      // modifier so the bar re-appears at-rest, no flicker.
      if (findBarExitTimerRef.current !== null) {
        window.clearTimeout(findBarExitTimerRef.current)
        findBarExitTimerRef.current = null
      }
      setFindBarExiting(false)
      setFindBarOpen(true)
      setFindBarFocusSeq((s) => s + 1)
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [pluginConfig?.search])

  // Phase 94 SRC-01..04: FindBar callback handlers. The 100ms input debounce
  // (RESEARCH §"Pattern 4") coalesces keystrokes; toggle changes re-search
  // instantly from local state then fire-and-forget SetPluginSettings
  // (RESEARCH §"Pattern 3"; Pitfall #2 — no mid-open re-sync from props).
  const handleSearchQueryChange = useCallback((q: string) => {
    setSearchQuery(q)
    if (debounceTimerRef.current !== null) {
      window.clearTimeout(debounceTimerRef.current)
    }
    debounceTimerRef.current = window.setTimeout(() => {
      debounceTimerRef.current = null
      if (q === '') {
        searchAddonRef.current?.clearDecorations()
        setMatchInfo({ index: -1, count: 0 })
        return
      }
      // Phase 94 SRC-02 + SRC-04 reconciliation: pass decorations: {} so
      // SearchAddon._fireResults fires the onDidChangeResults event (it gates on
      // !!opts.decorations). The empty object registers invisible decoration
      // overlays — no per-theme color overrides — so the active match still
      // highlights via xterm core's selection (theme.selectionBackground) and
      // the 138-theme invariant holds. The forbidden color keys are listed
      // and source-inspected by FindBar.themeMatrix.test.tsx.
      // Discovered during Plan 94-05 chromedp e2e (web parity wave).
      searchAddonRef.current?.findNext(q, { ...searchOptions, decorations: {} as never })
    }, 100)
  }, [searchOptions])

  const handleSearchOptionsChange = useCallback((opts: FindBarSearchOptions) => {
    setSearchOptions(opts)
    // Phase 94 SRC-02 + SRC-04: see decorations comment in handleSearchQueryChange.
    if (searchQuery) searchAddonRef.current?.findNext(searchQuery, { ...opts, decorations: {} as never })
    // Phase 94-07 WR-03 (gap closure) — write ONLY the searchConfig sub-key.
    // Previously we constructed a full PluginSettings from the App-level
    // prop and called SetPluginSettings, which raced PluginsSection's
    // stale edit buffer (PluginsSection only fetches GetPluginSettings on
    // mount and does NOT subscribe to settings:plugins). The new
    // SetSearchConfig RPC mutates only e.pluginSettings.SearchConfig
    // under the engine mutex — PluginsSection's unsaved boolean edits
    // can no longer be clobbered by a find-bar persistence call.
    //
    // The Phase 93 PLUG-04 listener fires from the daemon side, so the
    // settings:plugins SSE event still pushes to web subscribers — SRC-05
    // web parity unchanged. App.SetSearchConfig also re-emits the Wails
    // "settings:plugins" runtime event so the desktop App.tsx listener
    // continues to receive a frame on every change.
    SetSearchConfig(new daemon.SearchConfig(opts)).catch(() => {
      /* silent — Settings panel surfaces persistence errors */
    })
  }, [searchQuery])

  const handleSearchNext = useCallback(() => {
    // Phase 94 SRC-02 + SRC-04: see decorations comment in handleSearchQueryChange.
    if (searchQuery) searchAddonRef.current?.findNext(searchQuery, { ...searchOptions, decorations: {} as never })
  }, [searchQuery, searchOptions])

  const handleSearchPrev = useCallback(() => {
    // Phase 94 SRC-02 + SRC-04: see decorations comment in handleSearchQueryChange.
    if (searchQuery) searchAddonRef.current?.findPrevious(searchQuery, { ...searchOptions, decorations: {} as never })
  }, [searchQuery, searchOptions])

  const handleSearchClose = useCallback(() => {
    if (debounceTimerRef.current !== null) {
      window.clearTimeout(debounceTimerRef.current)
      debounceTimerRef.current = null
    }
    searchAddonRef.current?.clearDecorations()
    setSearchQuery('')
    setMatchInfo({ index: -1, count: 0 })
    // Phase 94 WR-01 / SC-4 — play the 200ms exit transition before unmount.
    // CSS .find-bar--exiting (frontend/src/style.css:2180-2184): transform
    // 200ms ease + opacity 150ms ease. Match the longer (200ms) duration so
    // we unmount only after the slide-up completes. UI-SPEC §"Closing the
    // Find Bar" lines 304-311 (animation-first then unmount sequence).
    setFindBarExiting(true)
    if (findBarExitTimerRef.current !== null) {
      window.clearTimeout(findBarExitTimerRef.current)
    }
    findBarExitTimerRef.current = window.setTimeout(() => {
      findBarExitTimerRef.current = null
      setFindBarExiting(false)
      setFindBarOpen(false)
      // Return focus to the xterm helper textarea (xterm's internal input
      // sink). UI-SPEC §"Closing the Find Bar > Return focus to terminal".
      containerRef.current
        ?.querySelector<HTMLTextAreaElement>('.xterm-helper-textarea')
        ?.focus()
    }, 200)
  }, [])

  return (
    <div
      ref={containerRef}
      className="terminal-session-container"
      style={{
        flex: 1,
        width: '100%',
        minHeight: 0,
      }}
    >
      {isGuestRef.current && sessionEnded.ended && (
        <SessionEndedBanner
          reason={sessionEnded.reason}
          kind={sessionEnded.shareStopped ? 'share-stopped' : 'ended'}
          host={hostLabel}
          onDismiss={() => setSessionEnded({ ended: false })}
        />
      )}
      {pasteNotice && (
        <SessionEndedBanner
          kind="paste-interrupted"
          pasteSentKB={pasteNotice.sentKB}
          pasteTotalKB={pasteNotice.totalKB}
          onDismiss={() => setPasteNotice(null)}
        />
      )}
      {reconnecting && !sessionEnded.ended && (
        <SessionEndedBanner kind={hostUnreachable ? 'unreachable' : 'reconnecting'} host={hostLabel} />
      )}
      {pathNotice && isActive && !otherBannerRef.current && (
        <SessionEndedBanner kind={pathNotice} onDismiss={clearPathNotice} />
      )}
      {(findBarOpen || findBarExiting) && pluginConfig?.search && (
        <FindBar
          query={searchQuery}
          onQueryChange={handleSearchQueryChange}
          matchCount={matchInfo.count}
          currentMatchIndex={matchInfo.index}
          searchOptions={searchOptions}
          onSearchOptionsChange={handleSearchOptionsChange}
          onNext={handleSearchNext}
          onPrev={handleSearchPrev}
          onClose={handleSearchClose}
          focusSeq={findBarFocusSeq}
          exiting={findBarExiting}
        />
      )}
      {linkConfirmState && (
        <LinkConfirmPopover
          url={linkConfirmState.url}
          risk={linkConfirmState.risk}
          x={linkConfirmState.x}
          y={linkConfirmState.y}
          onContinue={() => {
            openLink(linkConfirmState.url)
            setLinkConfirmState(null)
          }}
          onCancel={() => setLinkConfirmState(null)}
        />
      )}
    </div>
  )
}
