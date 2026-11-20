/**
 * 09-06 (UPGR-02): a host tab's re-auth before each redial. A drop can be the
 * old daemon leaving with 1012 after a handoff (the session lives on), or a
 * crash (the respawned daemon lists no such session).
 */
import { describe, it, expect } from 'vitest'
import { hostReconnectVerdict } from './hostReconnect'

const session = (id: string, state: string) => ({ id, state })
const deps = (port: () => Promise<number>, list: () => Promise<{ id: string; state: string }[]>) => ({
  GetRelayPort: port,
  ListSessions: list,
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

  it('dials the re-read port when the session is still running (a handoff)', async () => {
    const v = await hostReconnectVerdict('s1', deps(() => Promise.resolve(4321), () => Promise.resolve([session('s1', 'running')])))
    expect(v).toEqual({ kind: 'ok', port: 4321 })
  })
})
