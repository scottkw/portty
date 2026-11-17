// TS half of the close-reason pin; the Go half is internal/relay/close_reasons_test.go.
import { describe, it, expect } from 'vitest'
import { CLOSE_REASON_SHARE_STOPPED, CLOSE_REASON_SESSION_ENDED } from '../closeReasons'

describe('close reasons', () => {
  it('match the Go literals exactly', () => {
    expect(CLOSE_REASON_SHARE_STOPPED).toBe('share stopped')
    expect(CLOSE_REASON_SESSION_ENDED).toBe('session ended')
  })
})
