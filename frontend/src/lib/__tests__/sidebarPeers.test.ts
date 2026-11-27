import { describe, it, expect } from 'vitest'
import { DESKTOP_ROW_LINES, PEER_COLOR_COUNT, desktopConnectHost, peerColorIndex, peerView, shortHost } from '../sidebarPeers'
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

  // Quick 261116-c9f: an idle reachable peer is kept and reads "Nothing shared".
  it('a reachable peer with zero sessions is kept as a group', () => {
    const alone = groupsOf(peerView(false, true, [peer('a.ts.net', true)]))
    expect(alone).toHaveLength(1)
    expect(alone[0].reachable).toBe(true)
    expect(alone[0].peer.sessions).toEqual([])
    const g = groupsOf(peerView(false, true, [peer('b.ts.net', true, ['1']), peer('a.ts.net', true)]))
    expect(g.map((x) => x.key)).toEqual(['a.ts.net', 'b.ts.net'])
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

describe('peerColorIndex: one stable colour per Mac (UI polish)', () => {
  it('is stable: the same hostname always gets the same index, whatever the case', () => {
    const a = peerColorIndex('office-air.tail1234.ts.net')
    expect(peerColorIndex('office-air.tail1234.ts.net')).toBe(a)
    expect(peerColorIndex('Office-Air.tail1234.ts.net')).toBe(a)
    // Pinned so a hash change (which would recolour every Mac) is a deliberate test edit.
    expect([peerColorIndex('office-air.tail1234.ts.net'), peerColorIndex('home-air.tail1234.ts.net')]).toEqual([0, 3])
  })

  it('stays inside the palette and spreads across all of it', () => {
    const seen = new Set<number>()
    for (let i = 0; i < 40; i++) {
      const n = peerColorIndex(`mac-${i}.tail1234.ts.net`)
      expect(n).toBeGreaterThanOrEqual(0)
      expect(n).toBeLessThan(PEER_COLOR_COUNT)
      seen.add(n)
    }
    expect(seen.size).toBe(PEER_COLOR_COUNT)
  })
})

describe('peerView: a reconnecting Mac stays listed (UI polish)', () => {
  it('adds the reconnecting host as unreachable when discovery has dropped it', () => {
    const g = groupsOf(peerView(false, true, [], 'office-air.tail1234.ts.net'))
    expect(g.map((x) => [x.key, x.host, x.reachable])).toEqual([['office-air.tail1234.ts.net', 'office-air', false]])
  })

  it('leaves a listed host as the poll reports it, and ignores a null host', () => {
    const listed = [peer('office-air.tail1234.ts.net', true, ['1'])]
    expect(groupsOf(peerView(false, true, listed, 'office-air.tail1234.ts.net'))).toHaveLength(1)
    expect(groupsOf(peerView(false, true, listed, 'office-air.tail1234.ts.net'))[0].reachable).toBe(true)
    expect(peerView(false, true, [], null)).toEqual({ kind: 'none' })
    expect(peerView(true, true, [], 'office-air.tail1234.ts.net')).toEqual({ kind: 'off' })
  })
})

describe('desktop row helpers (DVWR-01)', () => {
  it('has the five UI-SPEC lines', () => {
    expect(DESKTOP_ROW_LINES).toEqual({
      needs_permission: 'Needs permission',
      awaiting_approval: 'Waiting for approval at the Mac',
      display_asleep: 'Display asleep',
      locked: 'Locked',
      capture_paused: 'Desktop paused',
    })
  })

  it('desktopConnectHost prefers the MagicDNS name, else the hostname', () => {
    expect(desktopConnectHost({ ...peer('office-air', true), dnsName: 'office-air.example.ts.net' })).toBe('office-air.example.ts.net')
    expect(desktopConnectHost(peer('office-air', true))).toBe('office-air')
  })

  it('the synthetic reconnecting peer has no desktop', () => {
    expect(groupsOf(peerView(false, true, [], 'office-air.example.ts.net'))[0].peer.desktop).toBe(false)
  })
})
