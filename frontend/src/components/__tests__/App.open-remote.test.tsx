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

import { describe, it, expect, vi, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

import raw from '../../App.tsx?raw'
import { Sidebar } from '../Sidebar'
import { peerView } from '../../lib/sidebarPeers'
import { adaptRemoteSession } from '../../lib/remoteAdapter'
import type { RemotePeerSessions } from '../../lib/remoteSession'
import { mountApp, unmountApp, flush } from './appHarness'

vi.mock('../WebShareSessionView', () => ({
  WebShareSessionView: (p: { sessionId: string; baseURL?: string; capToken: string; hostLabel?: string; remote?: boolean; onShareStopped?: () => void }) =>
    React.createElement('div', {
      'data-testid': 'web-session-view',
      'data-session': p.sessionId,
      'data-base': p.baseURL ?? '',
      'data-cap': p.capToken,
      'data-host': p.hostLabel ?? '',
      'data-remote': String(p.remote),
    }, React.createElement('button', { type: 'button', 'data-testid': 'fire-share-stopped', onClick: () => p.onShareStopped?.() })),
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
    expect(c.textContent).toContain('office-air › Remote Session')
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
    expect(c.textContent).toContain('office-air › Remote Session')
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

  it('when the host stops sharing, the row disappears at once and the tab stays', async () => {
    const connect = vi.fn(() => okResult)
    const c = await mountApp(baseImpl(connect))
    await click(peerRow(c)!)
    await click(c.querySelector<HTMLButtonElement>('[data-testid="fire-share-stopped"]')!)
    expect(peerRow(c)).toBeUndefined()
    expect(c.textContent).toContain('office-air › Remote Session')
    expect(c.querySelector('[data-testid="web-session-view"]')).not.toBeNull()
  })
})

describe('App.tsx — no code-based access remains', () => {
  it('App has no dialog state, cached-cap set or removed remote-cap bindings', () => {
    expect(raw).toContain('ConnectPeerSession(session.peerDNSName || peerHost, session.id)')
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
// isolation: (1) the tab id is deterministically keyed by the sessionId ARGUMENT
// (webSessionTabId(sessionId)), so two different remote sessionIds always produce two
// different tab ids; and (2) the created Tab's baseURL/capToken are sourced directly
// from THIS call's own parameters — never from the shared/global webParams — so two
// concurrent remote tabs can never cross-contaminate each other's cap/host
// (RESEARCH Pitfall 3 / T-168-04).
describe('App.tsx — openWebSessionTab per-tab isolation (FIX-03, two remote sessions)', () => {
  it('tab id is keyed by the sessionId argument, not a shared value (distinct sessions -> distinct tab ids)', () => {
    const idx = raw.indexOf('const openWebSessionTab')
    expect(idx, 'openWebSessionTab must be present in App.tsx').toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 900)
    expect(slice).toContain('webSessionTabId(sessionId)')
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
  it('CR-03: <WebShareSessionView> is keyed by wsSessionId so switching remote tabs forces a remount (no cross-session state leak)', () => {
    const idx = raw.indexOf("activeId.startsWith('__websession__')")
    expect(idx, 'the __websession__ render branch must be present').toBeGreaterThan(-1)
    const slice = raw.slice(idx, idx + 3500)
    const wsvIdx = slice.indexOf('<WebShareSessionView')
    expect(wsvIdx, '<WebShareSessionView> element must be present in this branch').toBeGreaterThan(-1)
    // The key prop must appear on the element itself (before sessionId prop),
    // not merely somewhere later in the file.
    const elementSlice = slice.slice(wsvIdx, wsvIdx + 400)
    expect(elementSlice).toContain('key={wsSessionId}')
  })
})

