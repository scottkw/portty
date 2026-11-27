// Portty 21-08 (DVWR-01) — a peer's Desktop row opens one Desktop tab that streams through the loopback
// options. The real App is mounted with the Wails bridge faked (appHarness). The signalling socket and the
// peer connection are fakes, so nothing contacts a peer; TerminalPanel is a recorder, so the test can assert
// a Desktop tab is never mounted as a terminal (research Pitfall 3, T-21-29).

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { act } from 'react'
import type { RemotePeerSessions } from '../../lib/remoteSession'
import { mountApp, unmountApp, flush } from './appHarness'

const h = vi.hoisted(() => {
  class FakeTransport {
    sent: Record<string, unknown>[] = []
    closedWith: number | undefined | 'never' = 'never'
    private openCb: () => void = () => {}
    private msgCb: (m: unknown) => void = () => {}
    send(m: object) { this.sent.push(m as Record<string, unknown>) }
    close(code?: number) { this.closedWith = code }
    onOpen(cb: () => void) { this.openCb = cb }
    onMessage(cb: (m: unknown) => void) { this.msgCb = cb }
    onClose() {}
    open() { this.openCb() }
    recv(m: object) { this.msgCb(m) }
  }
  class FakePC extends EventTarget {
    closed = false
    signalingState = 'stable'
    iceConnectionState = 'new'
    connectionState = 'new'
    iceGatheringState = 'complete'
    localDescription: { sdp: string } | null = null
    ontrack: ((e: unknown) => void) | null = null
    onconnectionstatechange: (() => void) | null = null
    onicegatheringstatechange: (() => void) | null = null
    addTransceiver() {}
    createDataChannel() { return { readyState: 'connecting', bufferedAmount: 0, send() {}, close() {} } }
    async createOffer() { return { type: 'offer', sdp: 'fake-offer-sdp' } }
    async setLocalDescription(d: { sdp: string }) { this.localDescription = d }
    async setRemoteDescription() {}
    close() { this.closed = true }
    async getStats() { return new Map() }
    fireStreaming() {
      this.ontrack?.({ streams: [{ id: 'stream' }], track: {} })
      this.connectionState = 'connected'
      this.onconnectionstatechange?.()
    }
  }
  return { FakeTransport, FakePC, transports: [] as InstanceType<typeof FakeTransport>[], pcs: [] as InstanceType<typeof FakePC>[], urls: [] as string[], panelIds: [] as string[] }
})

vi.mock('../TerminalPanel', () => ({
  TerminalPanel: (p: { sessionId: string }) => { h.panelIds.push(p.sessionId); return null },
}))

vi.mock('../../lib/desktopClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/desktopClient')>()),
  webSocketTransport: vi.fn((url: string) => {
    const t = new h.FakeTransport()
    h.transports.push(t)
    h.urls.push(url)
    return t
  }),
}))

const LOOPBACK = 'ws://127.0.0.1:7001/api/relay/remote-desktop/office-air.tail1234.ts.net/ws'
const CONNECT_HOST = 'office-air.tail1234.ts.net'
const TAB_ID = `desktop:${CONNECT_HOST}`

const peer: RemotePeerSessions = {
  hostname: 'office-air',
  dnsName: CONNECT_HOST,
  reachable: true,
  desktop: true,
  sessions: [],
}

let peers: RemotePeerSessions[] = [peer]
const connect = vi.fn(() => ({ host: 'office-air', url: LOOPBACK, errorCode: '' }))

function impl() {
  return {
    GetDaemonError: () => '',
    GetRelayPort: () => 7001,
    ListSessions: () => [{ id: 'local-1', name: 'shell', state: 'running', status: 'idle' }],
    GetTailscaleStatus: () => ({ connected: true, hasCerts: true, ip: '100.64.0.1', domain: 'home-air.tail1234.ts.net' }),
    GetRemoteSessionsWithMeta: () => peers,
    ConnectPeerDesktop: connect,
  }
}

