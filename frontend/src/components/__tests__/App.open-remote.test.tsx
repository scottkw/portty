// Portty 04-06 — opening a peer session from the sidebar (SHAR-01, SHAR-04).
//
// A peer row opens (or activates) its '<host> › <session>' tab at once through
// ConnectPeerSession; there is no dialog and no code. A failed connect keeps the
// tab open with a warning banner, and clicking the row again retries. When the
// host stops sharing, the row disappears at once.
//
// Behavior tests mount the real App with the Wails bridge faked (appHarness) and
// WebShareSessionView stubbed (no xterm in jsdom). The stub exposes the props App
// passes and a button that fires onShareStopped.

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

import raw from '../../App.tsx?raw'
import { Sidebar } from '../Sidebar'
import { peerView } from '../../lib/sidebarPeers'
import { adaptRemoteSession } from '../../lib/remoteAdapter'
import type { RemotePeerSessions } from '../../lib/remoteSession'
import { mountApp, unmountApp, flush, wailsEvents } from './appHarness'
import { peerSnapshotKey, putPeerSnapshot, takePeerSnapshot } from '../../lib/peerSnapshots'

// Mount count and the last beforeReconnect verdict, shared with the hoisted stub.
const stub = vi.hoisted(() => ({ mounts: 0, verdict: undefined as unknown, throwFor: '' }))

vi.mock('../WebShareSessionView', () => ({
  WebShareSessionView: (p: { sessionId: string; baseURL?: string; capToken: string; hostLabel?: string; remote?: boolean; onShareStopped?: () => void; beforeReconnect?: () => Promise<unknown>; onReconnectingChange?: (on: boolean) => void; snapshotHost?: string; fontSize?: number; onFontSizeChange?: (delta: number) => void }) => {
    React.useEffect(() => { stub.mounts++ }, [])
    if (p.sessionId === stub.throwFor) throw new SyntaxError(`bad URL wss://x/ws?cap=${p.capToken}`)
    return React.createElement('div', {
      'data-testid': 'web-session-view',
      'data-session': p.sessionId,
      'data-base': p.baseURL ?? '',
      'data-cap': p.capToken,
      'data-host': p.hostLabel ?? '',
      'data-remote': String(p.remote),
      'data-has-reconnect': String(p.beforeReconnect !== undefined),
      'data-snapshot-host': p.snapshotHost ?? '',
      'data-font-size': String(p.fontSize),
    },
    React.createElement('button', { type: 'button', 'data-testid': 'fire-font-up', onClick: () => p.onFontSizeChange?.(+1) }),
    React.createElement('button', { type: 'button', 'data-testid': 'fire-share-stopped', onClick: () => p.onShareStopped?.() }),
    React.createElement('button', { type: 'button', 'data-testid': 'fire-before-reconnect', onClick: () => { void p.beforeReconnect?.().then((v) => { stub.verdict = v }) } }),
    React.createElement('button', { type: 'button', 'data-testid': 'fire-reconnecting', onClick: () => p.onReconnectingChange?.(true) }),
    React.createElement('button', { type: 'button', 'data-testid': 'fire-reconnected', onClick: () => p.onReconnectingChange?.(false) }))
  },
}))

