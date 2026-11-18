import React, { useEffect, useState } from 'react'
import type { ITheme } from '@xterm/xterm'
import { daemon } from '../wailsjs/go/models'
import { TerminalPanel } from './TerminalPanel'
import type { ReconnectVerdict } from '../lib/relayClient'
import { shareInfoVerdict } from '../lib/closeReasons'
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
  /** Portty 04-06: host label for the stopped-sharing banner (local tab state or cap-verified session info). */
  hostLabel?: string
  /** Portty 04-06: the share stopped (close reason constant); the caller drops the sidebar row. */
  onShareStopped?: () => void
  /** Portty 05-05: App's pre-redial re-auth for peer tabs (remote only). */
  beforeReconnect?: () => Promise<ReconnectVerdict>
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
  hostLabel,
  onShareStopped,
  beforeReconnect,
}: WebShareSessionViewProps): React.ReactElement {
  // FIX-03 (plan 09) RC-A transport switch. When true, the terminal routes
  // through the daemon proxy (remote:true, wsURL undefined).
  const useProxy = remote === true

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
  const probeShare = (): Promise<ReconnectVerdict> =>
    fetch(`${apiBaseURL}/api/sessions/${encodeURIComponent(sessionId)}/info?cap=${encodeURIComponent(capToken)}`, { cache: 'no-store', signal: AbortSignal.timeout(10_000) })
      .then((res) => shareInfoVerdict(res.status), () => ({ kind: 'retry' }))

  // Phase 168 FIX-01 (#112) — web-guest plugin-config self-fetch + SSE
  // hot-swap. Restores the config that only ever arrived via a Wails
  // `pluginConfig` prop pre-Phase-159; a web guest (or FIX-03's in-app
  // remote-peer tab, which has no local pluginConfig for a different peer)
  // has no such prop and must self-fetch from the same origin the wsURL
  // above targets.
  const [livePluginConfig, setLivePluginConfig] = useState<PluginSettings | null>(null)
  const isWebGuest = pluginConfig === undefined

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
      {/* isActive={true}: constant on web-share (no grow animation). */}
      <TerminalPanel
        sessionId={sessionId}
        isActive={true}
        relayPort={relayPort}
        fontFamily={effectiveFontFamily}
        fontSize={effectiveFontSize}
        onFontSizeChange={() => {}}
        theme={theme ?? {}}
        pluginConfig={effectivePluginConfig}
        remote={useProxy}
        wsURL={useProxy ? undefined : wsURL}
        hostLabel={hostLabel}
        onShareStopped={onShareStopped}
        beforeReconnect={useProxy ? beforeReconnect : probeShare}
      />
    </div>
  )
}
