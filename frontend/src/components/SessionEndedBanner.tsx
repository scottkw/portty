import React, { useEffect } from 'react'
import { InformationCircleIcon, XMarkIcon } from '@heroicons/react/24/outline'

/**
 * SessionEndedBanner — Phase 175-06 (BUG-02 / #125).
 *
 * Rendered on the guest (remote viewer) path when the RelayClient WebSocket
 * closes because the owner ended/stopped the shared session — the server's
 * write pump calls conn.Close(StatusNormalClosure, "session ended") on
 * hub.Done() (both internal/webserver/server.go and internal/relay/server.go).
 * Without this banner a remote viewer just sees a frozen, silently dead
 * terminal with zero explanation.
 *
 * Mirrors WebGLRecoveryBanner's accessible-banner pattern: role="status" +
 * aria-live="polite" + fixed copy + dismiss button.
 *
 * Final closes (share stopped, session ended) are still never retried: the
 * session or the share is genuinely gone. Since Phase 5 (SHAR-07) a network
 * drop reconnects instead, and the panel shows kind 'reconnecting' while it
 * does (no dismiss button; it goes away when the socket reopens). Every kind
 * has fixed copy, and the wire reason is still never rendered.
 *
 * DEBT-05: kind 'paste-interrupted' says a split paste was cut short by a socket
 * drop. Its numbers are computed on the client from bytes it queued, never taken
 * from wire text, so the rule above holds.
 *
 * Phase 12 (PATH-02): kinds 'path-unsupported' and 'path-too-large' tell the clicking pane the host
 * refused a clicked file. The kind is chosen from the host's two exact notice codes, so the copy is
 * fixed and the wire value is never rendered.
 *
 * Phase 13 (URL-01): the five 'port-*' kinds tell the clicking pane why a remote localhost URL did not open.
 * They are chosen from the binding's error codes and show only the host label and a numeric port.
 *
 * Colorblind-safe (MEMORY: user is colorblind): the message is conveyed via
 * text + an accessible status role, never via color alone.
 *
 * Security (T-175-06-02): `reason` is the raw WebSocket CloseEvent.reason —
 * untrusted text from the wire. It is accepted here ONLY for logging/
 * branching (never rendered into the DOM, never passed to
 * dangerouslySetInnerHTML). The visible copy is always the fixed message
 * below, regardless of what `reason` contains.
 */
export interface SessionEndedBannerProps {
  /** Renders the Dismiss button when given; kinds 'reconnecting' and 'unreachable' never have one. */
  onDismiss?: () => void
  /**
   * Phase 14 (MOB-03): renders a fixed-label "Reload" button, only for the kinds a phone cannot
   * leave any other way: 'share-stopped', 'ended', 'unreachable' and 'reconnecting'. Never for the
   * paste, path or port notices. The caller decides when (a phone; reconnecting only while stuck).
   */
  onReload?: () => void
  /**
   * Raw WebSocket CloseEvent.reason, if any. Used only for logging/
   * branching — see the Security note above. Never rendered.
   */
  reason?: string
  className?: string
  /**
   * Portty 04-06: 'share-stopped' when the close reason was the share-stopped
   * constant (the host turned Share off). Chosen by the caller from the
   * constant, never from wire text. Phase 5 (SHAR-07): 'reconnecting' while a
   * dropped guest socket is being replaced; B5: 'unreachable' once the browser's
   * probes have failed for minutes (still reconnecting, never final).
   */
  kind?:
    | 'ended' | 'share-stopped' | 'reconnecting' | 'unreachable' | 'paste-interrupted' | 'path-unsupported' | 'path-too-large'
    | 'port-host-outdated' | 'port-daemon-outdated' | 'port-in-use' | 'port-refused' | 'port-unreachable'
  /** Local host label (peer tab or cap-verified session info) for 'share-stopped', 'unreachable' and the 'port-*' kinds. */
  host?: string
  /** 'port-in-use' and 'port-refused': the port number clicked. */
  port?: number
  /** 'paste-interrupted': KB of the paste that left the browser; absent when the host never confirmed it. */
  pasteSentKB?: number
  /** 'paste-interrupted': the paste's size in KB. */
  pasteTotalKB?: number
}

const RELOAD_KINDS = ['share-stopped', 'ended', 'unreachable', 'reconnecting']

export function SessionEndedBanner({
  onDismiss,
  onReload,
  reason,
  className,
  kind = 'ended',
  host,
  pasteSentKB,
  pasteTotalKB,
  port,
}: SessionEndedBannerProps): React.ReactElement {
  useEffect(() => {
    if (reason) {
      // Logging only (T-175-06-02) — the fixed message below is the only
      // text ever rendered into the DOM for this banner.
      console.debug(`[SessionEndedBanner] close reason: ${reason}`)
    }
  }, [reason])

  const cls = ['banner', 'banner--info', 'session-ended-banner', className].filter(Boolean).join(' ')

  return (
    <div className={cls} role="status" aria-live="polite">
      <InformationCircleIcon className="banner__icon" aria-hidden="true" />
      <span className="banner__message">
        {kind === 'paste-interrupted'
          ? (pasteSentKB !== undefined
              ? `Paste interrupted: ${pasteSentKB} of ${pasteTotalKB} KB sent`
              : `Paste may have been cut short (${pasteTotalKB} KB)`)
          : kind === 'path-unsupported'
            ? "Can't preview this file type. Portty previews images, PDFs and Markdown files."
          : kind === 'path-too-large'
            ? 'Too large to preview. The limit is 20 MB.'
          : kind === 'port-host-outdated'
            ? `${host || 'The host Mac'} needs a Portty update to open its local pages.`
          : kind === 'port-daemon-outdated'
            ? "Portty's background service on this Mac is too old to open remote pages. Relaunch Portty to update it."
          : kind === 'port-in-use'
            ? `Port ${port} is in use on this Mac.`
          : kind === 'port-refused'
            ? `${host || 'The host Mac'} refused access to port ${port}.`
          : kind === 'port-unreachable'
            ? `Couldn't reach ${host || 'the host Mac'}.`
          : kind === 'reconnecting'
            ? 'Connection lost — reconnecting…'
            : kind === 'unreachable'
              ? `Can't reach ${host || 'the host Mac'}. The session may have ended or the Mac may be asleep. Still retrying…`
              : kind === 'share-stopped'
                ? (host ? `${host} stopped sharing this session.` : 'This session is no longer shared.')
                : 'Session ended — the owner stopped this session.'}
      </span>
      {onReload && RELOAD_KINDS.includes(kind) && (
        <button type="button" className="banner__action" onClick={onReload}>Reload</button>
      )}
      {onDismiss && kind !== 'reconnecting' && kind !== 'unreachable' && (
        <button
          type="button"
          className="banner__dismiss"
          aria-label="Dismiss"
          onClick={onDismiss}
        >
          <XMarkIcon aria-hidden="true" />
        </button>
      )}
    </div>
  )
}
