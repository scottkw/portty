// TS half of the close-reason pin; the Go half is internal/relay/close_reasons_test.go.
import { describe, it, expect } from 'vitest'
import { CLOSE_REASON_SHARE_STOPPED, CLOSE_REASON_SESSION_ENDED, isTerminalClose } from '../closeReasons'

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
