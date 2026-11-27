// Phase 22-07 — the desktop client in guest mode: the share server's own socket and check, no offer
// until {type: admission, state: admitted}, live rights and lease, and the fixed close reasons.
// Driven through opts (fake clock, transport, peer connection): no real socket, timer or network.

import { describe, it, expect, vi } from 'vitest'
import {
  RECONNECT,
  createDesktopClient,
  defaultGuestMode,
  defaultGuestWsUrl,
  type CheckVerdict,
  type ClientStatus,
  type DesktopClientOptions,
  type SignalTransport,
} from '../desktopClient'

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
  private openCb: () => void = () => {}
  private msgCb: (m: unknown) => void = () => {}
  private closeCb: (c: number) => void = () => {}
  send(m: object) { this.sent.push(m as Record<string, unknown>) }
  close() {}
  onOpen(cb: () => void) { this.openCb = cb }
  onMessage(cb: (m: unknown) => void) { this.msgCb = cb }
  onClose(cb: (c: number) => void) { this.closeCb = cb }
  open() { this.openCb() }
  recv(m: object) { this.msgCb(m) }
  dropWith(code: number) { this.closeCb(code) }
  types() { return this.sent.map((m) => m.type) }
}

class FakeChannel {
  readyState = 'connecting'
  bufferedAmount = 0
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  constructor(readonly label: string) {}
  send() {}
}