const remotePeer: RemotePeerSessions = {
  hostname: 'office-air.tail1234.ts.net',
  reachable: true,
  sessions: [{ id: 'sess-remote-1', name: 'Remote Session', status: 'idle', url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-1' }],
}

const okResult = {
  sessionId: 'sess-remote-1',
  name: 'Remote Session',
  host: 'office-air',
  url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-1?cap=CAP123',
  errorCode: '',
}

function baseImpl(connect: (...a: unknown[]) => unknown) {
  return {
    GetDaemonError: () => '',
    GetRelayPort: () => 0,
    ListSessions: () => [],
    GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
    GetRemoteSessionsWithMeta: () => [remotePeer],
    ConnectPeerSession: connect,
  }
}

function peerRow(c: HTMLElement): HTMLButtonElement | undefined {
  return Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === 'Remote Session')
}

async function click(el: HTMLElement) {
  await act(async () => { el.click() })
  await flush()
}

const CONNECT_FAILED = "Couldn't connect to office-air. Check that it's awake and signed in to your Tailscale account."

describe('App — peer row opens its tab through ConnectPeerSession (SHAR-01)', () => {
  afterEach(async () => { await unmountApp(); vi.clearAllMocks() })

  it('one click calls ConnectPeerSession(hostname, id) once and opens a <host> › <name> tab, no dialog', async () => {
    const connect = vi.fn(() => okResult)
    const c = await mountApp(baseImpl(connect))
    const row = peerRow(c)
    expect(row, 'peer row listed from the poll').toBeDefined()
    await click(row!)
    expect(connect).toHaveBeenCalledTimes(1)
    expect(connect).toHaveBeenCalledWith('office-air.tail1234.ts.net', 'sess-remote-1')
    expect(c.textContent).toContain('Remote Session · office-air')
    const view = c.querySelector('[data-testid="web-session-view"]')!
    expect(view).not.toBeNull()
    expect(view.getAttribute('data-base')).toBe('https://office-air.tail1234.ts.net:7444')
    expect(view.getAttribute('data-cap')).toBe('CAP123')
    expect(view.getAttribute('data-remote')).toBe('true')
    expect(view.getAttribute('data-host')).toBe('office-air')
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(document.querySelector('input')).toBeNull()
  })

  it('connects by the MagicDNS name when the peer has one (WR-05: HostNames can repeat)', async () => {
    const connect = vi.fn(() => okResult)
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [{ ...remotePeer, hostname: 'Kens-MacBook-Air', dnsName: 'office-air-1.tail1234.ts.net' }] })
    await click(peerRow(c)!)
    expect(connect).toHaveBeenCalledWith('office-air-1.tail1234.ts.net', 'sess-remote-1')
  })

  it('clicking the row of an open tab activates it without connecting again', async () => {
    const connect = vi.fn(() => okResult)
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    await click(peerRow(c)!)
    expect(connect).toHaveBeenCalledTimes(1)
  })

  it('an errorCode opens the tab with the connect-failed banner, and a second click retries', async () => {
    const connect = vi.fn()
      .mockReturnValueOnce({ sessionId: '', name: '', host: '', url: '', errorCode: 'forbidden' })
      .mockReturnValueOnce(okResult)
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    expect(c.textContent).toContain('Remote Session · office-air')
    expect(c.textContent).toContain(CONNECT_FAILED)
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
    await click(peerRow(c)!)
    expect(connect).toHaveBeenCalledTimes(2)
    expect(c.textContent).not.toContain(CONNECT_FAILED)
    expect(c.querySelector('[data-testid="web-session-view"]')).not.toBeNull()
  })

  it('a rejected binding is shown as a failed connect', async () => {
    const connect = vi.fn(() => { throw new Error('bridge down') })
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    expect(c.textContent).toContain(CONNECT_FAILED)
  })

  // Portty 05-09: a second click while the first connect is still in flight
  // used to open a duplicate tab (both clicks passed the stale tabs check).
  it('a double click during a slow connect opens one tab and connects once', async () => {
    let resolve!: (v: unknown) => void
    const connect = vi.fn(() => new Promise((r) => { resolve = r }))
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    await click(peerRow(c)!)
    await act(async () => { resolve(okResult) })
    await flush()
    expect(connect).toHaveBeenCalledTimes(1)
    expect(c.querySelectorAll('[data-testid="web-session-view"]')).toHaveLength(1)
    expect(c.querySelectorAll('[id="tabpanel-__websession__office-air.tail1234.ts.net/sess-remote-1"]')).toHaveLength(1)
    expect(Array.from(c.querySelectorAll('[role="tab"]')).filter((t) => t.textContent?.includes('Remote Session · office-air'))).toHaveLength(1)
    await click(peerRow(c)!) // a third click after it opened only activates it
    expect(connect).toHaveBeenCalledTimes(1)
  })

  it('two different remote sessions clicked back to back open two tabs', async () => {
    const resolvers: ((v: unknown) => void)[] = []
    const connect = vi.fn(() => new Promise((r) => { resolvers.push(r) }))
    const second = { id: 'sess-remote-2', name: 'Second', status: 'idle', url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-2' }
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [{ ...remotePeer, sessions: [...remotePeer.sessions, second] }] })
    const row2 = Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === 'Second')!
    await click(peerRow(c)!)
    await click(row2)
    expect(connect).toHaveBeenCalledTimes(2)
    await act(async () => {
      resolvers[0](okResult)
      resolvers[1]({ ...okResult, sessionId: 'sess-remote-2', name: 'Second', url: okResult.url.replace(/sess-remote-1/g, 'sess-remote-2') })
    })
    await flush()
    const labels = Array.from(c.querySelectorAll('[role="tab"]')).map((t) => t.textContent ?? '')
    expect(labels.filter((l) => l.includes('Remote Session · office-air'))).toHaveLength(1)
    expect(labels.filter((l) => l.includes('Second · office-air'))).toHaveLength(1)
  })

  it('when the host stops sharing, the row disappears at once and the tab stays', async () => {
    const connect = vi.fn(() => okResult)
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-share-stopped"]')!)
    expect(peerRow(c)).toBeUndefined()
    expect(c.textContent).toContain('Remote Session · office-air')
    expect(c.querySelector('[data-testid="web-session-view"]')).not.toBeNull()
  })
})

