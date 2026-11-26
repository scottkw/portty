// Phase 19-06 Task 2 — the /desktop/ client's seams: injectable transport,
// pre-redial hook, wake events, capped backoff, liveness ping and fixed-code
// error branching. Everything is driven through opts (fake clock, document,
// window, transport and peer connection): no real socket, timer or network.

import { describe, it, expect, afterEach, vi } from 'vitest'
import {
  RECONNECT,
  createDesktopClient,
  defaultDesktopWsUrl,
  fetchCheck,
  type CheckVerdict,
  type ClientStatus,
  type DesktopClientOptions,
  type SignalTransport,
  type WakeReason,
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
  types() { return this.sent.map((m) => m.type) }
  last(type: string) { return [...this.sent].reverse().find((m) => m.type === type) }
}

class FakePC {
  closed = false
  connectionState = 'new'
  iceGatheringState = 'complete'
  localDescription: { sdp: string } | null = null
  remote: { type: string; sdp: string } | null = null
  transceivers: unknown[] = []
  ontrack: ((e: unknown) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  addTransceiver(kind: string, init: unknown) { this.transceivers.push([kind, init]) }
  async createOffer() { return { type: 'offer', sdp: 'fake-offer-sdp' } }
  async setLocalDescription(d: { sdp: string }) { this.localDescription = d }
  async setRemoteDescription(d: { type: string; sdp: string }) { this.remote = d }
  close() { this.closed = true }
  async getStats() { return new Map() }
  fireTrack() { this.ontrack?.({ streams: [{ id: 'stream' }], track: {} }) }
  setConn(s: string) { this.connectionState = s; this.onconnectionstatechange?.() }
}

function fakeEnv() {
  const doc = Object.assign(new EventTarget(), { visibilityState: 'visible' }) as unknown as Document
  const win = Object.assign(new EventTarget(), { location: { hash: '', host: 'h', search: '' } }) as unknown as Window
  return { doc, win }
}

function setup(over: Partial<DesktopClientOptions> = {}) {
  const clock = new FakeClock()
  const { doc, win } = fakeEnv()
  const transports: FakeTransport[] = []
  const pcs: FakePC[] = []
  const statuses: ClientStatus[] = []
  const wakes: WakeReason[] = []
  const check = vi.fn<() => Promise<CheckVerdict>>(async () => 'retry')
  const client = createDesktopClient({
    transport: () => { const t = new FakeTransport(); transports.push(t); return t },
    createPeerConnection: () => { const p = new FakePC(); pcs.push(p); return p as unknown as RTCPeerConnection },
    check,
    clock,
    doc,
    win,
    ...over,
  })
  client.on('status', (s) => statuses.push(s))
  client.on('wake', (r) => wakes.push(r))
  return { client, clock, doc, win, transports, pcs, statuses, wakes, check }
}
type Env = ReturnType<typeof setup>

const lastT = (e: Env) => e.transports[e.transports.length - 1]
const lastPC = (e: Env) => e.pcs[e.pcs.length - 1]
const kinds = (e: Env) => e.statuses.map((s) => s.kind)

/** Walk the newest transport through hello, offer and answer. */
async function handshake(e: Env) {
  const t = lastT(e)
  t.open()
  t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
  await settle()
  t.recv({ type: 'answer', sdp: 'fake-answer-sdp' })
  await settle()
}

/** A connected, streaming client. */
async function streaming(over: Partial<DesktopClientOptions> = {}) {
  const e = setup(over)
  e.client.start()
  await handshake(e)
  lastPC(e).fireTrack()
  return e
}

afterEach(() => vi.unstubAllGlobals())

describe('transport injection', () => {
  it('sends hello first, then an offer carrying the injected peer connection SDP, with no real WebSocket', async () => {
    const ws = vi.fn(() => { throw new Error('a real WebSocket was created') })
    vi.stubGlobal('WebSocket', ws)
    const e = setup()
    e.client.start()
    const t = lastT(e)
    t.open()
    expect(t.sent[0]).toEqual({ type: 'hello', v: 1, caps: { video: 1 } })
    t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
    await settle()
    expect(t.sent[1]).toEqual({ type: 'offer', sdp: 'fake-offer-sdp' })
    t.recv({ type: 'answer', sdp: 'fake-answer-sdp' })
    await settle()
    expect(lastPC(e).remote).toEqual({ type: 'answer', sdp: 'fake-answer-sdp' })
    expect(lastPC(e).transceivers).toEqual([['video', { direction: 'recvonly' }]])
    expect(ws).not.toHaveBeenCalled()
  })

  it('builds the default socket URL from the page host and the cap in the page URL', () => {
    const loc = { host: 'mac.example', search: '?cap=a+b%2Fc', pathname: '/desktop/' } as unknown as Location
    expect(defaultDesktopWsUrl(loc)).toBe('wss://mac.example/desktop/ws?cap=a%20b%2Fc')
  })

  it('reports track, state and size events and drops an unknown state', async () => {
    const e = setup()
    const got: unknown[] = []
    e.client.on('track', (s) => got.push(['track', s]))
    e.client.on('state', (s) => got.push(['state', s]))
    e.client.on('size', (s) => got.push(['size', s]))
    e.client.start()
    await handshake(e)
    lastPC(e).fireTrack()
    lastT(e).recv({ type: 'state', state: 'locked', viewers: 2, locked: true })
    lastT(e).recv({ type: 'state', state: 'rm -rf', viewers: 1, locked: false })
    lastT(e).recv({ type: 'size', w: 2560, h: 1600 })
    expect(got).toEqual([
      ['track', { id: 'stream' }],
      ['state', { state: 'locked', viewers: 2, locked: true }],
      ['size', { w: 2560, h: 1600 }],
    ])
    expect(kinds(e)).toEqual(['streaming'])
  })
})

describe('backoff and unreachable', () => {
  it('redials after 1000, 2000, 4000, 8000 ms then every 10000 ms', async () => {
    const e = setup()
    e.client.start()
    for (const ms of [1000, 2000, 4000, 8000, 10000, 10000]) {
      const n = e.transports.length
      lastT(e).dropWith(1006)
      await e.clock.advance(ms - 1)
      expect(e.transports.length, `no redial before ${ms} ms`).toBe(n)
      await e.clock.advance(1)
      expect(e.transports.length, `redial at ${ms} ms`).toBe(n + 1)
    }
  })

  it('becomes unreachable after 30000 ms with no track and keeps redialling', async () => {
    const e = setup()
    e.client.start()
    for (let i = 0; i < 4; i++) {
      lastT(e).dropWith(1006)
      await e.clock.advance(RECONNECT.delaysMs[i])
    }
    expect(kinds(e)).toEqual(['reconnecting'])
    lastT(e).dropWith(1006)
    await e.clock.advance(30000 - e.clock.t)
    expect(kinds(e)).toEqual(['reconnecting', 'unreachable'])
    const n = e.transports.length
    lastT(e).dropWith(1006)
    await e.clock.advance(10000)
    expect(e.transports.length).toBeGreaterThan(n)
    expect(kinds(e)).toEqual(['reconnecting', 'unreachable'])
  })

  it('a returning track clears unreachable and restarts the backoff at 1000 ms', async () => {
    const e = setup()
    e.client.start()
    await e.clock.advance(30000)
    expect(kinds(e)).toEqual(['reconnecting', 'unreachable'])
    await handshake(e)
    lastPC(e).fireTrack()
    expect(kinds(e)).toEqual(['reconnecting', 'unreachable', 'streaming'])
    const n = e.transports.length
    lastT(e).dropWith(1006)
    await e.clock.advance(999)
    expect(e.transports.length).toBe(n)
    await e.clock.advance(1)
    expect(e.transports.length).toBe(n + 1)
  })

  it('a socket that never says hello is abandoned after the hello timeout and redialled', async () => {
    const e = setup()
    e.client.start()
    lastT(e).open()
    await e.clock.advance(RECONNECT.helloTimeoutMs)
    expect(lastT(e)).toBe(e.transports[0])
    expect(e.transports[0].closedWith).not.toBe('never')
    await e.clock.advance(1000)
    expect(e.transports.length).toBe(2)
  })
})

describe('beforeRedial hook', () => {
  it('is not called before the first connect, is called before each redial, and may stop the loop', async () => {
    let calls = 0
    const e = setup({ beforeRedial: async () => (++calls === 2 ? 'stop' : undefined) })
    e.client.start()
    expect(calls).toBe(0)
    lastT(e).dropWith(1006)
    await e.clock.advance(1000)
    expect(calls).toBe(1)
    expect(e.transports.length).toBe(2)
    lastT(e).dropWith(1006)
    await e.clock.advance(2000)
    expect(calls).toBe(2)
    expect(e.transports.length).toBe(2)
    await e.clock.advance(60000)
    expect(e.transports.length).toBe(2)
  })

  it('treats a rejecting hook as a failed attempt and keeps looping', async () => {
    let calls = 0
    const e = setup({ beforeRedial: async () => { if (++calls === 1) throw new Error('re-auth failed') } })
    e.client.start()
    lastT(e).dropWith(1006)
    await e.clock.advance(1000)
    expect(calls).toBe(1)
    expect(e.transports.length).toBe(1)
    await e.clock.advance(2000)
    expect(calls).toBe(2)
    expect(e.transports.length).toBe(2)
  })
})

describe('wake', () => {
  it('emits a typed wake for visibilitychange to visible, pageshow and online', async () => {
    const e = await streaming()
    ;(e.doc as unknown as { visibilityState: string }).visibilityState = 'hidden'
    e.doc.dispatchEvent(new Event('visibilitychange'))
    expect(e.wakes).toEqual([])
    ;(e.doc as unknown as { visibilityState: string }).visibilityState = 'visible'
    e.doc.dispatchEvent(new Event('visibilitychange'))
    e.win.dispatchEvent(new Event('pageshow'))
    e.win.dispatchEvent(new Event('online'))
    expect(e.wakes).toEqual(['visibility', 'pageshow', 'online'])
  })

  it('while streaming a wake pings, and a pong within 3000 ms keeps the connection', async () => {
    const e = await streaming()
    e.client.wake('visibility')
    const ping = lastT(e).last('ping')
    expect(ping).toEqual({ type: 'ping', n: expect.any(Number) })
    await e.clock.advance(1000)
    lastT(e).recv({ type: 'pong', n: ping!.n })
    await e.clock.advance(10000)
    expect(e.transports.length).toBe(1)
    expect(lastPC(e).closed).toBe(false)
  })

  it('while streaming a missing pong rebuilds the socket and the peer connection at once', async () => {
    const e = await streaming()
    e.client.wake('visibility')
    await e.clock.advance(RECONNECT.pingTimeoutMs - 1)
    expect(e.transports.length).toBe(1)
    await e.clock.advance(1)
    expect(e.transports.length).toBe(2)
    expect(e.pcs[0].closed).toBe(true)
    expect(e.transports[0].closedWith).not.toBe('never')
    expect(kinds(e)).toEqual(['streaming', 'reconnecting'])
  })

  it('while reconnecting a wake redials at once and restarts the backoff', async () => {
    const e = setup()
    e.client.start()
    lastT(e).dropWith(1006)
    await e.clock.advance(1000)
    lastT(e).dropWith(1006)
    await e.clock.advance(2000)
    lastT(e).dropWith(1006)
    expect(e.transports.length).toBe(3)
    e.client.wake('online')
    await settle()
    expect(e.transports.length).toBe(4)
    lastT(e).dropWith(1006)
    await e.clock.advance(1000)
    expect(e.transports.length).toBe(5)
  })

  it("wake('external') acts like visibility and is reported as external", async () => {
    const e = setup()
    e.client.start()
    lastT(e).dropWith(1006)
    e.client.wake('external')
    await settle()
    expect(e.wakes).toEqual(['external'])
    expect(e.transports.length).toBe(2)
  })

  it('a wake during the very first connect does not tear it down', async () => {
    const e = setup()
    e.client.start()
    e.client.wake('pageshow')
    await settle()
    expect(e.transports.length).toBe(1)
  })

  it('an open socket closing with 1012 emits server-restart and redials at once', async () => {
    const e = await streaming()
    lastT(e).dropWith(1012)
    await settle()
    expect(e.wakes).toEqual(['server-restart'])
    expect(e.transports.length).toBe(2)
    expect(e.check).not.toHaveBeenCalled()
  })
})

describe('classification by fixed codes', () => {
  async function failFirstConnect(verdict: CheckVerdict) {
    const e = setup()
    e.check.mockResolvedValue(verdict)
    e.client.start()
    lastT(e).dropWith(1006)
    await settle()
    return e
  }

  it('check ok and check retry both retry', async () => {
    for (const v of ['ok', 'retry'] as const) {
      const e = await failFirstConnect(v)
      expect(e.check).toHaveBeenCalledTimes(1)
      await e.clock.advance(1000)
      expect(e.transports.length, v).toBe(2)
    }
  })

  it.each([
    ['token', 'token_rejected'],
    ['off', 'desktop_off'],
    ['other_account', 'other_account'],
  ] as const)('check %s is final: status %s and no redial', async (verdict, kind) => {
    const e = await failFirstConnect(verdict)
    expect(kinds(e)).toEqual([kind])
    await e.clock.advance(120000)
    expect(e.transports.length).toBe(1)
    e.client.wake('online')
    await settle()
    expect(e.transports.length).toBe(1)
  })

  it('an open socket closed with 4401 or 4410 is final without asking check()', async () => {
    for (const [code, kind] of [[4401, 'token_rejected'], [4410, 'desktop_off']] as const) {
      const e = await streaming()
      lastT(e).dropWith(code)
      await e.clock.advance(60000)
      expect(kinds(e), String(code)).toEqual(['streaming', kind])
      expect(e.transports.length).toBe(1)
      expect(e.check).not.toHaveBeenCalled()
    }
  })

  it('a socket closed after hello with another code retries without asking check()', async () => {
    const e = await streaming()
    lastT(e).dropWith(1006)
    await e.clock.advance(1000)
    expect(e.transports.length).toBe(2)
    expect(e.check).not.toHaveBeenCalled()
  })

  it('ignores a late close from a superseded socket', async () => {
    const e = setup()
    e.client.start()
    const first = lastT(e)
    first.dropWith(1006)
    await e.clock.advance(1000)
    expect(e.transports.length).toBe(2)
    first.dropWith(4401)
    await settle()
    expect(kinds(e)).toEqual(['reconnecting'])
  })
})

describe('ICE', () => {
  it('failed rebuilds the socket and the peer connection and closes the old one', async () => {
    const e = await streaming()
    lastPC(e).setConn('failed')
    await e.clock.advance(1000)
    expect(e.transports.length).toBe(2)
    expect(e.pcs[0].closed).toBe(true)
    expect(e.transports[0].closedWith).not.toBe('never')
    await handshake(e)
    expect(e.pcs.length).toBe(2)
  })

  it('disconnected held 2000 ms rebuilds; recovering sooner does not', async () => {
    const e = await streaming()
    lastPC(e).setConn('disconnected')
    await e.clock.advance(1999)
    lastPC(e).setConn('connected')
    await e.clock.advance(20000)
    expect(e.transports.length).toBe(1)
    lastPC(e).setConn('disconnected')
    await e.clock.advance(RECONNECT.iceDisconnectedMs)
    await e.clock.advance(1000)
    expect(e.transports.length).toBe(2)
    expect(e.pcs[0].closed).toBe(true)
  })
})

describe('stop and scope', () => {
  it('stop closes the socket and the peer connection and removes the wake listeners', async () => {
    const e = await streaming()
    e.client.stop()
    expect(lastT(e).closedWith).not.toBe('never')
    expect(lastPC(e).closed).toBe(true)
    e.win.dispatchEvent(new Event('online'))
    await e.clock.advance(60000)
    expect(e.wakes).toEqual([])
    expect(e.transports.length).toBe(1)
  })

  it('never sends input: only hello, offer and ping leave the client', async () => {
    const e = await streaming()
    e.client.wake('visibility')
    expect(new Set(lastT(e).types())).toEqual(new Set(['hello', 'offer', 'ping']))
    expect(lastPC(e).transceivers).toHaveLength(1)
  })
})

describe('fetchCheck', () => {
  const loc = { host: 'h', search: '?cap=tok' } as unknown as Location
  const respond = (status: number, headers: Record<string, string> = {}) =>
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: status === 204 ? 204 : status, headers })))

  it.each([
    [204, {}, 'ok'],
    [401, {}, 'token'],
    [410, {}, 'off'],
    [403, { 'X-Portty': '1' }, 'other_account'],
    [403, {}, 'retry'],
    [500, {}, 'retry'],
    [503, {}, 'retry'],
  ] as const)('maps %i %j to %s', async (status, headers, want) => {
    respond(status, headers as Record<string, string>)
    expect(await fetchCheck(loc)).toBe(want)
  })

  it('maps a thrown fetch to retry and sends the cap to /desktop/check', async () => {
    const f = vi.fn(async (_url: string) => { throw new TypeError('network') })
    vi.stubGlobal('fetch', f)
    expect(await fetchCheck(loc)).toBe('retry')
    expect(f.mock.calls[0][0]).toBe('/desktop/check?cap=tok')
  })
})
