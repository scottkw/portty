// Phase 19-06 Task 2 — the /desktop/ client's seams: injectable transport,
// pre-redial hook, wake events, capped backoff, liveness ping and fixed-code
// error branching. Everything is driven through opts (fake clock, document,
// window, transport and peer connection): no real socket, timer or network.

import { describe, it, expect, afterEach, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
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

class FakeChannel {
  readyState = 'connecting'
  bufferedAmount = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  constructor(readonly label: string, readonly init: unknown) {}
  send(d: string) { this.sent.push(d) }
  open() { this.readyState = 'open'; this.onopen?.() }
  close() { this.readyState = 'closed'; this.onclose?.() }
  recv(m: unknown) { this.onmessage?.({ data: typeof m === 'string' ? m : JSON.stringify(m) }) }
}

class FakePC {
  closed = false
  calls: string[] = []
  channels: FakeChannel[] = []
  connectionState = 'new'
  iceGatheringState = 'complete'
  localDescription: { sdp: string } | null = null
  remote: { type: string; sdp: string } | null = null
  transceivers: unknown[] = []
  ontrack: ((e: unknown) => void) | null = null
  onconnectionstatechange: (() => void) | null = null
  onicegatheringstatechange: (() => void) | null = null
  addTransceiver(kind: string, init: unknown) { this.transceivers.push([kind, init]) }
  createDataChannel(label: string, init: unknown) {
    this.calls.push('createDataChannel')
    const ch = new FakeChannel(label, init)
    this.channels.push(ch)
    return ch
  }
  async createOffer() {
    this.calls.push('createOffer')
    return { type: 'offer', sdp: 'fake-offer-sdp' } }
  async setLocalDescription(d: { sdp: string }) { this.localDescription = d }
  async setRemoteDescription(d: { type: string; sdp: string }) { this.remote = d }
  close() { this.closed = true }
  async getStats() { return new Map() }
  fireTrack() { this.ontrack?.({ streams: [{ id: 'stream' }], track: {} }) }
  /** A real picture: the track arrived and ICE connected. */
  fireStreaming() { this.fireTrack(); this.setConn('connected') }
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
  lastPC(e).fireStreaming()
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
    expect(t.sent[0]).toEqual({ type: 'hello', v: 1, caps: { video: 1, input: 1 } })
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
    lastPC(e).fireStreaming()
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
    lastPC(e).fireStreaming()
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

/** A handshake whose server hello offers the input channel, with rights. */
async function handshakeWithInput(e: Env, rights: object = { control: true, clipboard: true, surface: true }) {
  const t = lastT(e)
  t.open()
  t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1, input: 1 }, rights })
  await settle()
  t.recv({ type: 'answer', sdp: 'fake-answer-sdp' })
  await settle()
}