// DEBT-04: a peer tab's snapshot is keyed by host plus session id, and every tab
// removal drops it. The key the view saves under (snapshotHost) and the key App
// drops must agree, for a MagicDNS peer and a hostname-only peer alike.
describe('App — peer tab snapshots (DEBT-04)', () => {
  const peerA: RemotePeerSessions = { hostname: 'Kens-MacBook-Air', dnsName: 'office-air-1.tail1234.ts.net', reachable: true, sessions: [{ id: 's1', name: 'Alpha', status: 'idle', url: '' }] }
  const peerC: RemotePeerSessions = { hostname: 'studio-mac', reachable: true, sessions: [{ id: 's2', name: 'Charlie', status: 'idle', url: '' }] }
  const connect = vi.fn((host: unknown, id: unknown) => ({ sessionId: id, name: '', host: '', url: `https://${host}:7444/sessions/${id}?cap=C`, errorCode: '' }))
  const snap = { data: 'x', cols: 80, rows: 24, offset: 1 }
  const keys = [peerSnapshotKey('office-air-1.tail1234.ts.net', 's1'), peerSnapshotKey('studio-mac', 's2'), peerSnapshotKey('other-mac', 's1')]

  afterEach(async () => { await unmountApp(); vi.clearAllMocks(); keys.forEach((k) => takePeerSnapshot(k)) })

  const mount = () => mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [peerA, peerC] })
  const row = (c: HTMLElement, name: string) => Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === name)!
  const viewHost = (c: HTMLElement) => c.querySelector('[data-testid="web-session-view"]')!.getAttribute('data-snapshot-host')
  const closeButton = (c: HTMLElement, name: string) => c.querySelector<HTMLButtonElement>(`button[aria-label^="Close ${name} "]`)!

  it('the view gets the MagicDNS name for a MagicDNS peer and the hostname for a hostname-only peer', async () => {
    const c = await mount()
    await click(row(c, 'Alpha'))
    expect(viewHost(c)).toBe('office-air-1.tail1234.ts.net')
    await click(row(c, 'Charlie'))
    expect(viewHost(c)).toBe('studio-mac')
  })

  it('closing a MagicDNS peer\'s tab drops its entry and keeps another host\'s entry with the same id', async () => {
    const c = await mount()
    await click(row(c, 'Alpha'))
    const saveKey = peerSnapshotKey(viewHost(c)!, 's1')
    putPeerSnapshot(saveKey, snap)
    putPeerSnapshot(peerSnapshotKey('other-mac', 's1'), snap)
    await click(closeButton(c, 'Alpha'))
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
    expect(takePeerSnapshot(saveKey)).toBeUndefined()
    expect(takePeerSnapshot(peerSnapshotKey('other-mac', 's1'))).toEqual(snap)
  })

  it('closing a hostname-only peer\'s inactive tab drops its entry', async () => {
    const c = await mount()
    await click(row(c, 'Charlie'))
    const saveKey = peerSnapshotKey(viewHost(c)!, 's2')
    await click(row(c, 'Alpha')) // Charlie's tab is now hidden, as when its snapshot is saved
    putPeerSnapshot(saveKey, snap)
    await click(closeButton(c, 'Charlie'))
    expect(takePeerSnapshot(saveKey)).toBeUndefined()
  })

  // End Session… is offered on local tabs only (TabBar isLocal); Cmd+W is a peer tab's other removal path.
  it('File > Close Tab (Cmd+W) on the active peer tab also drops its entry', async () => {
    const c = await mount()
    await click(row(c, 'Charlie'))
    const saveKey = peerSnapshotKey(viewHost(c)!, 's2')
    putPeerSnapshot(saveKey, snap)
    await act(async () => { wailsEvents.get('menu:close-tab')!() })
    await flush()
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
    expect(takePeerSnapshot(saveKey)).toBeUndefined()
  })
})

