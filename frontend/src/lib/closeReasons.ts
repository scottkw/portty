import type { ReconnectVerdict } from './relayClient'

// WebSocket close reasons a share's server sends with a normal closure. They
// mirror relay.CloseReasonShareStopped and relay.CloseReasonSessionEnded in
// internal/relay/close_reasons.go; both sides pin the literals in a test.
// Clients branch on them and show fixed copy; the wire text is never rendered.
export const CLOSE_REASON_SHARE_STOPPED = 'share stopped'
export const CLOSE_REASON_SESSION_ENDED = 'session ended'

// True only for a final close: 1000 with one of the two reasons above. Code 1000
// alone is not enough (a client detach is also 1000), and any other code is a
// network drop that a reconnecting client retries (SHAR-07).
export function isTerminalClose(code: number, reason: string): boolean {
  return code === 1000 && (reason === CLOSE_REASON_SHARE_STOPPED || reason === CLOSE_REASON_SESSION_ENDED)
}

// Verdict for a failed ConnectPeerSession run before a GUI redial (errorCode is
// non-empty; the caller handles success). Codes mirror PeerErr* in
// internal/daemon/types.go. no_such_session also means share-stopped: mid-gap the
// daemon cannot tell "Share turned off for this session" from "session ended",
// and "<host> stopped sharing this session." is true for both.
export function peerConnectVerdict(errorCode: string): ReconnectVerdict {
  return errorCode === 'not_sharing' || errorCode === 'no_such_session' || errorCode === 'forbidden'
    ? { kind: 'stop', reason: 'share-stopped' }
    : { kind: 'retry' }
}
