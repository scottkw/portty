// Phase 22-07 — the desktop client in guest mode: the share server's own socket and check, no offer
// until {type: admission, state: admitted}, live rights and lease, and the fixed close reasons.
// Driven through opts (fake clock, transport, peer connection): no real socket, timer or network.

import { describe, it, expect, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import {
  RECONNECT,
  createDesktopClient,
  defaultGuestMode,
  defaultGuestWsUrl,
  fetchGuestCheck,
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
    expect(lastT(e).sent[0]).toEqual({ type: 'hello', v: 1, caps: { video: 1, audio: 1, input: 1 } })
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
    expect(lastPC(e).calls).toEqual(['createDataChannel', 'addTransceiver', 'addTransceiver', 'createOffer'])
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

  it('a wake while waiting at Admit pings; a pong keeps the socket and no answer redials', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    await settle()
    e.client.wake('visibility')
    const ping = lastT(e).sent.find((m) => m.type === 'ping')
    expect(ping).toEqual({ type: 'ping', n: expect.any(Number) })
    lastT(e).recv({ type: 'pong', n: ping!.n })
    await e.clock.advance(RECONNECT.pingTimeoutMs + 1)
    expect(e.transports).toHaveLength(1)

    e.client.wake('pageshow')
    expect(lastT(e).types().filter((x) => x === 'ping')).toHaveLength(2)
    await e.clock.advance(RECONNECT.pingTimeoutMs + 1)
    expect(e.transports.length).toBeGreaterThan(1)
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

describe('guest close reasons and the check', () => {
  const finals: [number, string][] = [
    [4430, 'denied'],
    [4431, 'admit_timeout'],
    [4432, 'revoked'],
    [4433, 'share_ended'],
    [4434, 'replaced'],
    [4401, 'token_invalid'],
  ]

  it.each(finals)('close %i is the final reason %s: reported once, the loop ends and nothing is redialed', async (code, reason) => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    lastT(e).dropWith(code)
    expect(e.statuses).toEqual([{ kind: 'guest_final', reason }])
    await e.clock.advance(120000)
    expect(e.transports).toHaveLength(1)
    expect(e.check).not.toHaveBeenCalled()
  })

  it('a failed handshake is classified by the guest check: a link that is gone is final, a hiccup retries', async () => {
    const e = setup()
    e.check.mockResolvedValueOnce('token')
    e.client.start()
    lastT(e).dropWith(1006)
    await settle()
    expect(e.statuses).toEqual([{ kind: 'guest_final', reason: 'token_invalid' }])
    expect(e.transports).toHaveLength(1)

    const f = setup()
    f.check.mockResolvedValue('retry')
    f.client.start()
    lastT(f).dropWith(1006)
    await settle()
    await f.clock.advance(RECONNECT.delaysMs[0])
    expect(f.statuses.map((s) => s.kind)).toEqual(['reconnecting'])
    expect(f.transports).toHaveLength(2)
  })

  it.each([
    [401, 'token'],
    [410, 'token'],
    [403, 'token'],
    [204, 'ok'],
    [500, 'retry'],
    [503, 'retry'],
  ])('the guest check answers %i as %s, at the share server check URL', async (status, verdict) => {
    const fetchMock = vi.fn(async () => new Response(null, { status }))
    vi.stubGlobal('fetch', fetchMock)
    try {
      const loc = { host: 'h', search: '?cap=t%20k', pathname: '/desktop/guest/' } as unknown as Location
      expect(await fetchGuestCheck(loc)).toBe(verdict)
      expect(fetchMock.mock.calls[0]).toEqual(['/desktop/guest/check?cap=t%20k', expect.objectContaining({ cache: 'no-store' })])
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('a network drop after the socket was up is Reconnecting, then Can\'t reach this Mac after 30 s of failed retries', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'admitted' })
    await settle()
    lastT(e).dropWith(1006)
    expect(e.statuses.map((s) => s.kind)).toEqual(['reconnecting'])
    await e.clock.advance(RECONNECT.unreachableAfterMs + 1)
    expect(e.statuses.map((s) => s.kind)).toEqual(['reconnecting', 'unreachable'])
  })

  it('a reconnect that reaches Admit again leaves Reconnecting for the wait, and never reaches unreachable', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    lastT(e).dropWith(1006)
    expect(e.statuses.map((s) => s.kind)).toEqual(['reconnecting'])
    await e.clock.advance(RECONNECT.delaysMs[0])
    lastT(e).open()
    lastT(e).recv(helloMsg)
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    await e.clock.advance(RECONNECT.unreachableAfterMs * 2)
    expect(e.statuses.map((s) => s.kind)).toEqual(['reconnecting', 'connecting'])
  })
})

describe('guest rights and lease messages', () => {
  it('takes a live rights message after admission and turns each field into a boolean', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'admission', state: 'admitted' })
    lastT(e).recv({ type: 'rights', rights: { control: true, clipboard: 'yes', surface: 1 } })
    lastT(e).recv({ type: 'rights' })
    expect(e.events.filter(([k]) => k === 'rights').slice(1)).toEqual([
      ['rights', { control: true, clipboard: false, surface: false }],
      ['rights', noRights],
    ])
  })

  it('keeps the name only for the holder, as clean text, and drops an unknown state', async () => {
    const e = setup()
    await hello(e)
    lastT(e).recv({ type: 'lease', state: 'other', name: '  Ada\u202e  \n Lovelace\u200b ' })
    lastT(e).recv({ type: 'lease', state: 'physical', name: 'Mallory' })
    lastT(e).recv({ type: 'lease', state: 'other', name: 'x'.repeat(300) })
    lastT(e).recv({ type: 'lease', state: 'other' })
    lastT(e).recv({ type: 'lease', state: 'admin', name: 'Eve' })
    lastT(e).recv({ type: 'lease', name: 'Eve' })
    const leases = e.events.filter(([k]) => k === 'lease').map(([, v]) => v)
    expect(leases).toEqual([
      { state: 'other', name: 'Ada Lovelace' },
      { state: 'physical', name: '' },
      { state: 'other', name: 'x'.repeat(128) },
      { state: 'other', name: '' },
    ])
  })

  it('an own-account client ignores admission and rights, and passes the lease on (WR-07)', async () => {
    const e = setup({ guest: false })
    e.client.start()
    lastT(e).open()
    lastT(e).recv({ type: 'hello', v: 1, viewer: 'abcd', caps: { video: 1 } })
    await settle()
    expect(lastT(e).types()).toEqual(['hello', 'offer'])
    lastT(e).recv({ type: 'admission', state: 'waiting' })
    lastT(e).recv({ type: 'rights', rights: { control: true, clipboard: true, surface: true } })
    lastT(e).recv({ type: 'lease', state: 'physical' })
    lastT(e).recv({ type: 'lease', state: 'unavailable' })
    expect(e.events).toEqual([
      ['rights', { control: false, clipboard: false, surface: false }],
      ['lease', { state: 'physical', name: '' }],
      ['lease', { state: 'unavailable', name: '' }],
    ])
  })
})

