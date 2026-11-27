// Phase 21-05 — the Desktop tab's connection logic, driven against the real
// Phase 19 createDesktopClient with a fake clock, document, window, transport
// and peer connection: no real socket, timer or network. The fakes are copied
// from desktopClient.test.ts on purpose (a test file is not imported).

import { describe, it, expect, vi } from 'vitest'
import {
  createDesktopClient,
  type ClientStatus,
  type DesktopClientOptions,
  type SignalTransport,
} from '../desktopClient'
import {
  connectVerdict,
  createPeerDesktopOptions,
  type PeerDesktopConnectResult,
} from '../peerDesktop'

class FakeClock {
  t = 0
  private seq = 0
  private timers: { id: number; at: number; fn: () => void }[] = []
  now = () => this.t
  setTimeout = (fn: () => void, ms: number) => {
    const id = ++this.seq
    this.timers.push({ id, at: this.t + ms, fn })
    return id
  }
  clearTimeout = (id: unknown) => {
    this.timers = this.timers.filter((x) => x.id !== id)
  }
  /** Advance the clock, firing due timers in order and letting promises settle after each. */
  async advance(ms: number) {
    const target = this.t + ms
    for (;;) {
      await settle()
      const due = this.timers.filter((x) => x.at <= target).sort((a, b) => a.at - b.at || a.id - b.id)[0]
      if (!due) break
      this.timers = this.timers.filter((x) => x !== due)
      this.t = due.at
      due.fn()
    }
    this.t = target
    await settle()
  }
}

async function settle() {
  for (let i = 0; i < 20; i++) await Promise.resolve()
}

class FakeTransport implements SignalTransport {
  sent: Record<string, unknown>[] = []
  closedWith: number | undefined | 'never' = 'never'
  private openCb: () => void = () => {}
  private msgCb: (m: unknown) => void = () => {}
  private closeCb: (c: number) => void = () => {}
  send(m: object) { this.sent.push(m as Record<string, unknown>) }
  close(code?: number) { this.closedWith = code }
  onOpen(cb: () => void) { this.openCb = cb }
  onMessage(cb: (m: unknown) => void) { this.msgCb = cb }
  onClose(cb: (c: number) => void) { this.closeCb = cb }
  open() { this.openCb() }
  recv(m: object) { this.msgCb(m) }
  dropWith(code: number) { this.closeCb(code) }
}