const desktopRow = (c: HTMLElement) =>
  Array.from(c.querySelectorAll<HTMLButtonElement>('button.sidebar__row')).find((b) => b.textContent === 'Desktop')
const desktopTabs = (c: HTMLElement) =>
  Array.from(c.querySelectorAll<HTMLElement>('[role="tab"]')).filter((t) => t.textContent?.includes('Desktop'))
const panel = (c: HTMLElement) => c.querySelector<HTMLElement>(`[id="tabpanel-${TAB_ID}"]`)
const localTab = (c: HTMLElement) => Array.from(c.querySelectorAll<HTMLElement>('[role="tab"]')).find((t) => t.textContent?.includes('shell'))!

async function click(el: HTMLElement) {
  await act(async () => { el.click() })
  await flush()
}

beforeEach(() => {
  h.transports.length = 0
  h.pcs.length = 0
  h.urls.length = 0
  h.panelIds.length = 0
  connect.mockClear()
  peers = [peer]
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.stubGlobal('RTCPeerConnection', function () {
    const pc = new h.FakePC()
    h.pcs.push(pc)
    return pc
  })
})

afterEach(async () => {
  await unmountApp()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('App: the Desktop row opens a Desktop tab (DVWR-01)', () => {
  it('opens one active Desktop tab at once, mounts the viewer, and never mounts a terminal for it', async () => {
    const c = await mountApp(impl())
    expect(desktopRow(c), 'the peer announced a desktop').toBeDefined()
    await click(desktopRow(c)!)
    expect(desktopTabs(c)).toHaveLength(1)
    expect(desktopTabs(c)[0].getAttribute('aria-selected')).toBe('true')
    expect(panel(c)?.querySelector('video[aria-label="Mac desktop"]')).not.toBeNull()
    // The Connecting card, in the app's words, while ConnectPeerDesktop runs inside the tab.
    expect(panel(c)?.querySelector('.desktop-card h1')?.textContent).toBe('Connecting…')
    expect(connect).toHaveBeenCalledWith(CONNECT_HOST)
    expect(h.panelIds).toContain('local-1')
    expect(h.panelIds.some((id) => id.startsWith('desktop:'))).toBe(false)
  })

  it('a second click focuses the open tab: one tab, one stream', async () => {
    const c = await mountApp(impl())
    await click(desktopRow(c)!)
    await click(localTab(c))
    expect(desktopTabs(c)[0].getAttribute('aria-selected')).toBe('false')
    await click(desktopRow(c)!)
    expect(desktopTabs(c)).toHaveLength(1)
    expect(desktopTabs(c)[0].getAttribute('aria-selected')).toBe('true')
    expect(c.querySelectorAll('video[aria-label="Mac desktop"]')).toHaveLength(1)
    expect(connect).toHaveBeenCalledTimes(1)
    expect(h.transports).toHaveLength(1)
  })

  it('streams through the loopback url: hello, answer and a track reach Streaming, with no token anywhere', async () => {
    const c = await mountApp(impl())
    await click(desktopRow(c)!)
    expect(h.urls).toEqual([LOOPBACK])
    const t = h.transports[0]
    await act(async () => { t.open() })
    expect(t.sent[0]).toEqual({ type: 'hello', v: 1, caps: { video: 1, audio: 1, input: 1 } })
    await act(async () => { t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } }) })
    await flush()
    await act(async () => { t.recv({ type: 'answer', sdp: 'fake-answer-sdp' }) })
    await flush()
    await act(async () => { h.pcs[0].fireStreaming() })
    await act(async () => { panel(c)!.querySelector('video')!.dispatchEvent(new Event('playing')) })
    const region = panel(c)!.querySelector('.desktop-status__text')!
    expect(region.textContent).toContain('Streaming')
    expect(panel(c)?.querySelector('.desktop-card')).toBeNull()
    expect(c.innerHTML).not.toMatch(/[?&]cap=|token/i)
  })

  it('a tab behind another tab stays mounted, decoding and inert; coming back clears that', async () => {
    const c = await mountApp(impl())
    await click(desktopRow(c)!)
    const video = panel(c)!.querySelector('video')
    await click(localTab(c))
    const hidden = panel(c)!
    expect(hidden.className).toContain('desktop-tab--hidden')
    expect(hidden.hasAttribute('inert')).toBe(true)
    expect(hidden.getAttribute('aria-hidden')).toBe('true')
    expect(hidden.style.display).not.toBe('none')
    expect(hidden.querySelector('video')).toBe(video)
    await click(desktopTabs(c)[0])
    const shown = panel(c)!
    expect(shown.className).not.toContain('desktop-tab--hidden')
    expect(shown.hasAttribute('inert')).toBe(false)
    expect(shown.hasAttribute('aria-hidden')).toBe(false)
    expect(shown.querySelector('video')).toBe(video)
  })
})

