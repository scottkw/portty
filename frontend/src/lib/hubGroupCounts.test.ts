import { describe, it, expect } from 'vitest'
import { computeCounts, computeGlobalCounts } from './hubGroupCounts'
import { memberKey } from './hubGroups'
import type { SessionInfo } from '../wailsjs/go/main/App'

// Minimal SessionInfo fixture factory. Only name and workDir matter to the
// counts; state/status/exitCode vary to prove they are ignored.
function makeSession(
  overrides: Partial<SessionInfo> & Pick<SessionInfo, 'name' | 'workDir'>
): SessionInfo {
  return {
    id: 'test-id',
    name: overrides.name,
    state: overrides.state ?? 'running',
    status: overrides.status ?? 'running',
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
  it('returns total 0 for an empty session list', () => {
    expect(computeGlobalCounts([])).toEqual({ total: 0 })
  })

  it('counts every session regardless of state or exit code (no status, CONTEXT Amendment 2)', () => {
    const sessions = [
      makeSession({ name: 's1', workDir: '/proj' }),
      makeSession({ name: 's2', workDir: '/proj', status: 'idle' }),
      makeSession({ name: 's3', workDir: '/proj', state: 'stopped', status: 'errored', exitCode: 1 }),
    ]
    expect(computeGlobalCounts(sessions)).toEqual({ total: 3 })
  })
})

describe('computeCounts', () => {
  it('excludes sessions whose memberKey is not in the Set (total stays at 0)', () => {
    const sessions = [makeSession({ name: 'other', workDir: '/other' })]
    const keys = new Set<string>([memberKey('mine', '/mine')])
    expect(computeCounts(sessions, keys)).toEqual({ total: 0 })
  })

  it('counts only sessions whose memberKey is in the Set', () => {
    const sessions = [
      makeSession({ name: 's1', workDir: '/proj' }),
      makeSession({ name: 's2', workDir: '/other' }),
      makeSession({ name: 's3', workDir: '/proj', state: 'stopped', exitCode: 2 }),
    ]
    const keys = new Set<string>([memberKey('s1', '/proj'), memberKey('s3', '/proj')])
    expect(computeCounts(sessions, keys)).toEqual({ total: 2 })
  })
})
