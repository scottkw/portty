import { describe, it, expect } from 'vitest'
import { isAttentionStatus, normalizeStatus, statusLabel } from './hubStatus'
import type { HubStatus } from './hubStatus'

describe('isAttentionStatus', () => {
  it('returns true for waiting', () => {
    const status: HubStatus = 'waiting'
    expect(isAttentionStatus(status)).toBe(true)
  })

  it('returns true for errored', () => {
    const status: HubStatus = 'errored'
    expect(isAttentionStatus(status)).toBe(true)
  })

  it('returns true for stopped-err', () => {
    const status: HubStatus = 'stopped-err'
    expect(isAttentionStatus(status)).toBe(true)
  })

  it('returns false for running', () => {
    const status: HubStatus = 'running'
    expect(isAttentionStatus(status)).toBe(false)
  })

  it('returns false for idle', () => {
    const status: HubStatus = 'idle'
    expect(isAttentionStatus(status)).toBe(false)
  })

  it('returns false for stopped-ok', () => {
    const status: HubStatus = 'stopped-ok'
    expect(isAttentionStatus(status)).toBe(false)
  })
})

describe('normalizeStatus / statusLabel', () => {
  it.each([
    ['running', 'running', 'Running'],
    ['exited', 'exited', 'Exited'],
    ['errored', 'errored', 'Error'],
    ['idle', 'running', 'Running'],
    ['waiting', 'running', 'Running'],
    ['', 'running', 'Running'],
    [undefined, 'running', 'Running'],
    ['bogus', 'running', 'Running'],
  ])('%j → %s (%s)', (raw, want, label) => {
    const s = normalizeStatus(raw)
    expect(s).toBe(want)
    expect(statusLabel(s)).toBe(label)
  })
})