// INPT-06: a peer tab's Cmd+/Cmd− is that tab's own font size (kept per tab id),
// and it makes the tab's panel claim the session grid.
describe('App — peer tab font size per tab (INPT-06)', () => {
  const peerA: RemotePeerSessions = { hostname: 'Kens-MacBook-Air', dnsName: 'office-air-1.tail1234.ts.net', reachable: true, sessions: [{ id: 's1', name: 'Alpha', status: 'idle', url: '' }] }
  const peerC: RemotePeerSessions = { hostname: 'studio-mac', reachable: true, sessions: [{ id: 's2', name: 'Charlie', status: 'idle', url: '' }] }
  const connect = vi.fn((host: unknown, id: unknown) => ({ sessionId: id, name: '', host: '', url: `https://${host}:7444/sessions/${id}?cap=C`, errorCode: '' }))

  afterEach(async () => { await unmountApp(); vi.clearAllMocks() })

  const row = (c: HTMLElement, name: string) => Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === name)!
  const fontSize = (c: HTMLElement) => Number(c.querySelector('[data-testid="web-session-view"]')!.getAttribute('data-font-size'))
  const closeButton = (c: HTMLElement, name: string) => c.querySelector<HTMLButtonElement>(`button[aria-label^="Close ${name} "]`)!

  it('Cmd+ raises only that peer tab\'s font; a second peer tab keeps the default; a reopened tab starts from the default', async () => {
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [peerA, peerC] })
    await click(row(c, 'Alpha'))
    const base = fontSize(c)
    expect(base).toBeGreaterThan(0)
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-font-up"]')!)
    expect(fontSize(c)).toBe(base + 1)
    await click(row(c, 'Charlie'))
    expect(fontSize(c)).toBe(base)
    await click(row(c, 'Alpha'))
    expect(fontSize(c)).toBe(base + 1)
    await click(closeButton(c, 'Alpha'))
    await click(row(c, 'Alpha'))
    expect(fontSize(c)).toBe(base)
  })
})

// INPT-04: two Macs' daemons can hand out the same session id. Each gets its own
// tab (id and snapshot key carry the host), and every lookup matches host plus session.
describe('App — two peers with the same session id (INPT-04)', () => {
  const peerA: RemotePeerSessions = { hostname: 'A', dnsName: 'a.tail.ts.net', reachable: true, sessions: [{ id: 's1', name: 'Alpha', status: 'idle', url: '' }] }
  const peerB: RemotePeerSessions = { hostname: 'B', reachable: true, sessions: [{ id: 's1', name: 'Bravo', status: 'idle', url: '' }] }
  const ok = (host: unknown, id: unknown) => ({ sessionId: id, name: '', host: '', url: `https://${host}:7444/sessions/${id}?cap=C-${host}`, errorCode: '' })

  afterEach(async () => { await unmountApp(); vi.clearAllMocks() })

  const row = (c: HTMLElement, name: string) => Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === name)!
  const view = (c: HTMLElement) => c.querySelector('[data-testid="web-session-view"]')!
  const tabCount = (c: HTMLElement) => c.querySelectorAll('[role="tab"]').length

  it('same session id on two hosts opens two tabs with two snapshot hosts; re-clicking A only activates A', async () => {
    const connect = vi.fn(ok)
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [peerA, peerB] })
    await click(row(c, 'Alpha'))
    expect(view(c).getAttribute('data-snapshot-host')).toBe('a.tail.ts.net')
    await click(row(c, 'Bravo'))
    expect(tabCount(c)).toBe(2)
    expect(view(c).getAttribute('data-snapshot-host')).toBe('B')
    expect(view(c).getAttribute('data-cap')).toBe('C-B')
    expect(connect).toHaveBeenCalledTimes(2)
    await click(row(c, 'Alpha'))
    expect(connect).toHaveBeenCalledTimes(2)
    expect(tabCount(c)).toBe(2)
    expect(view(c).getAttribute('data-snapshot-host')).toBe('a.tail.ts.net')
    expect(view(c).getAttribute('data-cap')).toBe('C-a.tail.ts.net')
  })

  it('share stopped on B marks only B\'s tab: re-clicking A still only activates A', async () => {
    const connect = vi.fn(ok)
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [peerA, peerB] })
    await click(row(c, 'Alpha'))
    await click(row(c, 'Bravo'))
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-share-stopped"]')!)
    await click(row(c, 'Alpha'))
    expect(connect).toHaveBeenCalledTimes(2)
    expect(view(c).getAttribute('data-snapshot-host')).toBe('a.tail.ts.net')
  })

  // IN-04: two Macs can share a HostName; share stopped matches by the MagicDNS name.
  it('share stopped on one of two peers sharing a hostname keeps the other\'s row and live tab', async () => {
    const airA: RemotePeerSessions = { hostname: 'Air', dnsName: 'air-a.tail.ts.net', reachable: true, sessions: [{ id: 's1', name: 'Alpha', status: 'idle', url: '' }] }
    const airB: RemotePeerSessions = { hostname: 'Air', dnsName: 'air-b.tail.ts.net', reachable: true, sessions: [{ id: 's1', name: 'Bravo', status: 'idle', url: '' }] }
    const connect = vi.fn(ok)
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [airA, airB] })
    await click(row(c, 'Alpha'))
    await click(row(c, 'Bravo'))
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-share-stopped"]')!)
    expect(row(c, 'Bravo')).toBeUndefined()
    await click(row(c, 'Alpha'))
    expect(connect).toHaveBeenCalledTimes(2)
    expect(view(c).getAttribute('data-snapshot-host')).toBe('air-a.tail.ts.net')
  })

  it('a MagicDNS peer whose connect fails, then succeeds on a re-click, has exactly one tab', async () => {
    const connect = vi.fn()
      .mockReturnValueOnce({ sessionId: '', name: '', host: '', url: '', errorCode: 'unreachable' })
      .mockImplementationOnce(ok)
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [peerA] })
    await click(row(c, 'Alpha'))
    expect(c.querySelector('[data-testid="web-session-view"]')).toBeNull()
    await click(row(c, 'Alpha'))
    expect(connect).toHaveBeenCalledTimes(2)
    expect(tabCount(c)).toBe(1)
    expect(view(c).getAttribute('data-snapshot-host')).toBe('a.tail.ts.net')
  })
})