describe('App: the tab feeds the sidebar row and closes cleanly (Task 2)', () => {
  const muted = (c: HTMLElement) => c.querySelector('.sidebar__muted[id^="sidebar-desktop-line"]')?.textContent ?? null
  const poll = async () => {
    await act(async () => { window.dispatchEvent(new CustomEvent('portty:wake', { detail: 'system:wake' })) })
    await flush()
  }

  async function connected(c: HTMLElement) {
    await click(desktopRow(c)!)
    const t = h.transports[0]
    await act(async () => { t.open() })
    await act(async () => { t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } }) })
    await flush()
    return t
  }
  const say = (t: InstanceType<typeof h.FakeTransport>, state: string, locked = false) =>
    act(async () => { t.recv({ type: 'state', state, viewers: 0, locked }) })

  it('shows the muted line for a non-ready state, clears it on streaming, and forgets it when the peer stops announcing the desktop', async () => {
    const c = await mountApp(impl())
    const t = await connected(c)
    await say(t, 'display_asleep')
    expect(muted(c)).toBe('Display asleep')
    expect(desktopRow(c)?.getAttribute('data-muted')).toBe('true')
    await say(t, 'streaming')
    expect(muted(c)).toBeNull()
    await say(t, 'needs_permission')
    expect(muted(c)).toBe('Needs permission')
    // The peer stops announcing the desktop: the row goes, and the old line must not come back with it.
    peers = [{ ...peer, desktop: false }]
    await poll()
    expect(desktopRow(c)).toBeUndefined()
    peers = [peer]
    await poll()
    expect(desktopRow(c)).toBeDefined()
    expect(muted(c)).toBeNull()
  })

  it('closing the tab takes its muted line with it while the peer still announces the desktop', async () => {
    const c = await mountApp(impl())
    const t = await connected(c)
    await say(t, 'locked', true)
    expect(muted(c)).toBe('Locked')
    await click(c.querySelector<HTMLButtonElement>('button.tab__close[aria-label^="Close Desktop"]')!)
    expect(desktopTabs(c)).toHaveLength(0)
    expect(desktopRow(c)).toBeDefined()
    expect(muted(c)).toBeNull()
  })

  it('a failed probe (peer not reachable) does not drop a still-true Locked line', async () => {
    const c = await mountApp(impl())
    const t = await connected(c)
    await say(t, 'locked', true)
    peers = [{ ...peer, reachable: false, desktop: false }]
    await poll()
    peers = [peer]
    await poll()
    expect(muted(c)).toBe('Locked')
  })

  it('closing the tab unmounts the viewer, disconnects, and activates the neighbour', async () => {
    const c = await mountApp(impl())
    await click(desktopRow(c)!)
    const t = h.transports[0]
    const close = c.querySelector<HTMLButtonElement>('button.tab__close[aria-label^="Close Desktop"]')!
    await click(close)
    expect(desktopTabs(c)).toHaveLength(0)
    expect(c.querySelector('video[aria-label="Mac desktop"]')).toBeNull()
    expect(t.closedWith).not.toBe('never')
    expect(localTab(c).getAttribute('aria-selected')).toBe('true')
  })
})
