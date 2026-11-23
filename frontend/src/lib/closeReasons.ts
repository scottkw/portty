import type { ReconnectVerdict } from './relayClient'

// WebSocket close reasons a share's server sends with a normal closure. They
// mirror relay.CloseReasonShareStopped and relay.CloseReasonSessionEnded in
// internal/relay/close_reasons.go; both sides pin the literals in a test.
// Clients branch on them and show fixed copy; the wire text is never rendered.
export const CLOSE_REASON_SHARE_STOPPED = 'share stopped'
export const CLOSE_REASON_SESSION_ENDED = 'session ended'

// Client-only close reason: the reconnect probe was refused because this device is
// on another Tailscale account (MOB-05). No server ever sends it, so it has no Go
// mirror and isTerminalClose does not match it; the client's own verdict mapping
// produces it and TerminalPanel branches on it to show the refusal screen.
export const CLOSE_REASON_OTHER_ACCOUNT = 'other account'

// True only for a final close: 1000 with one of the two reasons above. Code 1000
// alone is not enough (a client detach is also 1000), and any other code is a
// network drop that a reconnecting client retries (SHAR-07).
export function isTerminalClose(code: number, reason: string): boolean {
  return code === 1000 && (reason === CLOSE_REASON_SHARE_STOPPED || reason === CLOSE_REASON_SESSION_ENDED)
}

// Verdict for a failed ConnectPeerSession run before a GUI redial (errorCode is
// non-empty; the caller handles success). Codes mirror PeerErr* in
// internal/daemon/types.go. Mid-gap the daemon cannot tell "Share turned off for
// this session" from "session ended", so no_such_session reads as share-stopped,
// except for a tab that already saw the session-ended close (sawEnded): that
// session is gone, not unshared (todo 2026-11-21-ended-peer-tab-banner).
export function peerConnectVerdict(errorCode: string, sawEnded = false): ReconnectVerdict {
  if (errorCode === 'no_such_session' && sawEnded) return { kind: 'stop', reason: 'ended' }
  return errorCode === 'not_sharing' || errorCode === 'no_such_session' || errorCode === 'forbidden'
    ? { kind: 'stop', reason: 'share-stopped' }
    : { kind: 'retry' }
}

// Verdict for GET /api/sessions/{id}/info?cap= run before a browser redial. 401
// and 403 mean the cap is revoked (never retried); 404 means the session is gone;
// anything else (5xx, network error as 0) may heal, so retry. A 403 that carries
// the X-Portty marker (porttyMarker) is the same-account refusal, not a revoked
// cap: it stops with 'other-account'. The marker is a required argument so a call
// site cannot forget it and read a refusal as a dead link.
export function shareInfoVerdict(status: number, porttyMarker: boolean): ReconnectVerdict {
  if (status >= 200 && status < 300) return { kind: 'ok' }
  if (status === 403 && porttyMarker) return { kind: 'stop', reason: 'other-account' }
  if (status === 401 || status === 403) return { kind: 'stop', reason: 'share-stopped' }
  if (status === 404) return { kind: 'stop', reason: 'ended' }
  return { kind: 'retry' }
}