// The Go side owns internal/desktop/testdata/signal-guest-v1.jsonl; every server line must decode here.
describe('signal-guest-v1.jsonl golden lines', () => {
  const lines = readFileSync(resolve(__dirname, '../../../../internal/desktop/testdata/signal-guest-v1.jsonl'), 'utf8')
    .trim().split('\n').map((l) => JSON.parse(l) as { name: string; dir: string; wire: Record<string, unknown> })
  const named = (prefix: string) => lines.filter((l) => l.dir === 'server' && l.name.startsWith(prefix))

  it('has every kind of line the page decodes', () => {
    expect(lines.map((l) => l.name)).toEqual(expect.arrayContaining([
      'hello-guest', 'admission-waiting', 'admission-admitted', 'rights-view-only', 'rights-control-and-clipboard',
      'close-denied', 'close-admit-timeout', 'close-revoked', 'close-share-ended', 'close-token-invalid',
      'close-replaced', 'close-full', 'lease-you', 'lease-other', 'lease-owner', 'lease-physical', 'lease-unavailable', 'lease-free',
    ]))
  })

  it('decodes the hello, admission and rights lines to the events the page acts on', async () => {
    const e = setup()
    e.client.start()
    lastT(e).open()
    for (const l of [...named('hello'), ...named('admission'), ...named('rights')]) lastT(e).recv(l.wire)
    await settle()
    expect(e.events).toEqual([
      ['rights', noRights],
      ['admission', { state: 'waiting' }],
      ['admission', { state: 'admitted' }],
      ['rights', noRights],
      ['rights', { control: true, clipboard: true, surface: false }],
    ])
    expect(lastT(e).types()).toEqual(['hello', 'offer'])
  })

  it('decodes each lease line to its state, with a name only for the other holder', async () => {
    const e = setup()
    await hello(e)
    for (const l of named('lease')) lastT(e).recv(l.wire)
    expect(e.events.filter(([k]) => k === 'lease').map(([, v]) => v)).toEqual([
      { state: 'you', name: '' },
      { state: 'other', name: 'Ada' },
      { state: 'owner', name: '' },
      { state: 'physical', name: '' },
      { state: 'unavailable', name: '' },
      { state: 'free', name: '' },
    ])
  })

  it('decodes each close line by its code to the reason the wire names, and only full does not end the loop', async () => {
    for (const l of named('close')) {
      const code = l.wire.close as number
      const reason = l.wire.reason as string
      const e = setup()
      await hello(e)
      lastT(e).dropWith(code)
      const final = e.statuses.find((s) => s.kind === 'guest_final')
      // replaced is final: a retry would knock again and replace the other tab, which would retry in turn.
      if (['denied', 'admit_timeout', 'revoked', 'share_ended', 'token_invalid', 'replaced'].includes(reason)) {
        expect(final, l.name).toEqual({ kind: 'guest_final', reason })
      } else {
        expect(final, l.name).toBeUndefined()
        expect(e.statuses.map((s) => s.kind), l.name).toEqual(['reconnecting'])
      }
    }
  })
})
