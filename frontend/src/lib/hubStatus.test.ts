import { describe, it, expect } from 'vitest'
import { deriveHubStatus, isAttentionStatus, normalizeStatus, statusLabel } from './hubStatus'
import type { SessionInfo } from '../wailsjs/go/main/App'
import type { HubStatus } from './hubStatus'

describe('isAttentionStatus', () => {
  it.each<[HubStatus, boolean]>([
    ['errored', true],
    ['running', false],
    ['exited', false],
  ])('%s → %s', (status, want) => {
    expect(isAttentionStatus(status)).toBe(want)
  })
})

describe('deriveHubStatus', () => {
  const base = { status: 'running', state: 'running' } as SessionInfo
  it.each<[Partial<SessionInfo>, HubStatus]>([
    [{ state: 'running', status: 'running' }, 'running'],
    [{ state: 'running', status: 'idle' }, 'running'],
    [{ state: 'running', status: 'waiting' }, 'running'],
    [{ state: 'running', status: 'errored' }, 'errored'],
    [{ state: 'stopped', status: 'exited', exitCode: 0 }, 'exited'],
    [{ state: 'stopped', status: 'errored', exitCode: 3 }, 'errored'],
    [{ state: 'stopped', status: 'running' }, 'exited'],
  ])('%j → %s', (fields, want) => {
    expect(deriveHubStatus({ ...base, ...fields } as SessionInfo)).toBe(want)
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