class FakePC {
  calls: string[] = []
  channels: FakeChannel[] = []
  connectionState = 'new'
  iceGatheringState = 'complete'
  localDescription: { sdp: string } | null = null
  ontrack: ((e: unknown) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  addTransceiver() { this.calls.push('addTransceiver') }
  createDataChannel(label: string) {
    this.calls.push('createDataChannel')
    const ch = new FakeChannel(label)
    this.channels.push(ch)
    return ch
  }
  async createOffer() {
    this.calls.push('createOffer')
    return { type: 'offer', sdp: 'fake-offer-sdp' }
  }
  async setLocalDescription(d: { sdp: string }) { this.localDescription = d }
  async setRemoteDescription() {}
  close() {}
  async getStats() { return new Map() }
  setConn(s: string) { this.connectionState = s; this.onconnectionstatechange?.() }
}

function setup(over: Partial<DesktopClientOptions> = {}) {
  const clock = new FakeClock()
  const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  const win = Object.assign(new EventTarget(), {
    location: { hash: '', host: 'mac.tail.ts.net', search: '?cap=tok', pathname: '/desktop/guest/' },
  }) as unknown as Window
  const transports: FakeTransport[] = []
  const pcs: FakePC[] = []
  const statuses: ClientStatus[] = []
  const events: [string, unknown][] = []
  const check = vi.fn<() => Promise<CheckVerdict>>(async () => 'retry')
  const client = createDesktopClient({
    transport: () => { const t = new FakeTransport(); transports.push(t); return t },
    createPeerConnection: () => { const p = new FakePC(); pcs.push(p); return p as unknown as RTCPeerConnection },
    check,
    clock,
    doc,
    win,
    guest: true,
    ...over,
  })
  client.on('status', (s) => statuses.push(s))
  for (const ev of ['admission', 'rights', 'lease'] as const) client.on(ev, (v) => events.push([ev, v]))
  return { client, clock, transports, pcs, statuses, events, check }
}
type Env = ReturnType<typeof setup>
const lastT = (e: Env) => e.transports[e.transports.length - 1]
const lastPC = (e: Env) => e.pcs[e.pcs.length - 1]
const noRights = { control: false, clipboard: false, surface: false }
const helloMsg = { type: 'hello', v: 1, viewer: '0123456789abcdef', caps: { input: 1, video: 1 }, rights: noRights }

/** Open the socket and take the guest hello. */
async function hello(e: Env) {
  e.client.start()
  lastT(e).open()
  lastT(e).recv(helloMsg)
  await settle()
}

describe('guest URLs and mode', () => {
  it('is guest mode on a /desktop/guest/ path only, and builds the share server socket URL from the page URL', () => {
    const at = (pathname: string, search = '?cap=a+b%2Fc') => ({ host: 'mac.example', search, pathname }) as unknown as Location
    expect(defaultGuestMode(at('/desktop/guest/'))).toBe(true)
    expect(defaultGuestMode(at('/desktop/guest/index.html'))).toBe(true)
    expect(defaultGuestMode(at('/desktop/'))).toBe(false)
    expect(defaultGuestMode(at('/desktop/guests'))).toBe(false)
    expect(defaultGuestWsUrl(at('/desktop/guest/'))).toBe('wss://mac.example/desktop/guest/ws?cap=a%20b%2Fc')
  })

  it('opens the guest socket by default on a guest page and the own-account socket elsewhere', () => {
    const urls: string[] = []
    class FakeWS {
      constructor(u: string) { urls.push(u) }
      addEventListener() {}
      send() {}
      close() {}
    }
    vi.stubGlobal('WebSocket', FakeWS)
    try {
      for (const [pathname, guest] of [['/desktop/guest/', undefined], ['/desktop/', undefined], ['/desktop/', true]] as const) {
        const win = Object.assign(new EventTarget(), { location: { hash: '', host: 'h', search: '?cap=t', pathname } }) as unknown as Window
        const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
        const c = createDesktopClient({ win, doc, guest, clock: new FakeClock() })
        c.start()
        c.stop()
      }
    } finally {
      vi.unstubAllGlobals()
    }
    expect(urls).toEqual(['wss://h/desktop/guest/ws?cap=t', 'wss://h/desktop/ws?cap=t', 'wss://h/desktop/guest/ws?cap=t'])
  })
})

describe('guest waits then offers (tracer)', () => {
  it('sends hello, takes the hello with all rights false and admission waiting, sends no offer, then exactly one offer on admitted', async () => {
    const e = setup()
    await hello(e)
    expect(lastT(e).sent[0]).toEqual({ type: 'hello', v: 1, caps: { video: 1, input: 1 } })
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    await settle()
    expect(lastT(e).types()).toEqual(['hello'])
    expect(e.pcs).toHaveLength(0)
    expect(e.events).toEqual([
      ['rights', noRights],
      ['admission', { state: 'waiting' }],
    ])
    lastT(e).recv({ type: 'admission', state: 'admitted' })
    await settle()
    expect(lastT(e).types()).toEqual(['hello', 'offer'])
    expect(lastT(e).sent[1]).toEqual({ type: 'offer', sdp: 'fake-offer-sdp' })
    // The input channel is opened before createOffer, as for an own-account viewer.
    expect(lastPC(e).calls).toEqual(['createDataChannel', 'addTransceiver', 'createOffer'])
    lastT(e).recv({ type: 'admission', state: 'admitted' })
    await settle()
    expect(lastT(e).types()).toEqual(['hello', 'offer'])
    expect(e.events[e.events.length - 1]).toEqual(['admission', { state: 'admitted' }])
  })

  it('a re-admission (hello, then admitted with no waiting) offers at once, and rights follow', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'admitted' })
    await settle()
    expect(lastT(e).types()).toEqual(['hello', 'offer'])
    lastT(e).recv({ type: 'rights', rights: { control: true, clipboard: false, surface: false } })
    expect(e.events[e.events.length - 1]).toEqual(['rights', { control: true, clipboard: false, surface: false }])
  })

  it('never reports unreachable while waiting at Admit, however long the wait', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    await e.clock.advance(RECONNECT.unreachableAfterMs * 3)
    expect(e.statuses.map((s) => s.kind)).not.toContain('unreachable')
  })

  it('reports unreachable about 30 s after admission when no picture ever comes', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    await e.clock.advance(60000)
    lastT(e).recv({ type: 'admission', state: 'admitted' })
    await settle()
    await e.clock.advance(RECONNECT.unreachableAfterMs - 1)
    expect(e.statuses.map((s) => s.kind)).not.toContain('unreachable')
    await e.clock.advance(2)
    expect(e.statuses.map((s) => s.kind)).toContain('unreachable')
  })
})
