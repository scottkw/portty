// Phase 19-06 Task 2 — the /desktop/ client's seams: injectable transport,
// pre-redial hook, wake events, capped backoff, liveness ping and fixed-code
// error branching. Everything is driven through opts (fake clock, document,
// window, transport and peer connection): no real socket, timer or network.

import { describe, it, expect, afterEach, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import {
  CLIP_CHUNK_BYTES,
  CLIP_FRAME_MAX,
  CLIP_MAX_BYTES,
  CLIP_TIMEOUT_MS,
  RECONNECT,
  chunkClipText,
  createDesktopClient,
  defaultDesktopWsUrl,
  fetchCheck,
  withOpusStereo,
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

/** What the fake peer connection offers; a test sets it, afterEach restores it. */
let offerSdp = 'fake-offer-sdp'

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
    return { type: 'offer', sdp: offerSdp } }
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

afterEach(() => {
  offerSdp = 'fake-offer-sdp'
  vi.unstubAllGlobals()
})

describe('transport injection', () => {
  it('sends hello first, then an offer carrying the injected peer connection SDP, with no real WebSocket', async () => {
    const ws = vi.fn(() => { throw new Error('a real WebSocket was created') })
    vi.stubGlobal('WebSocket', ws)
    const e = setup()
    e.client.start()
    const t = lastT(e)
    t.open()
    expect(t.sent[0]).toEqual({ type: 'hello', v: 1, caps: { video: 1, audio: 1, input: 1 } })
    t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
    await settle()
    expect(t.sent[1]).toEqual({ type: 'offer', sdp: 'fake-offer-sdp' })
    t.recv({ type: 'answer', sdp: 'fake-answer-sdp' })
    await settle()
    expect(lastPC(e).remote).toEqual({ type: 'answer', sdp: 'fake-answer-sdp' })
    expect(lastPC(e).transceivers).toEqual([['video', { direction: 'recvonly' }], ['audio', { direction: 'recvonly' }]])
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

describe('clipboard transfer (Phase 20-08)', () => {
  const golden = () =>
    readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-v1.jsonl'), 'utf8')
      .trim().split('\n').map((l) => JSON.parse(l) as { name: string; dir: string; wire: Record<string, unknown> })
  const wire = (name: string) => golden().find((l) => l.name === name)!.wire
  /** The golden server line, answering the transfer the client just started. */
  const reply = (name: string, xid: string, over: object = {}) => ({ ...wire(name), xid, ...over })
  const enc = (s: string) => new TextEncoder().encode(s).length

  async function connected() {
    const e = setup()
    e.client.start()
    await handshakeWithInput(e)
    return e
  }
  const sentXid = (e: Env, type: string) => (lastT(e).last(type) as { xid: string }).xid

  describe('copyFromMac', () => {
    it('sends the golden clip_get shape and resolves ok with the assembled text (golden clip_data and clip_result lines)', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const sent = lastT(e).last('clip_get') as Record<string, unknown>
      expect(Object.keys(sent).sort()).toEqual(Object.keys(wire('clip-get')).sort())
      expect(typeof sent.xid).toBe('string')
      lastT(e).recv(reply('clip-data', sent.xid as string))
      lastT(e).recv(reply('clip-result-ok', sent.xid as string))
      await expect(p).resolves.toEqual({ outcome: 'ok', text: 'hello' })
    })

    it('joins several chunks in seq order', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const xid = sentXid(e, 'clip_get')
      lastT(e).recv({ type: 'clip_data', xid, seq: 0, total: 2, len: 7, text: 'hel' })
      lastT(e).recv({ type: 'clip_data', xid, seq: 1, total: 2, len: 7, text: 'lo, ' })
      lastT(e).recv(reply('clip-result-ok', xid))
      await expect(p).resolves.toEqual({ outcome: 'ok', text: 'hello, ' })
    })

    it('counts bytes, not characters: a multi-byte text must match len', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const xid = sentXid(e, 'clip_get')
      lastT(e).recv({ type: 'clip_data', xid, seq: 0, total: 1, len: enc('é☃😀'), text: 'é☃😀' })
      lastT(e).recv(reply('clip-result-ok', xid))
      await expect(p).resolves.toEqual({ outcome: 'ok', text: 'é☃😀' })
    })

    it.each([
      ['a gap (0 then 2 of 3)', [[0, 3], [2, 3]]],
      ['a duplicate (0 then 0)', [[0, 2], [0, 2]]],
      ['a changed total', [[0, 2], [1, 3]]],
    ])('fails on %s', async (_n, seqs) => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const xid = sentXid(e, 'clip_get')
      for (const [seq, total] of seqs) lastT(e).recv({ type: 'clip_data', xid, seq, total, len: 6, text: 'ab' })
      lastT(e).recv(reply('clip-result-ok', xid))
      await expect(p).resolves.toEqual({ outcome: 'failed' })
    })

    it('fails when len does not match the assembled bytes, or a changed len, or a missing chunk at the end', async () => {
      for (const frames of [
        [{ seq: 0, total: 1, len: 6, text: 'hello' }],
        [{ seq: 0, total: 2, len: 5, text: 'he' }, { seq: 1, total: 2, len: 6, text: 'llo' }],
        [{ seq: 0, total: 2, len: 5, text: 'hel' }],
        [],
      ]) {
        const e = await connected()
        const p = e.client.copyFromMac()
        const xid = sentXid(e, 'clip_get')
        for (const f of frames) lastT(e).recv({ type: 'clip_data', xid, ...f })
        lastT(e).recv(reply('clip-result-ok', xid))
        await expect(p).resolves.toEqual({ outcome: 'failed' })
      }
    })

    it('fails on a chunk over 16384 bytes, over len, over the 1 MiB cap, or with a non-string text', async () => {
      for (const f of [
        { seq: 0, total: 1, len: CLIP_CHUNK_BYTES + 1, text: 'a'.repeat(CLIP_CHUNK_BYTES + 1) },
        { seq: 0, total: 1, len: 3, text: 'abcd' },
        { seq: 0, total: 1, len: CLIP_MAX_BYTES + 1, text: 'a' },
        { seq: 0, total: 1, len: 1, text: 7 },
      ]) {
        const e = await connected()
        const p = e.client.copyFromMac()
        lastT(e).recv({ type: 'clip_data', xid: sentXid(e, 'clip_get'), ...f })
        await expect(p).resolves.toEqual({ outcome: 'failed' })
      }
    })

    it('answers concealed, empty, waiting_approval and too_large with no text (golden concealed and waiting lines)', async () => {
      for (const [line, outcome] of [['clip-result-concealed', 'concealed'], ['clip-result-waiting', 'waiting_approval']] as const) {
        const e = await connected()
        const p = e.client.copyFromMac()
        lastT(e).recv(reply(line, sentXid(e, 'clip_get')))
        await expect(p).resolves.toEqual({ outcome })
      }
      for (const outcome of ['empty', 'too_large', 'failed', 'forbidden', 'busy']) {
        const e = await connected()
        const p = e.client.copyFromMac()
        lastT(e).recv({ type: 'clip_result', xid: sentXid(e, 'clip_get'), outcome })
        await expect(p).resolves.toEqual({ outcome })
      }
    })

    it('an unknown outcome is a failure, and a message for another xid is ignored', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const xid = sentXid(e, 'clip_get')
      lastT(e).recv({ type: 'clip_data', xid: 'other', seq: 0, total: 1, len: 5, text: 'hello' })
      lastT(e).recv({ type: 'clip_result', xid: 'other', outcome: 'concealed' })
      lastT(e).recv({ type: 'clip_result', xid, outcome: 'whatever' })
      await expect(p).resolves.toEqual({ outcome: 'failed' })
    })

    it('a text that arrived is dropped when the result is not ok', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const xid = sentXid(e, 'clip_get')
      lastT(e).recv({ type: 'clip_data', xid, seq: 0, total: 1, len: 5, text: 'hello' })
      lastT(e).recv({ type: 'clip_result', xid, outcome: 'failed' })
      await expect(p).resolves.toEqual({ outcome: 'failed' })
    })

    it('times out as failed after CLIP_TIMEOUT_MS, and a late answer does nothing', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      const xid = sentXid(e, 'clip_get')
      const done = vi.fn()
      void p.then(done)
      await e.clock.advance(CLIP_TIMEOUT_MS - 1)
      expect(done).not.toHaveBeenCalled()
      await e.clock.advance(1)
      await expect(p).resolves.toEqual({ outcome: 'failed' })
      lastT(e).recv(reply('clip-result-ok', xid))
      // a new transfer is free to start
      void e.client.copyFromMac()
      expect(lastT(e).sent.filter((m) => m.type === 'clip_get')).toHaveLength(2)
    })

    it('a dropped connection resolves failed at once', async () => {
      const e = await connected()
      const p = e.client.copyFromMac()
      lastT(e).dropWith(1006)
      await expect(p).resolves.toEqual({ outcome: 'failed' })
    })

    it('is failed with nothing sent when there is no connection', async () => {
      const e = setup()
      await expect(e.client.copyFromMac()).resolves.toEqual({ outcome: 'failed' })
    })

    it('answers busy at once, sending nothing, while another transfer is in flight', async () => {
      const e = await connected()
      void e.client.copyFromMac()
      await expect(e.client.copyFromMac()).resolves.toEqual({ outcome: 'busy' })
      await expect(e.client.pasteToMac('x')).resolves.toEqual({ outcome: 'busy' })
      expect(lastT(e).sent.filter((m) => m.type === 'clip_get' || m.type === 'clip_put')).toHaveLength(1)
    })
  })

  describe('pasteToMac', () => {
    it('sends clip_put in the golden shape, one xid, seq from 0, total and len on every chunk, and resolves on clip_result', async () => {
      const e = await connected()
      const p = e.client.pasteToMac('hello, ')
      const puts = lastT(e).sent.filter((m) => m.type === 'clip_put')
      expect(puts).toHaveLength(1)
      expect(Object.keys(puts[0]).sort()).toEqual(Object.keys(wire('clip-put-0')).sort())
      expect(puts[0]).toMatchObject({ seq: 0, total: 1, len: 7, text: 'hello, ' })
      lastT(e).recv({ type: 'clip_result', xid: puts[0].xid, outcome: 'ok' })
      await expect(p).resolves.toEqual({ outcome: 'ok' })
    })

    it('splits a long text: contiguous seq, the same xid, total and len, texts that join back, every chunk at most 16384 bytes', async () => {
      const e = await connected()
      const text = 'é☃😀 abc'.repeat(6000)
      void e.client.pasteToMac(text)
      const puts = lastT(e).sent.filter((m) => m.type === 'clip_put') as { xid: string; seq: number; total: number; len: number; text: string }[]
      expect(puts.length).toBeGreaterThan(1)
      expect(new Set(puts.map((m) => m.xid)).size).toBe(1)
      expect(puts.map((m) => m.seq)).toEqual(puts.map((_, i) => i))
      expect(new Set(puts.map((m) => m.total))).toEqual(new Set([puts.length]))
      expect(new Set(puts.map((m) => m.len))).toEqual(new Set([enc(text)]))
      expect(puts.map((m) => m.text).join('')).toBe(text)
      for (const m of puts) {
        expect(enc(m.text)).toBeLessThanOrEqual(CLIP_CHUNK_BYTES)
        expect(enc(JSON.stringify(m))).toBeLessThanOrEqual(CLIP_FRAME_MAX)
      }
    })

    it('sends nothing for an empty text (empty) or one over 1 MiB (too_large); exactly 1 MiB is sent', async () => {
      const e = await connected()
      await expect(e.client.pasteToMac('')).resolves.toEqual({ outcome: 'empty' })
      await expect(e.client.pasteToMac('a'.repeat(CLIP_MAX_BYTES + 1))).resolves.toEqual({ outcome: 'too_large' })
      await expect(e.client.pasteToMac('é'.repeat(CLIP_MAX_BYTES / 2 + 1))).resolves.toEqual({ outcome: 'too_large' })
      expect(lastT(e).sent.filter((m) => m.type === 'clip_put')).toHaveLength(0)
      void e.client.pasteToMac('a'.repeat(CLIP_MAX_BYTES))
      expect(lastT(e).sent.filter((m) => m.type === 'clip_put').length).toBeGreaterThan(60)
    })

    it('resolves the daemon outcome, a timeout, and a dropped connection as failed', async () => {
      const e = await connected()
      const p1 = e.client.pasteToMac('abc')
      lastT(e).recv({ type: 'clip_result', xid: sentXid(e, 'clip_put'), outcome: 'forbidden' })
      await expect(p1).resolves.toEqual({ outcome: 'forbidden' })
      const p2 = e.client.pasteToMac('abc')
      await e.clock.advance(CLIP_TIMEOUT_MS)
      await expect(p2).resolves.toEqual({ outcome: 'failed' })
      const p3 = e.client.pasteToMac('abc')
      lastT(e).dropWith(1006)
      await expect(p3).resolves.toEqual({ outcome: 'failed' })
    })
  })

  describe('chunkClipText', () => {
    it('gives 1 chunk for 16384 ASCII bytes and 2 for 16385', () => {
      expect(chunkClipText('a'.repeat(16384))).toHaveLength(1)
      expect(chunkClipText('a'.repeat(16385))).toHaveLength(2)
      expect(chunkClipText('')).toEqual([])
    })

    it('never splits a code point: 16383 a then é (2 bytes) moves é to the next chunk, and an astral pair stays whole', () => {
      const c = chunkClipText('a'.repeat(16383) + 'é')
      expect(c).toEqual(['a'.repeat(16383), 'é'])
      const emoji = 'a'.repeat(16382) + '😀'
      expect(chunkClipText(emoji)).toEqual(['a'.repeat(16382), '😀'])
    })

    it('keeps every encoded frame at most 30000 bytes: 20000 double quotes, and control characters that escape to six', () => {
      for (const t of ['"'.repeat(20000), '\u0001'.repeat(16000), '\n'.repeat(20000), 'x"'.repeat(30000)]) {
        const chunks = chunkClipText(t)
        expect(chunks.join('')).toBe(t)
        for (const [seq, text] of chunks.entries()) {
          const frame = JSON.stringify({ type: 'clip_put', xid: 'x999999', seq, total: chunks.length, len: enc(t), text })
          expect(enc(frame)).toBeLessThanOrEqual(CLIP_FRAME_MAX)
          expect(enc(text)).toBeLessThanOrEqual(CLIP_CHUNK_BYTES)
        }
      }
      expect(chunkClipText('"'.repeat(20000)).length).toBeGreaterThan(1)
    })
  })
})