/** A peer connection that also speaks addEventListener, signalingState and iceConnectionState. */
class FakePC extends EventTarget {
  closed = false
  connectionState = 'new'
  iceConnectionState = 'new'
  signalingState = 'stable'
  iceGatheringState = 'complete'
  localDescription: { sdp: string } | null = null
  ontrack: ((e: unknown) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  addTransceiver() {}
  async createOffer() { return { type: 'offer', sdp: 'fake-offer-sdp' } }
  async setLocalDescription(d: { sdp: string }) {
    this.localDescription = d
    this.setSignaling('have-local-offer')
  }
  async setRemoteDescription() { this.setSignaling('stable') }
  close() {
    this.closed = true
    this.signalingState = 'closed'
  }
  fireTrack() { this.ontrack?.({ streams: [{ id: 'stream' }], track: {} }) }
  setSignaling(s: string) {
    this.signalingState = s
    this.dispatchEvent(new Event('signalingstatechange'))
  }
  setIce(s: string) {
    this.iceConnectionState = s
    this.dispatchEvent(new Event('iceconnectionstatechange'))
  }
}

const LOOPBACK = 'ws://127.0.0.1:41000/api/relay/remote-desktop/office-air.example/ws'
const ok = (url = LOOPBACK): PeerDesktopConnectResult => ({ host: 'office-air.example', url, errorCode: '' })

function setup(connect: (host: string) => Promise<PeerDesktopConnectResult>) {
  const clock = new FakeClock()
  const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  const win = Object.assign(new EventTarget(), { location: { hash: '', host: 'h', search: '' } }) as unknown as Window
  const urls: string[] = []
  const transports: FakeTransport[] = []
  const pcs: FakePC[] = []
  const statuses: ClientStatus[] = []
  const blocked: boolean[] = []
  const peer = createPeerDesktopOptions({
    host: 'office-air.example',
    connect,
    onLocalNetwork: (b) => blocked.push(b),
    clock,
    makeTransport: (url) => {
      urls.push(url)
      const t = new FakeTransport()
      transports.push(t)
      return t
    },
    makePC: () => {
      const p = new FakePC()
      pcs.push(p)
      return p as unknown as RTCPeerConnection
    },
  })
  const client = createDesktopClient({ ...peer.options, doc, win } as DesktopClientOptions)
  client.on('status', (s) => statuses.push(s))
  return { clock, doc, urls, transports, pcs, statuses, blocked, peer, client }
}
type Env = ReturnType<typeof setup>

const lastT = (e: Env) => e.transports[e.transports.length - 1]
const lastPC = (e: Env) => e.pcs[e.pcs.length - 1]

/** Walk the newest transport through hello, offer and answer. */
async function handshake(e: Env) {
  const t = lastT(e)
  t.open()
  t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
  await settle()
  t.recv({ type: 'answer', sdp: 'fake-answer-sdp' })
  await settle()
}

describe('tracer: connect, then stream over the loopback url', () => {
  it('takes the real client from connect to streaming with one transport and no token in any url', async () => {
    const e = setup(async () => ok())
    e.client.start()
    await settle()
    expect(e.urls).toEqual([LOOPBACK])
    await handshake(e)
    lastPC(e).fireTrack()
    expect(e.statuses.map((s) => s.kind)).toEqual(['streaming'])
    expect(e.urls).toHaveLength(1)
    for (const u of e.urls) expect(u).not.toMatch(/cap=|token=/)
    e.client.stop()
    e.peer.dispose()
  })
})

const fail = (errorCode: string): PeerDesktopConnectResult => ({ host: 'office-air.example', url: '', errorCode })

describe('connectVerdict', () => {
  it.each([
    ['', 'ok'],
    ['forbidden', 'other_account'],
    ['desktop_off', 'off'],
    ['unreachable', 'retry'],
    ['unknown_host', 'retry'],
    ['tailscale_off', 'retry'],
    ['anything', 'retry'],
  ])('maps %j to %s', (code, verdict) => {
    expect(connectVerdict(code)).toBe(verdict)
  })

  it('treats a rejected connect inside check as retry', async () => {
    const peer = createPeerDesktopOptions({
      host: 'office-air.example',
      connect: vi.fn<(h: string) => Promise<PeerDesktopConnectResult>>()
        .mockResolvedValueOnce(ok())
        .mockRejectedValueOnce(new Error('binding gone')),
      onLocalNetwork: () => {},
    })
    expect(await peer.options.check?.()).toBe('retry')
    peer.dispose()
  })
})

describe('connect and redial', () => {
  it('creates no transport before the initial connect resolves', async () => {
    let resolve: (r: PeerDesktopConnectResult) => void = () => {}
    const e = setup(() => new Promise((r) => { resolve = r }))
    e.client.start()
    await settle()
    expect(e.urls).toEqual([])
    resolve(ok())
    await settle()
    expect(e.urls).toEqual([LOOPBACK])
    e.client.stop()
    e.peer.dispose()
  })

  it('re-runs connect before a redial and opens the next transport on the new url', async () => {
    const second = 'ws://127.0.0.1:41999/api/relay/remote-desktop/office-air.example/ws'
    const connect = vi.fn<(h: string) => Promise<PeerDesktopConnectResult>>()
      .mockResolvedValueOnce(ok())
      .mockResolvedValueOnce(ok(second))
    const e = setup(connect)
    e.client.start()
    await settle()
    await handshake(e)
    lastPC(e).fireTrack()
    expect(connect).toHaveBeenCalledTimes(1)
    lastT(e).dropWith(1006)
    await e.clock.advance(1000)
    expect(connect).toHaveBeenCalledTimes(2)
    expect(connect).toHaveBeenLastCalledWith('office-air.example')
    expect(e.urls).toEqual([LOOPBACK, second])
    e.client.stop()
    e.peer.dispose()
  })

  it('retries after an initial unreachable with no url, using the connect that runs before the redial', async () => {
    const connect = vi.fn<(h: string) => Promise<PeerDesktopConnectResult>>()
      .mockResolvedValueOnce(fail('unreachable')) // the initial connect
      .mockResolvedValueOnce(fail('unreachable')) // the client's check after the empty-url close
      .mockResolvedValueOnce(ok()) // beforeRedial
    const e = setup(connect)
    e.client.start()
    await settle()
    expect(e.urls).toEqual([])
    expect(e.statuses.map((s) => s.kind)).toEqual(['reconnecting'])
    await e.clock.advance(1000)
    expect(connect).toHaveBeenCalledTimes(3)
    expect(e.urls).toEqual([LOOPBACK])
    e.client.stop()
    e.peer.dispose()
  })
})

describe('Local Network watcher', () => {
  /** A client whose answer has been applied (signalingState stable). */
  async function answered() {
    const e = setup(async () => ok())
    e.client.start()
    await settle()
    await handshake(e)
    return e
  }

  it('reports nothing at 9900 ms, then blocked once at 10000 ms and closes the transport with 4000', async () => {
    const e = await answered()
    await e.clock.advance(9900)
    expect(e.blocked).toEqual([])
    expect(lastT(e).closedWith).toBe('never')
    await e.clock.advance(100)
    expect(e.blocked).toEqual([true])
    expect(lastT(e).closedWith).toBe(4000)
    e.client.stop()
    e.peer.dispose()
  })

  it('rebuilds on the close it forced: the client redials with a fresh connect', async () => {
    const connect = vi.fn(async () => ok())
    const e = setup(connect)
    e.client.start()
    await settle()
    await handshake(e)
    await e.clock.advance(10000)
    lastT(e).dropWith(4000)
    await e.clock.advance(1000)
    expect(connect).toHaveBeenCalledTimes(2)
    expect(e.urls).toHaveLength(2)
    e.client.stop()
    e.peer.dispose()
  })

  it('reports clear when ICE reaches checking, and never blocked afterwards', async () => {
    const e = await answered()
    lastPC(e).setIce('checking')
    expect(e.blocked).toEqual([false])
    await e.clock.advance(20000)
    expect(e.blocked).toEqual([false])
    e.client.stop()
    e.peer.dispose()
  })

  it('reports blocked at once when ICE goes from new straight to failed', async () => {
    const e = await answered()
    lastPC(e).setIce('failed')
    expect(e.blocked).toEqual([true])
    e.client.stop()
    e.peer.dispose()
  })

  it('does not report blocked when ICE goes checking then failed', async () => {
    const e = await answered()
    lastPC(e).setIce('checking')
    lastPC(e).setIce('failed')
    expect(e.blocked).toEqual([false])
    e.client.stop()
    e.peer.dispose()
  })

  it('does not arm before the answer is applied', async () => {
    const e = setup(async () => ok())
    e.client.start()
    await settle()
    lastT(e).open()
    lastT(e).recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
    await settle()
    await e.clock.advance(15000)
    expect(e.blocked).toEqual([])
    e.client.stop()
    e.peer.dispose()
  })
})
