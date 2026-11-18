// TS half of the close-reason pin; the Go half is internal/relay/close_reasons_test.go.
import { describe, it, expect } from 'vitest'
import { CLOSE_REASON_SHARE_STOPPED, CLOSE_REASON_SESSION_ENDED, isTerminalClose, peerConnectVerdict, shareInfoVerdict } from '../closeReasons'

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
})

describe('shareInfoVerdict', () => {
  it.each([200, 204])('%i redials', (status) => {
    expect(shareInfoVerdict(status)).toEqual({ kind: 'ok' })
  })
  it.each([401, 403])('%i stops with share-stopped (a revoked link is never retried)', (status) => {
    expect(shareInfoVerdict(status)).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })
  it('404 stops with ended', () => {
    expect(shareInfoVerdict(404)).toEqual({ kind: 'stop', reason: 'ended' })
  })
  it.each([500, 502, 0])('%i retries', (status) => {
    expect(shareInfoVerdict(status)).toEqual({ kind: 'retry' })
  })
})
