import React, { useEffect, useRef, useState } from 'react'
import type { ITheme } from '@xterm/xterm'
import { daemon } from '../wailsjs/go/models'
import { TerminalPanel } from './TerminalPanel'
import type { ReconnectVerdict } from '../lib/relayClient'
import type { PreviewEvent } from '../lib/previewAssembler'
import { shareInfoVerdict } from '../lib/closeReasons'
import { KeyBar } from './KeyBar'
import { ctrlByte } from '../lib/ctrlMap'
import { usePhone } from '../lib/phoneQuery'
import { DEFAULT_TERMINAL_FONT_SIZE, parseTerminalFont, terminalFontFamily } from '../lib/terminalFont'

type PluginSettings = daemon.PluginSettings

export interface WebShareSessionViewProps {
  /** Session to display. */
  sessionId: string
  /** Web-share capability token from URL ?cap= param. Never decoded client-side. */
  capToken: string
  /**
   * Relay port. 0 only for the native web-share guest path (safe sentinel,
   * ignored when wsURL is present). For FIX-03 in-app remote-peer tabs
   * (`remote===true`) this MUST be the desktop daemon relay port — RelayClient
   * builds `ws://127.0.0.1:{relayPort}/api/relay/remote/{id}/ws` from it.
   */
  relayPort: number
  /**
   * FIX-03 (plan 09, RC-A): true when this is an in-app remote-peer tab. Routes
   * the terminal through the daemon WS proxy (remote:true, NO wsURL) — the same
   * transport the working Phase 134 HubInteractiveModal uses — instead of a
   * direct cross-origin `wss://<peer>/sessions/{id}/ws?cap=` the peer's byte-exact
   * Origin allowlist 403s.
   */
  remote?: boolean
  /** Terminal colour theme (pass terminalTheme from App.tsx). */
  theme?: ITheme
  /** Plugin configuration (pass pluginConfig from App.tsx). */
  pluginConfig?: PluginSettings | null
  /**
   * Origin to derive apiBaseURL/wsURL from (e.g. `https://peer.example.ts.net`).
   * Defaults to `window.location.origin` — the native web-share guest path.
   * FIX-03 (plan 03) supplies a different peer's origin for in-app remote-peer
   * tabs; this seam MUST stay parameterized (never hardcode window.location
   * inside URL-building logic below).
   */
  baseURL?: string
  /**
   * Quick 261116-ku9 R1: the viewer's own Terminal Font (desktop in-app remote
   * tabs; the font must be installed on the viewing Mac). Absent on web
   * guests, which use the host's font from /api/plugin-config.
   */
  fontFamily?: string
  fontSize?: number
  /**
   * INPT-06: a desktop peer tab's Cmd+/Cmd− (App keeps the size per tab), which
   * makes the panel claim the session grid. Absent on the browser viewer, which
   * zooms with the browser (a window resize, also a claim).
   */
  onFontSizeChange?: (delta: number) => void
  /** Portty 04-06: host label for the stopped-sharing banner (local tab state or cap-verified session info). */
  hostLabel?: string
  /** Portty 04-06: the share stopped (close reason constant); the caller drops the sidebar row. */
  onShareStopped?: () => void
  /** Portty 14-04 (MOB-05): passed through to TerminalPanel; the share server refused this device's Tailscale account. */
  onOtherAccount?: () => void
  /** Portty 05-05: App's pre-redial re-auth for peer tabs (remote only). */
  beforeReconnect?: () => Promise<ReconnectVerdict>
  /** Passed through to TerminalPanel: the reconnecting banner shows (true) or hides (false). */
  onReconnectingChange?: (reconnecting: boolean) => void
  /** DEBT-04: passed through to TerminalPanel, the host half of a peer tab's snapshot key. */
  snapshotHost?: string
  /** Phase 11: passed through to TerminalPanel, this view's preview events. */
  onPreview?: (event: PreviewEvent) => void
  /** Phase 11: false while App keeps this view mounted but hidden behind one of its previews. Default true. */
  isActive?: boolean
  /** Portty 14-10: false when the capability has no write (no key bar); undefined (info call failed) shows it, the server still drops the input. */
  canWrite?: boolean
}