describe('App — the peer dot while its tab reconnects (UI polish)', () => {
  afterEach(async () => { await unmountApp(); vi.clearAllMocks() })

  it('the host dot turns amber ("Reconnecting") while the tab reconnects and green ("Reachable") once it is back', async () => {
    const c = await mountApp(baseImpl(vi.fn(() => okResult)))
    await click(peerRow(c)!)
    const dot = () => c.querySelector('.sidebar__host .sidebar__dot')!
    expect(dot().classList.contains('sidebar__dot--on')).toBe(true)

    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-reconnecting"]')!)
    expect(dot().classList.contains('sidebar__dot--warn')).toBe(true)
    expect(dot().classList.contains('sidebar__dot--on')).toBe(false)
    expect(dot().getAttribute('aria-label')).toBe('Reconnecting')

    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-reconnected"]')!)
    expect(dot().classList.contains('sidebar__dot--warn')).toBe(false)
    expect(dot().classList.contains('sidebar__dot--on')).toBe(true)
    expect(dot().getAttribute('aria-label')).toBe('Reachable')
  })
  it('the host row stays (amber) when discovery drops the blocked Mac while its tab reconnects', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    let peers = [remotePeer]
    const c = await mountApp({ ...baseImpl(vi.fn(() => okResult)), GetRemoteSessionsWithMeta: () => peers })
    await click(peerRow(c)!)
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-reconnecting"]')!)
    peers = []
    await act(async () => { vi.advanceTimersByTime(30_000) }) // next peer poll: the office no longer answers
    await flush()
    const dot = c.querySelector('.sidebar__host .sidebar__dot')!
    expect(dot.classList.contains('sidebar__dot--warn')).toBe(true)
    expect(dot.getAttribute('aria-label')).toBe('Reconnecting')
    expect(c.textContent).toContain("Can't reach this Mac.")
    vi.useRealTimers()
  })
})

describe('App — reconnect after the host shares again (WR-06)', () => {
  afterEach(async () => { await unmountApp(); vi.clearAllMocks(); vi.useRealTimers() })

  it('a click on the returned row reconnects the stopped tab with the new cap', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const connect = vi.fn()
      .mockReturnValueOnce(okResult)
      .mockReturnValueOnce({ ...okResult, url: okResult.url.replace('CAP123', 'CAP456') })
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-share-stopped"]')!)
    expect(peerRow(c)).toBeUndefined()
    await act(async () => { vi.advanceTimersByTime(30_000) }) // next peer poll: shared again
    await flush()
    await click(peerRow(c)!)
    expect(connect).toHaveBeenCalledTimes(2)
    expect(c.querySelector('[data-testid="web-session-view"]')!.getAttribute('data-cap')).toBe('CAP456')
  })
})

