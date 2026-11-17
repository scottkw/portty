// WebSocket close reasons a share's server sends with a normal closure. They
// mirror relay.CloseReasonShareStopped and relay.CloseReasonSessionEnded in
// internal/relay/close_reasons.go; both sides pin the literals in a test.
// Clients branch on them and show fixed copy; the wire text is never rendered.
export const CLOSE_REASON_SHARE_STOPPED = 'share stopped'
export const CLOSE_REASON_SESSION_ENDED = 'session ended'
