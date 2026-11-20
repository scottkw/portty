/**
 * 09-06 (UPGR-02): a host tab's re-auth before each redial. A drop can be the
 * old daemon leaving with 1012 after a handoff (the session lives on), or a
 * crash (the respawned daemon lists no such session).
 */
import { describe, it, expect, vi } from 'vitest'
import { hostReconnectVerdict } from './hostReconnect'

// WR-04: the plain bindings resolve [] and 0 on a failed read; the checked ones reject.
vi.mock('../wailsjs/go/main/App', () => ({
  GetRelayPort: () => Promise.resolve(0),
  ListSessions: () => Promise.resolve([]),
  GetRelayPortChecked: () => Promise.resolve(4321),
  ListSessionsChecked: () => Promise.reject(new Error('read failed')),
}))

const session = (id: string, state: string) => ({ id, state })
const deps = (port: () => Promise<number>, list: () => Promise<{ id: string; state: string }[]>) => ({
  GetRelayPortChecked: port,
  ListSessionsChecked: list,
})

describe('hostReconnectVerdict', () => {
  it('retries when the relay port cannot be read (daemon still coming up)', async () => {
    const v = await hostReconnectVerdict('s1', deps(() => Promise.reject(new Error('down')), () => Promise.resolve([session('s1', 'running')])))
    expect(v).toEqual({ kind: 'retry' })
  })

  it('retries when the session list cannot be read', async () => {
    const v = await hostReconnectVerdict('s1', deps(() => Promise.resolve(4321), () => Promise.reject(new Error('down'))))
    expect(v).toEqual({ kind: 'retry' })
  })

  it('stops (ended) when the daemon no longer lists the session: a crash and respawn', async () => {
    const v = await hostReconnectVerdict('s1', deps(() => Promise.resolve(4321), () => Promise.resolve([session('s2', 'running')])))
    expect(v).toEqual({ kind: 'stop', reason: 'ended' })
  })

  it('stops (ended) when the session is stopped', async () => {
    const v = await hostReconnectVerdict('s1', deps(() => Promise.resolve(4321), () => Promise.resolve([session('s1', 'stopped')])))
    expect(v).toEqual({ kind: 'stop', reason: 'ended' })
  })

  it('reads the checked bindings by default, so a failed read retries instead of ending the tab (WR-04)', async () => {
    expect(await hostReconnectVerdict('s1')).toEqual({ kind: 'retry' })
  })

  it('dials the re-read port when the session is still running (a handoff)', async () => {
    const v = await hostReconnectVerdict('s1', deps(() => Promise.resolve(4321), () => Promise.resolve([session('s1', 'running')])))
    expect(v).toEqual({ kind: 'ok', port: 4321 })
  })
})