// Portty 05-05 (SHAR-07): before each redial a peer tab re-runs GetRelayPort and
// ConnectPeerSession (re-checks access, refills the daemon's cap store, picks up a
// new relay port after a daemon restart). The view is never remounted.
describe('App — peer tab re-authenticates before each redial (SHAR-07)', () => {
  beforeEach(() => { stub.mounts = 0; stub.verdict = undefined })
  afterEach(async () => { await unmountApp(); vi.clearAllMocks() })

  async function openThenReconnect(connect: (...a: unknown[]) => unknown, relayPort: () => unknown = () => 5555, peer: RemotePeerSessions = remotePeer) {
    const c = await mountApp({ ...baseImpl(connect), GetRelayPort: relayPort, GetRemoteSessionsWithMeta: () => [peer] })
    await click(peerRow(c)!)
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-before-reconnect"]')!)
    return c
  }

  it('success resolves ok with the current relay port and redials without remounting the view', async () => {
    const connect = vi.fn(() => okResult)
    const c = await openThenReconnect(connect)
    expect(stub.verdict).toEqual({ kind: 'ok', port: 5555 })
    expect(connect).toHaveBeenCalledTimes(2)
    expect(connect).toHaveBeenLastCalledWith('office-air.tail1234.ts.net', 'sess-remote-1')
    const view = c.querySelector('[data-testid="web-session-view"]')!
    expect(view.getAttribute('data-cap')).toBe('CAP123')
    expect(view.getAttribute('data-session')).toBe('sess-remote-1')
    expect(stub.mounts).toBe(1)
  })

  it('reconnects by the MagicDNS name when the peer has one', async () => {
    const connect = vi.fn(() => okResult)
    await openThenReconnect(connect, () => 5555, { ...remotePeer, hostname: 'Kens-MacBook-Air', dnsName: 'office-air-1.tail1234.ts.net' })
    expect(connect).toHaveBeenCalledTimes(2)
    expect(connect).toHaveBeenLastCalledWith('office-air-1.tail1234.ts.net', 'sess-remote-1')
  })

  it.each(['not_sharing', 'no_such_session', 'forbidden'])('errorCode %s stops with the stopped-sharing verdict', async (code) => {
    const connect = vi.fn().mockReturnValueOnce(okResult).mockReturnValueOnce({ ...okResult, url: '', errorCode: code })
    await openThenReconnect(connect)
    expect(stub.verdict).toEqual({ kind: 'stop', reason: 'share-stopped' })
  })

  it.each(['unreachable', 'tailscale_off'])('errorCode %s retries', async (code) => {
    const connect = vi.fn().mockReturnValueOnce(okResult).mockReturnValueOnce({ ...okResult, url: '', errorCode: code })
    await openThenReconnect(connect)
    expect(stub.verdict).toEqual({ kind: 'retry' })
  })

  it('a rejected ConnectPeerSession retries', async () => {
    const connect = vi.fn().mockReturnValueOnce(okResult).mockImplementationOnce(() => { throw new Error('bridge down') })
    await openThenReconnect(connect)
    expect(stub.verdict).toEqual({ kind: 'retry' })
  })

  it('a rejected GetRelayPort retries', async () => {
    let calls = 0
    const connect = vi.fn(() => okResult)
    await openThenReconnect(connect, () => { if (++calls > 1) throw new Error('daemon down'); return 5555 })
    expect(stub.verdict).toEqual({ kind: 'retry' })
  })

  it('a local web-share tab gets no beforeReconnect from App', async () => {
    window.history.pushState({}, '', '/app/?session=sess-local&cap=good')
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 200 })))
    try {
      const c = await mountApp({})
      expect(c.querySelector('[data-testid="web-session-view"]')!.getAttribute('data-has-reconnect')).toBe('false')
    } finally {
      vi.unstubAllGlobals()
      window.history.pushState({}, '', '/')
    }
  })
})