describe('audio (Phase 23-06)', () => {
  const CHROME_AUDIO = [
    'v=0',
    'm=video 9 UDP/TLS/RTP/SAVPF 96',
    'a=rtpmap:96 VP8/90000',
    'a=fmtp:96 profile-id=0',
    'm=audio 9 UDP/TLS/RTP/SAVPF 111 63',
    'a=rtpmap:111 opus/48000/2',
    'a=fmtp:111 minptime=10;useinbandfec=1',
    'a=rtpmap:63 red/48000/2',
    'a=fmtp:63 111/111',
    '',
  ].join('\r\n')

  it('withOpusStereo adds stereo=1 to the Opus fmtp line and touches no other line', () => {
    const out = withOpusStereo(CHROME_AUDIO)
    expect(out.split('\r\n')).toEqual(
      CHROME_AUDIO.split('\r\n').map((l) => (l === 'a=fmtp:111 minptime=10;useinbandfec=1' ? 'a=fmtp:111 minptime=10;useinbandfec=1;stereo=1' : l)),
    )
  })

  it('withOpusStereo replaces stereo=0, adds an fmtp line when there is none, and leaves a video-only offer alone', () => {
    expect(withOpusStereo('a=rtpmap:111 opus/48000/2\r\na=fmtp:111 stereo=0;useinbandfec=1\r\n')).toBe(
      'a=rtpmap:111 opus/48000/2\r\na=fmtp:111 useinbandfec=1;stereo=1\r\n',
    )
    expect(withOpusStereo('m=audio 9 RTP/AVP 111\r\na=rtpmap:111 opus/48000/2\r\na=sendrecv\r\n')).toBe(
      'm=audio 9 RTP/AVP 111\r\na=rtpmap:111 opus/48000/2\r\na=fmtp:111 stereo=1\r\na=sendrecv\r\n',
    )
    expect(withOpusStereo('a=rtpmap:111 opus/48000/2\na=fmtp:111 minptime=10\n')).toBe('a=rtpmap:111 opus/48000/2\na=fmtp:111 minptime=10;stereo=1\n')
    expect(withOpusStereo('m=video 9 RTP/AVP 96\r\na=rtpmap:96 VP8/90000\r\n')).toBe('m=video 9 RTP/AVP 96\r\na=rtpmap:96 VP8/90000\r\n')
  })

  it('adds the audio recvonly transceiver after video and before createOffer, and offers stereo', async () => {
    offerSdp = CHROME_AUDIO
    const e = setup()
    e.client.start()
    await handshake(e)
    const pc = lastPC(e)
    expect(pc.transceivers).toEqual([['video', { direction: 'recvonly' }], ['audio', { direction: 'recvonly' }]])
    expect(pc.localDescription?.sdp).toContain('a=fmtp:111 minptime=10;useinbandfec=1;stereo=1')
    expect(lastT(e).sent[1]).toMatchObject({ type: 'offer' })
    expect((lastT(e).sent[1] as { sdp: string }).sdp).toContain('stereo=1')
  })

  // Phase 23-08: the daemon's quality message (golden lines from internal/desktop/testdata/signal-v1.jsonl).
  it('turns the golden quality lines into quality events and takes only a true slow as slow', async () => {
    const e = setup()
    const got: { slow: boolean }[] = []
    e.client.on('quality', (q) => got.push(q))
    e.client.start()
    await handshakeWithInput(e)
    const lines = readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-v1.jsonl'), 'utf8')
      .trim().split('\n').map((l) => JSON.parse(l) as { name: string; wire: object })
    for (const n of ['quality-slow-true', 'quality-slow-false']) lastT(e).recv(lines.find((l) => l.name === n)!.wire)
    lastT(e).recv({ type: 'quality', slow: 'yes' })
    expect(got).toEqual([{ slow: true }, { slow: false }, { slow: false }])
  })

  // Phase 23-07: the unlock panel's wire (golden lines from internal/desktop/testdata/signal-v1.jsonl).
  describe('unlock', () => {
    const goldenLines = () =>
      readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-v1.jsonl'), 'utf8')
        .trim().split('\n').map((l) => JSON.parse(l) as { name: string; dir: string; wire: object })

    it('turns every golden unlock line into a typed event', async () => {
      const e = setup()
      const got: unknown[] = []
      e.client.on('unlock', (u) => got.push(u))
      e.client.start()
      await handshakeWithInput(e)
      const lines = goldenLines().filter((l) => l.name.startsWith('unlock-'))
      expect(lines).toHaveLength(11)
      for (const l of lines) lastT(e).recv(l.wire)
      const b = { failures: 0, waitMs: 0, reason: '', byPortty: false }
      expect(got).toEqual([
        { ...b, state: 'unlocking' },
        { ...b, state: 'unlocked', byPortty: true },
        { ...b, state: 'idle' },
        { ...b, state: 'retrying' },
        { ...b, state: 'failed', failures: 1 },
        { ...b, state: 'rate_limited', failures: 1, waitMs: 3000 },
        { ...b, state: 'paused', failures: 3, waitMs: 900000 },
        { ...b, state: 'refused', reason: 'caps_lock' },
        { ...b, state: 'refused', reason: 'unmappable' },
        { ...b, state: 'unavailable', reason: 'accessibility' },
        { ...b, state: 'error' },
      ])
    })

    it('takes only enum values and bounded numbers from an unlock message', async () => {
      const e = setup()
      const got: { state: string; failures: number; waitMs: number; reason: string }[] = []
      e.client.on('unlock', (u) => got.push(u))
      e.client.start()
      await handshakeWithInput(e)
      lastT(e).recv({ type: 'unlock', state: 'open-sesame', failures: 0 })
      lastT(e).recv({ type: 'unlock', state: 7 })
      lastT(e).recv({ type: 'unlock', state: 'failed', failures: -4, wait_ms: 'x', reason: '<b>' })
      lastT(e).recv({ type: 'unlock', state: 'paused', failures: 3, wait_ms: 9e15 })
      expect(got).toHaveLength(2)
      expect(got[0]).toMatchObject({ state: 'failed', failures: 0, waitMs: 0, reason: '' })
      expect(got[1].waitMs).toBe(3600000)
    })

    it('sendUnlock is false until the input channel opens, then puts the bytes on it as one message and sends nothing on the socket', async () => {
      const e = setup()
      e.client.start()
      await handshakeWithInput(e)
      const ch = (lastPC(e) as unknown as FakePC).channels[0]
      const bytes = Uint8Array.from([1, 0x61, 0x62])
      expect(e.client.sendUnlock(bytes)).toBe(false)
      ch.open()
      const before = lastT(e).sent.length
      expect(e.client.sendUnlock(bytes)).toBe(true)
      expect(ch.sent).toHaveLength(1)
      expect(ch.sent[0]).toBe(bytes)
      expect(lastT(e).sent).toHaveLength(before)
    })
  })

  it('emits caps from the server hello: audio and unlock are booleans, missing is false', async () => {
    const e = setup()
    const seen: unknown[] = []
    e.client.on('caps', (c) => seen.push(c))
    e.client.start()
    const t = lastT(e)
    t.open()
    t.recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1, audio: 1, unlock: 1 } })
    await settle()
    expect(seen).toEqual([{ audio: true, unlock: true }])
    // A reconnect's hello with no audio says so.
    t.dropWith(1006)
    await e.clock.advance(RECONNECT.delaysMs[0])
    lastT(e).open()
    lastT(e).recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
    await settle()
    expect(seen[1]).toEqual({ audio: false, unlock: false })
  })

  it('carries the state message audio failed, and only that value', async () => {
    const e = setup()
    const seen: unknown[] = []
    e.client.on('state', (s) => seen.push(s))
    e.client.start()
    await handshake(e)
    lastT(e).recv({ type: 'state', state: 'streaming', viewers: 1, audio: 'failed' })
    lastT(e).recv({ type: 'state', state: 'streaming', viewers: 1, audio: 'whatever' })
    lastT(e).recv({ type: 'state', state: 'streaming', viewers: 1 })
    expect(seen).toEqual([
      { state: 'streaming', viewers: 1, locked: false, audio: 'failed' },
      { state: 'streaming', viewers: 1, locked: false },
      { state: 'streaming', viewers: 1, locked: false },
    ])
  })

  it('an audio track alone is not a picture; the video track is, and both play from one stream', async () => {
    const e = setup()
    const streams: unknown[] = []
    e.client.on('track', (s) => streams.push(s))
    e.client.start()
    await handshake(e)
    const pc = lastPC(e)
    const shared = { id: 'portty-desktop' }
    pc.ontrack?.({ streams: [shared], track: { kind: 'audio' } })
    pc.setConn('connected')
    expect(streams).toEqual([])
    expect(kinds(e)).not.toContain('streaming')
    pc.ontrack?.({ streams: [shared], track: { kind: 'video' } })
    expect(streams).toEqual([shared])
    expect(kinds(e)).toContain('streaming')
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
    expect(lastPC(e).transceivers).toHaveLength(2)
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