// Review it3 IN-02: AbortSignal.timeout is missing before Safari 16 / Chrome 103, and
// calling it there would throw every probe into a retry, forever.
// B5: a browser cannot tell a refused probe (host daemon dead) from an unreachable or
// sleeping host, so it never stops on one; after this long failing, the banner says so.
const LONG_OUTAGE_MS = 120_000

function timeoutSignal(ms: number): AbortSignal {
  if (typeof AbortSignal.timeout === 'function') return AbortSignal.timeout(ms)
  const ctrl = new AbortController()
  setTimeout(() => ctrl.abort(), ms)
  return ctrl.signal
}

/**
 * WebShareSessionView mounts the TerminalPanel for the web-share browser surface
 * and for in-app remote-peer tabs. The native web-guest path connects via the
 * webserver WS endpoint (`wss://`) using the wsURL override (Phase 155); remote
 * tabs go through the daemon proxy. Terminal only: chat was removed (CHAT-01).
 */
export function WebShareSessionView({
  sessionId,
  capToken,
  relayPort,
  theme,
  pluginConfig,
  baseURL,
  remote,
  fontFamily,
  fontSize,
  onFontSizeChange,
  hostLabel,
  onShareStopped,
  onOtherAccount,
  beforeReconnect,
  onReconnectingChange,
  snapshotHost,
  onPreview,
  isActive = true,
  canWrite,
}: WebShareSessionViewProps): React.ReactElement {
  // FIX-03 (plan 09) RC-A transport switch. When true, the terminal routes
  // through the daemon proxy (remote:true, wsURL undefined).
  const useProxy = remote === true

  // Portty 14-10 (MOB-01): the phone key bar and its one-shot Ctrl. The ref is what the input
  // transform reads (it runs inside xterm's onData, outside React's render); the state drives the key.
  const keyInputRef = useRef<{ send(seq: string): void; applicationCursor(): boolean } | null>(null)
  const ctrlRef = useRef(false)
  const [ctrlArmed, setCtrlArmedState] = useState(false)
  const setCtrl = (armed: boolean) => { ctrlRef.current = armed; setCtrlArmedState(armed) }
  const phone = usePhone()
  const showKeyBar = !useProxy && phone && isActive && canWrite !== false
  // A typed character gets the armed Ctrl once; a character with no control code goes as typed.
  const inputTransform = (data: string): string => {
    if (!ctrlRef.current) return data
    setCtrl(false)
    return ctrlByte(data) ?? data
  }
  // Ctrl never outlives its view: unmount, or a preview taking focus.
  useEffect(() => { if (!isActive) setCtrl(false) }, [isActive])
  useEffect(() => () => { ctrlRef.current = false }, [])

  // Phase 168 FIX-01/FIX-03 — baseURL seam. Defaults to window.location.origin
  // (today's native web-share guest behavior, unchanged). FIX-03 (plan 03)
  // passes a different peer's origin for in-app remote-peer tabs; apiBaseURL
  // and wsURL MUST derive from this resolved origin, never a hardcoded
  // window.location reference, so both plans share the same seam.
  const resolvedOrigin = baseURL ?? window.location.origin
  const apiBaseURL = resolvedOrigin
  // Convert the resolved origin's http/https scheme to ws/wss for the socket
  // (replaces the old hardcoded `wss://${window.location.host}`).
  const wsOrigin = resolvedOrigin.replace(/^http/, 'ws')

  // Phase 155 — webserver WS URL (Pitfall 6: forgetting wsURL on TerminalPanel
  // leaves the terminal on ws://127.0.0.1:0).
  const wsURL = `${wsOrigin}/sessions/${encodeURIComponent(sessionId)}/ws?cap=${encodeURIComponent(capToken)}`

  // Portty 05-05 (SHAR-07): browser path, before each redial ask the share server
  // whether the cap still works, so a revoked link stops instead of retrying.
  // WR-06: a probe to a sleeping host times out into a retry, never a hang.
  // B5: failingSince is the first failed fetch since the last answer; any HTTP answer resets it.
  const failingSince = useRef<number | null>(null)
  const [hostUnreachable, setHostUnreachable] = useState(false)
  const probeShare = (): Promise<ReconnectVerdict> =>
    fetch(`${apiBaseURL}/api/sessions/${encodeURIComponent(sessionId)}/info?cap=${encodeURIComponent(capToken)}`, { cache: 'no-store', signal: timeoutSignal(10_000) })
      .then(
        (res) => {
          failingSince.current = null
          setHostUnreachable(false)
          return shareInfoVerdict(res.status, res.headers.get('X-Portty') === '1')
        },
        () => {
          const now = Date.now()
          failingSince.current ??= now
          if (now - failingSince.current >= LONG_OUTAGE_MS) setHostUnreachable(true)
          return { kind: 'retry' }
        },
      )

  // Phase 168 FIX-01 (#112) — web-guest plugin-config self-fetch + SSE hot-swap, for the browser
  // viewer at /app/ only: it is same-origin with the share server, so it keeps the host's config and
  // live updates. A remote desktop peer tab never self-fetches (Phase 12 gap 1): from the app's
  // wails:// origin the request is cross-origin and the share server sends no CORS header, so it
  // always failed and its EventSource retried forever. App hands a peer tab this Mac's link
  // settings instead.
  const [livePluginConfig, setLivePluginConfig] = useState<PluginSettings | null>(null)
  const isWebGuest = pluginConfig === undefined && remote !== true

  useEffect(() => {
    if (!isWebGuest) return

    const ctrl = new AbortController()
    fetch(`${apiBaseURL}/api/plugin-config?cap=${encodeURIComponent(capToken)}`, {
      signal: ctrl.signal,
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((cfg) => {
        if (cfg) setLivePluginConfig(cfg as PluginSettings)
      })
      .catch(() => {
        // Network error / abort — fall through to the built-in TerminalPanel
        // defaults (matches the /api/plugin-config 503 fallback contract).
      })

    const es = new EventSource(`${apiBaseURL}/api/plugin-config/stream?cap=${encodeURIComponent(capToken)}`)
    es.addEventListener('plugin-config', (ev) => {
      try {
        setLivePluginConfig(JSON.parse((ev as MessageEvent).data) as PluginSettings)
      } catch {
        // Malformed frame — ignore, keep the last-known-good config.
      }
    })

    return () => {
      ctrl.abort()
      es.close()
    }
  }, [isWebGuest, capToken, apiBaseURL])

  // Effective config: the live self-fetched/SSE value takes priority once
  // populated; otherwise fall back to the Wails-provided prop (desktop path).
  const effectivePluginConfig = livePluginConfig ?? pluginConfig

  // Font: explicit props (viewer's setting), else the host's terminalFont from
  // the fetched/SSE config, else the defaults.
  const hostFont = parseTerminalFont((livePluginConfig as { terminalFont?: unknown } | null)?.terminalFont)
  const effectiveFontFamily = fontFamily ?? (hostFont ? terminalFontFamily(hostFont.family) : terminalFontFamily(''))
  const effectiveFontSize = fontSize ?? hostFont?.size ?? DEFAULT_TERMINAL_FONT_SIZE

  return (
    <div className="hub-modal__body hub-modal__body--interactive">
      <TerminalPanel
        sessionId={sessionId}
        isActive={isActive}
        relayPort={relayPort}
        fontFamily={effectiveFontFamily}
        fontSize={effectiveFontSize}
        onFontSizeChange={onFontSizeChange ?? (() => {})}
        theme={theme ?? {}}
        pluginConfig={effectivePluginConfig}
        remote={useProxy}
        wsURL={useProxy ? undefined : wsURL}
        hostLabel={hostLabel}
        onShareStopped={onShareStopped}
        onOtherAccount={onOtherAccount}
        beforeReconnect={useProxy ? beforeReconnect : probeShare}
        hostUnreachable={hostUnreachable}
        onReconnectingChange={onReconnectingChange}
        snapshotHost={snapshotHost}
        onPreview={onPreview}
        keyInputRef={keyInputRef}
        inputTransform={inputTransform}
      />
      {showKeyBar && (
        <KeyBar
          send={(seq) => keyInputRef.current?.send(seq)}
          applicationCursor={() => keyInputRef.current?.applicationCursor() ?? false}
          ctrlArmed={ctrlArmed}
          onCtrlChange={setCtrl}
        />
      )}
    </div>
  )
}
