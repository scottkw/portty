import { describe, it, expect } from 'vitest'
import { peerView, shortHost } from '../sidebarPeers'
import type { RemotePeerSessions } from '../remoteSession'

const peer = (hostname: string, reachable: boolean, ids: string[] = []): RemotePeerSessions => ({
  hostname,
  reachable,
  sessions: ids.map((id) => ({ id, name: `s-${id}`, status: 'running', url: `https://${hostname}:8443/sessions/${id}` })),
})

const groupsOf = (v: ReturnType<typeof peerView>) => {
  if (v.kind !== 'groups') throw new Error(`expected groups, got ${v.kind}`)
  return v.groups
}

describe('shortHost', () => {
  it('returns the first DNS label', () => {
    expect(shortHost('office-air.tail1234.ts.net')).toBe('office-air')
  })

  it('returns a bare hostname unchanged', () => {
    expect(shortHost('home-air')).toBe('home-air')
  })
})

describe('peerView: UI-SPEC peer group states (NAV-02)', () => {
  it('Tailscale off wins over everything', () => {
    expect(peerView(true, true, [peer('a.ts.net', true, ['1'])])).toEqual({ kind: 'off' })
    expect(peerView(true, false, [])).toEqual({ kind: 'off' })
  })

  it('first poll in flight is loading', () => {
    expect(peerView(false, false, [])).toEqual({ kind: 'loading' })
  })

  it('connected with no peers is none', () => {
    expect(peerView(false, true, [])).toEqual({ kind: 'none' })
  })

  it('a reachable peer with zero sessions is hidden (and alone gives none)', () => {
    expect(peerView(false, true, [peer('a.ts.net', true)])).toEqual({ kind: 'none' })
    const g = groupsOf(peerView(false, true, [peer('a.ts.net', true), peer('b.ts.net', true, ['1'])]))
    expect(g.map((x) => x.key)).toEqual(['b.ts.net'])
  })

  it('an unreachable peer survives, even with no sessions', () => {
    const g = groupsOf(peerView(false, true, [peer('down.ts.net', false)]))
    expect(g).toHaveLength(1)
    expect(g[0]).toMatchObject({ key: 'down.ts.net', host: 'down', reachable: false })
  })

  it('a reachable peer with sessions keeps them in the peer order', () => {
    const g = groupsOf(peerView(false, true, [peer('a.ts.net', true, ['z', 'a', 'm'])]))
    expect(g[0].peer.sessions.map((s) => s.id)).toEqual(['z', 'a', 'm'])
    expect(g[0].reachable).toBe(true)
  })

  it('sorts groups A→Z by short host', () => {
    const g = groupsOf(
      peerView(false, true, [peer('zulu.ts.net', true, ['1']), peer('alpha.ts.net', true, ['2']), peer('mike.ts.net', false)]),
    )
    expect(g.map((x) => x.host)).toEqual(['alpha', 'mike', 'zulu'])
  })

  it('two hosts sharing a first label stay two groups with distinct keys, tie broken by full hostname', () => {
    const g = groupsOf(
      peerView(false, true, [peer('air.tail-b.ts.net', true, ['1']), peer('air.tail-a.ts.net', true, ['2'])]),
    )
    expect(g.map((x) => x.host)).toEqual(['air', 'air'])
    expect(g.map((x) => x.key)).toEqual(['air.tail-a.ts.net', 'air.tail-b.ts.net'])
  })
})