// Portty 05-09: a crashing pane must leave the sidebar and the tab bar alive.
describe('App — a crashing remote pane is contained (SHAR-07)', () => {
  afterEach(async () => { stub.throwFor = ''; await unmountApp(); vi.restoreAllMocks(); vi.clearAllMocks() })

  it('the pane shows the fixed fallback, the sidebar and tab bar stay, and activating another tab clears it', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    stub.throwFor = 'sess-remote-1'
    const second = { id: 'sess-remote-2', name: 'Second', status: 'idle', url: 'https://office-air.tail1234.ts.net:7444/sessions/sess-remote-2' }
    const connect = vi.fn((_h: unknown, id: unknown) => (id === 'sess-remote-1' ? okResult : { ...okResult, sessionId: 'sess-remote-2', name: 'Second', url: okResult.url.replace(/sess-remote-1/g, 'sess-remote-2') }))
    const c = await mountApp({ ...baseImpl(connect), GetRemoteSessionsWithMeta: () => [{ ...remotePeer, sessions: [...remotePeer.sessions, second] }] })
    await click(peerRow(c)!)
    expect(c.textContent).toContain('This tab hit an error. Close it and open the session again.')
    expect(c.textContent).not.toContain('CAP123')
    expect(c.querySelector('.sidebar')).not.toBeNull()
    expect(peerRow(c)).toBeDefined()
    expect(c.querySelector('[role="tablist"]')).not.toBeNull()
    const row2 = Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === 'Second')!
    await click(row2)
    expect(c.textContent).not.toContain('This tab hit an error.')
    expect(c.querySelector('[data-testid="web-session-view"]')!.getAttribute('data-session')).toBe('sess-remote-2')
  })
})

describe('App.tsx — no code-based access remains', () => {
  it('App has no dialog state, cached-cap set or removed remote-cap bindings', () => {
    expect(raw).toContain('const connectHost = session.peerDNSName || peerHost')
    expect(raw).toContain('ConnectPeerSession(connectHost, session.id)')
    expect(raw).not.toContain('remoteCapsCached')
    expect(raw).not.toContain('handleModalExchange')
  })
})

// ─── Sidebar peer row (Portty 02-04) ─────────────────────────────────────────
let container: HTMLElement | undefined
let root: Root | undefined

function renderPeerSidebar(onOpenPeer: Parameters<typeof Sidebar>[0]['onOpenPeer']) {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  flushSync(() => {
    root!.render(
      React.createElement(Sidebar, {
        sessions: [],
        activeTab: null,
        onOpenLocal: vi.fn(),
        peerView: peerView(false, true, [remotePeer]),
        onOpenPeer,
        onHide: vi.fn(),
        onRename: vi.fn(),
        onEndSession: vi.fn(),
        onShareToggle: vi.fn(),
        onNewSession: vi.fn(),
        tailscaleHealth: { connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'me.ts.net' },
      }),
    )
  })
  return { container: container! }
}

describe('Sidebar peer row — open entry point (NAV-02)', () => {
  afterEach(() => {
    if (root) { flushSync(() => root!.unmount()); root = undefined }
    container?.remove()
    container = undefined
  })

  it('clicking the row calls onOpenPeer with the peer and session; adapting it keeps the peer URL', () => {
    const onOpenPeer = vi.fn()
    const { container: c } = renderPeerSidebar(onOpenPeer)
    const row = peerRow(c)!
    expect(row.disabled).toBe(false)
    flushSync(() => { row.click() })
    expect(onOpenPeer).toHaveBeenCalledWith(remotePeer, remotePeer.sessions[0])
    const adapted = adaptRemoteSession(remotePeer, remotePeer.sessions[0])
    expect(adapted).toMatchObject({ id: 'sess-remote-1', hostname: remotePeer.hostname, url: remotePeer.sessions[0].url })
  })

  it('App wires onOpenPeer through handleOpenRemoteSession(adaptRemoteSession(peer, s))', () => {
    expect(raw).toContain('onOpenPeer={(peer, s) => void handleOpenRemoteSession(adaptRemoteSession(peer, s))}')
  })
})

