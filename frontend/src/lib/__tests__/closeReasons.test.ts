// TS half of the close-reason pin; the Go half is internal/relay/close_reasons_test.go.
import { describe, it, expect } from 'vitest'
import { CLOSE_REASON_OTHER_ACCOUNT, CLOSE_REASON_SHARE_STOPPED, CLOSE_REASON_SESSION_ENDED, isTerminalClose, peerConnectVerdict, shareInfoVerdict } from '../closeReasons'

describe('close reasons', () => {
  it('match the Go literals exactly', () => {
    expect(CLOSE_REASON_SHARE_STOPPED).toBe('share stopped')
    expect(CLOSE_REASON_SESSION_ENDED).toBe('session ended')
  })
})

describe('isTerminalClose', () => {
  it('is true only for 1000 with share stopped or session ended', () => {
    expect(isTerminalClose(1000, 'share stopped')).toBe(true)
    expect(isTerminalClose(1000, 'session ended')).toBe(true)
    expect(isTerminalClose(1000, 'detach')).toBe(false)
    expect(isTerminalClose(1006, 'share stopped')).toBe(false)
    expect(isTerminalClose(1008, 'too slow')).toBe(false)
  })
})

// Portty 05-05 (SHAR-07): verdicts for the re-auth step run before each redial.
describe('peerConnectVerdict', () => {
  it.each(['not_sharing', 'no_such_session', 'forbidden'])('%s stops with share-stopped', (code) => {
    expect(peerConnectVerdict(code)).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })
  it.each(['unreachable', 'tailscale_off', 'unknown_host', 'ambiguous', 'multiple', 'something_new'])('%s retries', (code) => {
    expect(peerConnectVerdict(code)).toEqual({ kind: 'retry' })
  })
  it('no_such_session after the session-ended close stops with ended', () => {
    expect(peerConnectVerdict('no_such_session', true)).toEqual({ kind: 'stop', reason: 'ended' })
  })
  it.each(['not_sharing', 'forbidden'])('%s stays share-stopped after the session-ended close', (code) => {
    expect(peerConnectVerdict(code, true)).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })
  it('unreachable still retries after the session-ended close', () => {
    expect(peerConnectVerdict('unreachable', true)).toEqual({ kind: 'retry' })
  })
})

describe('shareInfoVerdict', () => {
  it.each([200, 204])('%i redials', (status) => {
    expect(shareInfoVerdict(status, false)).toEqual({ kind: 'ok' })
  })
  it.each([401, 403])('%i without the marker stops with share-stopped (a revoked link is never retried)', (status) => {
    expect(shareInfoVerdict(status, false)).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })
  it('401 with the marker is still share-stopped', () => {
    expect(shareInfoVerdict(401, true)).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })
  it('403 with the X-Portty marker stops with other-account (MOB-05)', () => {
    expect(shareInfoVerdict(403, true)).toEqual({ kind: 'stop', reason: 'other-account' })
  })
  it('404 stops with ended, marker or not', () => {
    expect(shareInfoVerdict(404, false)).toEqual({ kind: 'stop', reason: 'ended' })
    expect(shareInfoVerdict(404, true)).toEqual({ kind: 'stop', reason: 'ended' })
  })
  it.each([500, 502, 0])('%i retries', (status) => {
    expect(shareInfoVerdict(status, false)).toEqual({ kind: 'retry' })
  })
})

describe('CLOSE_REASON_OTHER_ACCOUNT', () => {
  it('is client-only: never a terminal close from the wire', () => {
    expect(CLOSE_REASON_OTHER_ACCOUNT).toBe('other account')
    expect(isTerminalClose(1000, CLOSE_REASON_OTHER_ACCOUNT)).toBe(false)
  })
})