describe('input data channel (Phase 20-06)', () => {
  it('with server caps input 1 creates one ordered channel labelled input before createOffer', async () => {
    const e = setup()
    e.client.start()
    await handshakeWithInput(e)
    const pc = lastPC(e) as unknown as FakePC
    expect(pc.channels).toHaveLength(1)
    expect(pc.channels[0].label).toBe('input')
    expect(pc.channels[0].init).toEqual({ ordered: true })
    expect(pc.calls.indexOf('createDataChannel')).toBeLessThan(pc.calls.indexOf('createOffer'))
  })

  it('without server input caps no channel is created and sendInput says no', async () => {
    const e = setup()
    e.client.start()
    await handshake(e)
    expect((lastPC(e) as unknown as FakePC).channels).toHaveLength(0)
    expect(e.client.sendInput({ t: 'hb' })).toBe(false)
  })

  it('sendInput is false until the channel opens, then sends JSON; a droppable move is skipped over 64 KiB', async () => {
    const e = setup()
    e.client.start()
    await handshakeWithInput(e)
    const ch = (lastPC(e) as unknown as FakePC).channels[0]
    expect(e.client.sendInput({ t: 'hb' })).toBe(false)
    ch.open()
    expect(e.client.sendInput({ t: 'hb' })).toBe(true)
    expect(ch.sent).toEqual(['{"t":"hb"}'])
    ch.bufferedAmount = 65537
    expect(e.client.sendInput({ t: 'move', x: 0, y: 0, e: 1 }, true)).toBe(false)
    expect(e.client.sendInput({ t: 'release' })).toBe(true)
    expect(ch.sent).toEqual(['{"t":"hb"}', '{"t":"release"}'])
  })

  it('reports the channel opening and closing, and the closing of a torn-down connection once', async () => {
    const e = setup()
    const got: boolean[] = []
    e.client.on('input', (v) => got.push(v.open))
    e.client.start()
    await handshakeWithInput(e)
    const ch = (lastPC(e) as unknown as FakePC).channels[0]
    ch.open()
    ch.close()
    expect(got).toEqual([true, false])
    ;(lastPC(e) as unknown as FakePC).channels[0].open()
    lastT(e).dropWith(1006)
    expect(got).toEqual([true, false, true, false])
  })

  it('emits the rights the hello grants, missing fields false', async () => {
    const e = setup()
    const got: unknown[] = []
    e.client.on('rights', (r) => got.push(r))
    e.client.start()
    await handshakeWithInput(e, { control: true })
    expect(got).toEqual([{ control: true, clipboard: false, surface: false }])
  })

  it('learns the surface epoch from a surface message and forgets it with the connection', async () => {
    const e = setup()
    const got: unknown[] = []
    e.client.on('surface', (s) => got.push(s))
    e.client.start()
    await handshakeWithInput(e)
    expect(e.client.currentEpoch()).toBeNull()
    lastT(e).recv({ type: 'surface', e: 2, kind: 'window', id: 4242, name: 'Terminal', w: 1280, h: 800, reason: '' })
    expect(e.client.currentEpoch()).toBe(2)
    expect(got).toEqual([{ e: 2, kind: 'window', id: 4242, name: 'Terminal', w: 1280, h: 800, reason: '' }])
    lastT(e).recv({ type: 'surface', e: 'x', kind: 'window', id: 1, name: 'n', w: 1, h: 1 })
    lastT(e).recv({ type: 'surface', e: 5, kind: 'monitor', id: 1, name: 'n', w: 1, h: 1 })
    expect(e.client.currentEpoch()).toBe(2)
    lastT(e).recv({ type: 'surface', e: 3, kind: 'display', id: 1, name: 'Main', w: 1470, h: 956, reason: '<b>' })
    expect((got[1] as { reason: string }).reason).toBe('')
    lastT(e).dropWith(1006)
    expect(e.client.currentEpoch()).toBeNull()
  })

  it('turns the golden server lines control-ok, control-needs-accessibility and cursor into typed events', async () => {
    const lines = readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/input-v1.jsonl'), 'utf8')
      .trim().split('\n').map((l) => JSON.parse(l) as { name: string; dir: string; wire: object })
    const wire = (name: string) => lines.find((l) => l.name === name)!.wire
    const e = setup()
    const got: unknown[] = []
    e.client.on('control', (c) => got.push(['control', c]))
    e.client.on('cursor', (c) => got.push(['cursor', c]))
    e.client.start()
    await handshakeWithInput(e)
    const ch = (lastPC(e) as unknown as FakePC).channels[0]
    ch.recv(wire('control-ok'))
    ch.recv(wire('control-needs-accessibility'))
    ch.recv(wire('cursor'))
    expect(got).toEqual([
      ['control', { input: 'ok', window: '' }],
      ['control', { input: 'needs_accessibility', window: 'raise_failed' }],
      ['cursor', { x: 0.5, y: 0.25, e: 1 }],
    ])
  })

  it('drops unknown control values, a non-finite cursor, binary and non-JSON frames', async () => {
    const e = setup()
    const got: unknown[] = []
    e.client.on('control', (c) => got.push(c))
    e.client.on('cursor', (c) => got.push(c))
    e.client.start()
    await handshakeWithInput(e)
    const ch = (lastPC(e) as unknown as FakePC).channels[0]
    ch.recv({ t: 'control', input: 'rm -rf', window: '' })
    ch.recv({ t: 'control', input: 'ok', window: '<img>' })
    ch.recv({ t: 'cursor', x: 'a', y: 0, e: 1 })
    ch.recv({ t: 'cursor', x: 0, y: 0, e: 1.5 })
    ch.recv({ t: 'teleport' })
    ch.recv('not json')
    ch.recv(null)
    ch.onmessage?.({ data: new ArrayBuffer(4) })
    expect(got).toEqual([{ input: 'ok', window: '' }])
  })

  it('send puts a signalling message on the socket', async () => {
    const e = setup()
    e.client.start()
    await handshake(e)
    e.client.send({ type: 'surfaces' })
    expect(lastT(e).last('surfaces')).toEqual({ type: 'surfaces' })
  })
})