// ─── FIX-03 behavior: per-tab isolation — two remote sessions open independent tabs ──
//
// openWebSessionTab is internal to the App component (not exported), so this suite
// follows the established source-inspection convention (see App.test.tsx) rather than
// mounting the full App tree. Together the two assertions below prove per-call
// isolation: (1) the tab id is deterministically keyed by the sessionId ARGUMENT and,
// for a peer tab, its host (webSessionTabId(sessionId, host), INPT-04), so two different
// remote sessionIds (or one id on two hosts) always produce two different tab ids; and (2) the created Tab's baseURL/capToken are sourced directly
// from THIS call's own parameters — never from the shared/global webParams — so two
// concurrent remote tabs can never cross-contaminate each other's cap/host
// (RESEARCH Pitfall 3 / T-168-04).
describe('App.tsx — openWebSessionTab per-tab isolation (FIX-03, two remote sessions)', () => {
  // INPT-04: plus this call's own peer host, so two hosts' equal session ids get two tab ids.
  it('tab id is keyed by the sessionId argument and this call\'s peer host, not a shared value (distinct sessions or hosts -> distinct tab ids)', () => {
    const idx = raw.indexOf('const openWebSessionTab')
    expect(idx, 'openWebSessionTab must be present in App.tsx').toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 900)
    expect(slice).toContain('webSessionTabId(sessionId, peer ? peerSnapshotHost(peer) : undefined)')
  })

  it('openWebSessionTab signature is (sessionId, baseURL?, capToken?) — extended for FIX-03', () => {
    const idx = raw.indexOf('const openWebSessionTab')
    const slice = raw.slice(idx, idx + 200)
    expect(slice).toContain('sessionId: string, baseURL?: string, capToken?: string')
  })

  it('the created Tab carries baseURL/capToken from THIS call, not the global webParams', () => {
    const idx = raw.indexOf('const openWebSessionTab')
    const slice = raw.slice(idx, idx + 900)
    expect(slice).toContain('baseURL,')
    expect(slice).toContain('capToken,')
    // Must not read from the mount-stable global webParams inside this function —
    // that would make a second remote tab silently reuse the first's cap/host.
    expect(slice).not.toContain('webParams')
  })

  it('the __websession__ render branch resolves per-tab params from the active tab, not the global webParams, for remote tabs (RESEARCH Pitfall 3)', () => {
    const idx = raw.indexOf("activeId.startsWith('__websession__')")
    expect(idx, 'the __websession__ render branch must be present').toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 3500)
    expect(slice).toContain('activeWebTab')
    expect(slice).toContain('isRemoteWebTab')
    expect(slice).toContain('baseURL={activeWebTab?.baseURL}')
  })

  // FIX-03 (plan 09): RC-B full-height wrapper + RC-A remote flag threading.
  // The __websession__ branch must wrap WebShareSessionView in .terminal-wrapper
  // (the full-height flex-column chain that resolves the modal's flex:1, fixing
  // the half-height terminal + dead band) AND pass remote={isRemoteWebTab} so the
  // remote tab uses the daemon proxy instead of a direct cross-origin peer wss.
  // Portty 05-09: on the desktop a missing web tab renders nothing. The old
  // fallback dialed window.location.origin (wails://), whose WebSocket threw and
  // took the root down. Unreachable through the UI after the dedupe fixes.
  it('the __websession__ branch renders nothing on the desktop when the active web tab is missing', () => {
    const idx = raw.indexOf("activeId.startsWith('__websession__')")
    const slice = raw.slice(idx, idx + 600)
    expect(slice).toContain("if (!activeWebTab && mode !== 'web') return null")
  })

  it('RC-B/RC-A: the __websession__ branch wraps WebShareSessionView in .terminal-wrapper and threads remote={isRemoteWebTab}', () => {
    const idx = raw.indexOf("activeId.startsWith('__websession__')")
    expect(idx, 'the __websession__ render branch must be present').toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 3500)
    expect(slice).toContain('terminal-wrapper')
    expect(slice).toContain('remote={isRemoteWebTab}')
  })

  // CR-03 (168-REVIEW): the WebShareSessionView render site had no `key`, so
  // switching between two remote-peer tabs did NOT unmount/remount the
  // component — its local useState (livePluginConfig) leaked from session A into session B's render. Fixed
  // by keying on the same wsSessionId used to resolve this render's own
  // sessionId prop, forcing a clean remount whenever the active
  // __websession__ tab changes to a different session.
  // INPT-04: keyed by the tab id (host plus session), so two hosts' equal session ids still remount.
  it('CR-03: <WebShareSessionView> is keyed by the tab id and cap so switching remote tabs forces a remount (no cross-session state leak)', () => {
    const idx = raw.indexOf("activeId.startsWith('__websession__')")
    expect(idx, 'the __websession__ render branch must be present').toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 3500)
    const wsvIdx = slice.indexOf('<WebShareSessionView')
    expect(wsvIdx, '<WebShareSessionView> element must be present in this branch').toBeGreaterThan(-1)
    // The key prop must appear on the element itself (before sessionId prop),
    // not merely somewhere later in the file.
    const elementSlice = slice.slice(wsvIdx, wsvIdx + 400)
    expect(elementSlice).toContain("key={`${activeId}:${wsCapToken}`}")
  })
})

