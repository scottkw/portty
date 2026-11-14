import { describe, it, expect } from 'vitest'
import { computeCounts, computeGlobalCounts } from './hubGroupCounts'
import { memberKey } from './hubGroups'
import type { SessionInfo } from '../wailsjs/go/main/App'

// Minimal SessionInfo fixture factory. Only fields relevant to deriveHubStatus
// (state, status, exitCode) and computeCounts (name, workDir) need to be set;
// the rest are filled with safe defaults.
function makeSession(
  overrides: Partial<SessionInfo> & Pick<SessionInfo, 'name' | 'workDir' | 'status' | 'state'>
): SessionInfo {
  return {
    id: 'test-id',
    name: overrides.name,
    state: overrides.state,
    status: overrides.status,
    createdAt: '2026-01-01T00:00:00Z',
    hostname: 'localhost',
    webEnabled: false,
    viewerCount: 0,
    exitCode: overrides.exitCode,
    duration: 0,
    funnelActive: false,
    funnelWriteActive: false,
    workDir: overrides.workDir,
  }
}

describe('computeGlobalCounts', () => {
  it('returns all-zero counts for an empty session list', () => {
    expect(computeGlobalCounts([])).toEqual({ running: 0, total: 0, attention: 0 })
  })

  it('counts a running session toward running and total', () => {
    const sessions = [makeSession({ name: 's1', workDir: '/proj', status: 'running', state: 'running' })]
    expect(computeGlobalCounts(sessions)).toEqual({ running: 1, total: 1, attention: 0 })
  })

  it('counts legacy idle/waiting sessions as running, not attention', () => {
    const sessions = [
      makeSession({ name: 's1', workDir: '/proj', status: 'idle', state: 'running' }),
      makeSession({ name: 's2', workDir: '/proj', status: 'waiting', state: 'running' }),
    ]
    expect(computeGlobalCounts(sessions)).toEqual({ running: 2, total: 2, attention: 0 })
  })

  it('counts a non-zero exit toward attention but NOT running', () => {
    const sessions = [
      makeSession({ name: 's1', workDir: '/proj', status: 'errored', state: 'stopped', exitCode: 1 }),
    ]
    expect(computeGlobalCounts(sessions)).toEqual({ running: 0, total: 1, attention: 1 })
  })

  it('a clean exit counts toward neither running nor attention', () => {
    const sessions = [
      makeSession({ name: 's1', workDir: '/proj', status: 'exited', state: 'stopped', exitCode: 0 }),
    ]
    expect(computeGlobalCounts(sessions)).toEqual({ running: 0, total: 1, attention: 0 })
  })
})

describe('computeCounts', () => {
  it('excludes sessions whose memberKey is not in the Set (total stays at 0)', () => {
    const sessions = [
      makeSession({ name: 'other', workDir: '/other', status: 'running', state: 'running' }),
    ]
    const keys = new Set<string>([memberKey('mine', '/mine')])
    const counts = computeCounts(sessions, keys)
    expect(counts.total).toBe(0)
    expect(counts.running).toBe(0)
  })

  it('includes only sessions whose memberKey is in the Set', () => {
    const sessions = [
      makeSession({ name: 's1', workDir: '/proj', status: 'running', state: 'running' }),
      makeSession({ name: 's2', workDir: '/other', status: 'running', state: 'running' }),
      makeSession({ name: 's3', workDir: '/proj', status: 'errored', state: 'stopped', exitCode: 2 }),
    ]
    // Only s1 and s3 are members (same workDir group)
    const keys = new Set<string>([memberKey('s1', '/proj'), memberKey('s3', '/proj')])
    expect(computeCounts(sessions, keys)).toEqual({ running: 1, total: 2, attention: 1 })
  })

  it('returns total:2 when a 2-member Set is matched against 3 sessions', () => {
    const sessions = [
      makeSession({ name: 'a', workDir: '/x', status: 'running', state: 'running' }),
      makeSession({ name: 'b', workDir: '/x', status: 'idle', state: 'running' }),
      makeSession({ name: 'c', workDir: '/x', status: 'running', state: 'running' }),
    ]
    const keys = new Set<string>([memberKey('a', '/x'), memberKey('b', '/x')])
    const counts = computeCounts(sessions, keys)
    expect(counts.total).toBe(2)
  })
})