describe('surface list and select (Phase 20-07)', () => {
  const golden = () =>
    readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-v1.jsonl'), 'utf8')
      .trim().split('\n').map((l) => JSON.parse(l) as { name: string; dir: string; wire: object })
  const wire = (name: string) => golden().find((l) => l.name === name)!.wire

  async function connected() {
    const e = setup()
    e.client.start()
    await handshakeWithInput(e)
    return e
  }

  it('requestSurfaces sends the golden surfaces request and resolves with the reply (golden surfaces line)', async () => {
    const e = await connected()
    const p = e.client.requestSurfaces()
    expect(lastT(e).last('surfaces')).toEqual(wire('surfaces-request'))
    lastT(e).recv(wire('surfaces'))
    await expect(p).resolves.toEqual({
      displays: [
        { id: 1, name: 'Main Display', w: 1470, h: 956, main: true },
        { id: 2, name: 'Display 2', w: 1920, h: 1080, main: false },
      ],
      windows: [{ id: 4242, app: 'Terminal', title: 'zsh', name: 'Terminal \u2014 zsh' }],
    })
  })

  it('emits a surfaces event for a good reply', async () => {
    const e = await connected()
    const got: unknown[] = []
    e.client.on('surfaces', (s) => got.push(s))
    lastT(e).recv(wire('surfaces'))
    expect(got).toHaveLength(1)
  })

  it('a reply with error list_failed rejects', async () => {
    const e = await connected()
    const p = e.client.requestSurfaces()
    lastT(e).recv(wire('surfaces-error'))
    await expect(p).rejects.toThrow('list_failed')
  })

  it('rejects after 5 s with no reply, and a late reply does nothing', async () => {
    const e = await connected()
    const p = e.client.requestSurfaces()
    const done = vi.fn()
    p.catch(done)
    await e.clock.advance(4999)
    expect(done).not.toHaveBeenCalled()
    await e.clock.advance(1)
    await expect(p).rejects.toThrow('timeout')
    lastT(e).recv(wire('surfaces'))
  })

  it('rejects when there is no connection to ask', async () => {
    const e = setup()
    await expect(e.client.requestSurfaces()).rejects.toThrow('not connected')
  })

  it('a dropped connection rejects what is waiting', async () => {
    const e = await connected()
    const p = e.client.requestSurfaces()
    lastT(e).dropWith(1006)
    await expect(p).rejects.toThrow('not connected')
  })

  it('keeps only typed fields from the list: strings as strings, ids and sizes as integers, junk dropped', async () => {
    const e = await connected()
    const p = e.client.requestSurfaces()
    lastT(e).recv({
      type: 'surfaces',
      displays: [{ id: 1, name: 'Main Display', w: 1470, h: 956, main: true }, { id: 'x', name: 'bad', w: 1, h: 1 }, { id: 3, name: 7, w: 1.5, h: 1 }, null],
      windows: [{ id: 9, app: 'A', title: '<img src=x onerror=alert(1)>', name: 'A \u2014 <img src=x onerror=alert(1)>' }, { id: 1.5, name: 'z' }, { id: 4, name: 12 }],
    })
    await expect(p).resolves.toEqual({
      displays: [{ id: 1, name: 'Main Display', w: 1470, h: 956, main: true }],
      windows: [{ id: 9, app: 'A', title: '<img src=x onerror=alert(1)>', name: 'A \u2014 <img src=x onerror=alert(1)>' }],
    })
  })

  it('selectSurface sends the golden select line for a window and for a display', async () => {
    const e = await connected()
    e.client.selectSurface('window', 4242)
    expect(lastT(e).last('select')).toEqual(wire('select-window'))
    e.client.selectSurface('display', 0)
    expect(lastT(e).last('select')).toEqual(wire('select-main'))
  })

  it('select_failed emits selectFailed with its code (golden line)', async () => {
    const e = await connected()
    const got: unknown[] = []
    e.client.on('selectFailed', (v) => got.push(v))
    lastT(e).recv(wire('select-failed'))
    expect(got).toEqual([{ code: 'switch_failed' }])
    lastT(e).recv({ type: 'select_failed', code: 7 })
    expect(got[1]).toEqual({ code: '' })
  })
})

describe('picture means ICE connected, not ontrack (Phase 19 review follow-up)', () => {
  it('a track alone is announced to the page but is not a picture', async () => {
    const e = setup()
    const tracks: unknown[] = []
    e.client.on('track', (s) => tracks.push(s))
    e.client.start()
    await handshake(e)
    lastPC(e).fireTrack()
    expect(tracks).toHaveLength(1)
    expect(kinds(e)).toEqual([])
  })

  it.each(['track first', 'connected first'] as const)('streaming starts once both the track and ICE connected are in (%s)', async (order) => {
    const e = setup()
    e.client.start()
    await handshake(e)
    if (order === 'track first') {
      lastPC(e).fireTrack()
      expect(kinds(e)).toEqual([])
      lastPC(e).setConn('connected')
    } else {
      lastPC(e).setConn('connected')
      expect(kinds(e)).toEqual([])
      lastPC(e).fireTrack()
    }
    expect(kinds(e)).toEqual(['streaming'])
  })

  it('ICE failing after every ontrack still backs off 1, 2, 4 s instead of restarting at 1 s', async () => {
    const e = setup()
    e.client.start()
    let n = 1
    for (const ms of [1000, 2000, 4000]) {
      await handshake(e)
      lastPC(e).fireTrack()
      lastPC(e).setConn('failed')
      await e.clock.advance(ms - 1)
      expect(e.transports.length, `no redial before ${ms} ms`).toBe(n)
      await e.clock.advance(1)
      expect(e.transports.length, `redial at ${ms} ms`).toBe(++n)
    }
  })

  it("ICE that never connects ends in Can't reach this Mac after 30 s, never in streaming", async () => {
    const e = setup()
    e.client.start()
    for (let i = 0; i < 6; i++) {
      await handshake(e)
      lastPC(e).fireTrack()
      await e.clock.advance(5000) // ICE trying, the way a real browser does for about 30 s
      lastPC(e).setConn('failed')
      await e.clock.advance(RECONNECT.delaysMs[Math.min(i, RECONNECT.delaysMs.length - 1)])
    }
    expect(kinds(e)).toContain('unreachable')
    expect(kinds(e)).not.toContain('streaming')
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
