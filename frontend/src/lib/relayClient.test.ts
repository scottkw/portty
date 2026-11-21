import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  MSG_INPUT,
  MSG_OUTPUT,
  MSG_PING,
  MSG_PONG,
  MSG_RESIZE,
  MSG_RESIZE2,
  RECONNECT,
  backoffDelayMs,
  encodeInputFrames,
  MAX_INPUT_PAYLOAD,
  encodeResizeFrame,
  encodeClaimFrame,
  parseServerFrame,
  RelayClient,
  type ReconnectVerdict,
  type RelayClientCallbacks,
  MSG_PREVIEW,
  PREVIEW_CHUNK_MAX,
  type PreviewFrame,
} from './relayClient'
import previewFixture from './__tests__/fixtures/preview-frames.json'

const concat = (frames: Uint8Array[]) => {
  const out: number[] = []
  for (const f of frames) out.push(...f.subarray(1))
  return new Uint8Array(out)
}

describe('encodeInputFrames (HARD-02)', () => {
  it('short input is one frame: MSG_INPUT then the UTF-8 bytes', () => {
    const frames = encodeInputFrames('hello')
    expect(frames).toHaveLength(1)
    expect(frames[0]).toBeInstanceOf(Uint8Array)
    expect(Array.from(frames[0])).toEqual([MSG_INPUT, 104, 101, 108, 108, 111])
  })

  it('empty input is no frame', () => {
    expect(encodeInputFrames('')).toEqual([])
  })

  it('100,000 ASCII characters split into 4 frames under the 32768-byte read limit, in order', () => {
    const text = Array.from({ length: 100_000 }, (_, i) => String.fromCharCode(33 + (i % 90))).join('')
    const frames = encodeInputFrames(text)
    expect(frames).toHaveLength(4)
    for (const f of frames) {
      expect(f[0]).toBe(MSG_INPUT)
      expect(f.length).toBeLessThanOrEqual(MAX_INPUT_PAYLOAD + 1)
    }
    expect(MAX_INPUT_PAYLOAD + 1).toBe(32768)
    expect(Array.from(concat(frames))).toEqual(Array.from(new TextEncoder().encode(text))) // Array.from: jsdom's TextEncoder returns another realm's Uint8Array
  })

  it('a multibyte character straddling a frame boundary still concatenates byte-for-byte', () => {
    const text = '€'.repeat(20_000) // 3 bytes each, 60,000 bytes: 32767 is not a multiple of 3
    const frames = encodeInputFrames(text)
    expect(frames).toHaveLength(2)
    expect(Array.from(concat(frames))).toEqual(Array.from(new TextEncoder().encode(text))) // Array.from: jsdom's TextEncoder returns another realm's Uint8Array
  })
})

describe('RelayClient host path sends a large paste as ordered frames (HARD-02)', () => {
  class WS {
    static OPEN = 1
    static last: WS
    binaryType = ''
    readyState = 1
    onopen: (() => void) | null = null
    onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
    onclose: ((e: { code: number; reason: string }) => void) | null = null
    onerror: (() => void) | null = null
    send = vi.fn()
    close = vi.fn()
    constructor() { WS.last = this }
  }
  beforeEach(() => { vi.stubGlobal('WebSocket', WS) })
  afterEach(() => { vi.unstubAllGlobals() })

  it('sendInput of 100,000 characters on an open socket sends 4 MSG_INPUT frames in order', () => {
    const client = new RelayClient(34115, 'sess-1', { onOutput: vi.fn() })
    const text = 'x'.repeat(99_999) + 'y'
    client.sendInput(text)
    const sent = WS.last.send.mock.calls.map((c) => c[0] as Uint8Array)
    // DEBT-05: the 4 input frames in order, then exactly one MSG_PING (the paste's ack).
    expect(sent).toHaveLength(5)
    const inputs = sent.slice(0, 4)
    expect(inputs.every((f) => f[0] === MSG_INPUT && f.length <= 32768)).toBe(true)
    expect(new TextDecoder().decode(concat(inputs))).toBe(text)
    expect(Array.from(sent[4])).toEqual([MSG_PING])
    client.close()
  })
})

describe('encodeResizeFrame', () => {
  it('produces 5-byte frame [MSG_RESIZE2, cols_hi, cols_lo, rows_hi, rows_lo]', () => {
    const result = encodeResizeFrame(120, 40)
    expect(result).toBeInstanceOf(Uint8Array)
    expect(result.length).toBe(5)
    expect(Array.from(result)).toEqual([0x11, 0, 120, 0, 40])
  })

  it('handles large col/row values', () => {
    const result = encodeResizeFrame(256, 128)
    expect(result[0]).toBe(MSG_RESIZE2) // 0x11
    expect(result[1]).toBe(1) // cols_hi = 256 >> 8 = 1
    expect(result[2]).toBe(0) // cols_lo = 256 & 0xff = 0
    expect(result[3]).toBe(0) // rows_hi = 128 >> 8 = 0
    expect(result[4]).toBe(128) // rows_lo = 128 & 0xff = 128
  })

  it('handles 80x24 standard terminal size', () => {
    const result = encodeResizeFrame(80, 24)
    expect(Array.from(result)).toEqual([0x11, 0, 80, 0, 24])
  })
})

// INPT-06: a claim is the 5-byte report plus byte 5 bit 0 (Research Pattern 3).
describe('encodeClaimFrame and sendClaim (INPT-06)', () => {
  class WS {
    static OPEN = 1
    static last: WS
    binaryType = ''
    readyState = 1
    onopen: (() => void) | null = null
    onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
    onclose: ((e: { code: number; reason: string }) => void) | null = null
    onerror: (() => void) | null = null
    send = vi.fn()
    close = vi.fn()
    constructor() { WS.last = this }
  }
  beforeEach(() => { vi.stubGlobal('WebSocket', WS) })
  afterEach(() => { vi.unstubAllGlobals() })
  const sent = () => WS.last.send.mock.calls.map((c) => Array.from(c[0] as Uint8Array))

  it('encodeClaimFrame is [MSG_RESIZE2, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]', () => {
    expect(Array.from(encodeClaimFrame(256, 128))).toEqual([0x11, 1, 0, 0, 128, 0x01])
  })

  it('sendClaim(120, 30) on an open socket sends exactly [0x11, 0, 120, 0, 30, 0x01]; sendResize still sends 5 bytes', () => {
    const client = new RelayClient(34115, 'sess-1', { onOutput: vi.fn() })
    client.sendClaim(120, 30)
    client.sendResize(100, 30)
    expect(sent()).toEqual([[0x11, 0, 120, 0, 30, 0x01], [0x11, 0, 100, 0, 30]])
    client.close()
  })

  it('sendClaim on a closed socket sends nothing', () => {
    const client = new RelayClient(34115, 'sess-1', { onOutput: vi.fn() })
    WS.last.readyState = 3
    client.sendClaim(120, 30)
    expect(sent()).toEqual([])
    client.close()
  })
})

describe('parseServerFrame', () => {
  it('parses output frame (0x01) returning type output and payload', () => {
    const data = new Uint8Array([MSG_OUTPUT, 72, 101, 108, 108, 111])
    const result = parseServerFrame(data)
    expect(result.type).toBe('output')
    if (result.type === 'output') {
      expect(result.payload).toBeInstanceOf(Uint8Array)
      expect(Array.from(result.payload)).toEqual([72, 101, 108, 108, 111])
    }
  })

  it('parses resize frame (0x02) returning cols and rows', () => {
    const data = new Uint8Array([0x02, 0, 80, 0, 24])
    const result = parseServerFrame(data)
    expect(result.type).toBe('resize')
    if (result.type === 'resize') {
      expect(result.cols).toBe(80)
      expect(result.rows).toBe(24)
    }
  })

  it('returns unknown for empty frame', () => {
    const data = new Uint8Array([])
    const result = parseServerFrame(data)
    expect(result.type).toBe('unknown')
  })

  it('returns unknown for unrecognized type byte', () => {
    const data = new Uint8Array([0xff, 1, 2, 3])
    const result = parseServerFrame(data)
    expect(result.type).toBe('unknown')
  })

  it('parses output frame with no payload bytes', () => {
    const data = new Uint8Array([0x01])
    const result = parseServerFrame(data)
    expect(result.type).toBe('output')
    if (result.type === 'output') {
      expect(result.payload.length).toBe(0)
    }
  })
})

describe('RelayClient URL construction', () => {
  let capturedUrl: string | null = null

  // Stub global WebSocket so constructing RelayClient does not open a real socket
  // and we can capture the URL passed to the constructor.
  beforeEach(() => {
    capturedUrl = null
    // Minimal WebSocket stub: record the constructed URL, expose a no-op API
    const MockWebSocket = vi.fn(function (this: WebSocket, url: string) {
      capturedUrl = url
      // Satisfy the RelayClient constructor's property access
      this.binaryType = 'arraybuffer'
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ;(this as any).readyState = 0 // CONNECTING
    }) as unknown as typeof WebSocket
    // Assign prototype properties so the shape looks like a real WebSocket
    Object.assign(MockWebSocket.prototype, {
      close: vi.fn(),
      send: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  // FE-URL-01a: local-direct path (no opts — preserves every existing caller)
  it('FE-URL-01a: builds the local-direct path when no opts are passed', () => {
    const port = 51234
    const sid = 'test-session-abc'
    new RelayClient(port, sid, { onOutput: () => {} })
    expect(capturedUrl).toBe(`ws://127.0.0.1:${port}/sessions/${sid}/ws`)
    // The cap token is absent from the URL (T-134-07-01)
    expect(capturedUrl).not.toContain('cap')
  })

  // FE-URL-01b: remote path (remote: true) — daemon-proxy URL
  it('FE-URL-01b: builds the daemon-proxy path when opts.remote is true', () => {
    const port = 51234
    const sid = 'remote-session-xyz'
    new RelayClient(port, sid, { onOutput: () => {} }, { remote: true })
    expect(capturedUrl).toBe(`ws://127.0.0.1:${port}/api/relay/remote/${sid}/ws`)
    // The cap token is absent from the URL (T-134-07-01)
    expect(capturedUrl).not.toContain('cap')
  })

  // FE-URL-01a variant: explicit remote: false is treated as local-direct
  it('FE-URL-01a variant: explicit remote: false builds the local-direct path', () => {
    const port = 51234
    const sid = 'local-session-def'
    new RelayClient(port, sid, { onOutput: () => {} }, { remote: false })
    expect(capturedUrl).toBe(`ws://127.0.0.1:${port}/sessions/${sid}/ws`)
  })
})

// ─── Portty 02-07 (CHAT-01): retired 0x30-0x37 bytes parse as unknown ─────────

function retiredFrame(byte: number): Uint8Array {
  const body = new TextEncoder().encode(JSON.stringify({ personKey: 'tn:web', alias: 'guest', content: 'hi', typing: true }))
  return new Uint8Array([byte, ...body])
}

describe('parseServerFrame — retired chat/presence bytes (CHAT-01)', () => {
  for (let byte = 0x30; byte <= 0x37; byte++) {
    it(`0x${byte.toString(16)} with a JSON payload parses to unknown, and a following output frame still parses`, () => {
      expect(parseServerFrame(retiredFrame(byte))).toEqual({ type: 'unknown' })
      const out = parseServerFrame(new Uint8Array([MSG_OUTPUT, 0x41]))
      expect(out.type).toBe('output')
      if (out.type === 'output') expect(Array.from(out.payload)).toEqual([0x41])
    })
  }
})

describe('RelayClient — an old peer sending retired frames does not stop output (CHAT-01)', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let lastMockWS: any

  beforeEach(() => {
    lastMockWS = null
    const MockWS = vi.fn(function (this: Record<string, unknown>) {
      this.binaryType = 'arraybuffer'
      this.readyState = 1 // OPEN
      this.onopen = null
      this.onmessage = null
      this.onclose = null
      this.onerror = null
      this.send = vi.fn()
      this.close = vi.fn()
      // eslint-disable-next-line @typescript-eslint/no-this-alias
      lastMockWS = this
    }) as unknown as typeof WebSocket
    vi.stubGlobal('WebSocket', MockWS)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('drops 0x30-0x37 frames silently and still delivers the next output frame', () => {
    const onOutput = vi.fn()
    const client = new RelayClient(34115, 'sess-1', { onOutput })
    for (let byte = 0x30; byte <= 0x37; byte++) {
      expect(() => lastMockWS.onmessage?.({ data: retiredFrame(byte).buffer as ArrayBuffer })).not.toThrow()
    }
    expect(onOutput).not.toHaveBeenCalled()
    lastMockWS.onmessage?.({ data: new Uint8Array([MSG_OUTPUT, 0x41]).buffer as ArrayBuffer })
    expect(onOutput).toHaveBeenCalledTimes(1)
    expect(Array.from(onOutput.mock.calls[0][0] as Uint8Array)).toEqual([0x41])
    client.close()
  })
})

describe('RelayClient Phase 157 — 0x02 MsgResize dispatch', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let lastMockWS: any

  beforeEach(() => {
    lastMockWS = null
    const MockWS = vi.fn(function (this: Record<string, unknown>) {
      this.binaryType = 'arraybuffer'
      this.readyState = 1 // OPEN
      this.onopen = null
      this.onmessage = null
      this.onclose = null
      this.onerror = null
      this.send = vi.fn()
      this.close = vi.fn()
      // eslint-disable-next-line @typescript-eslint/no-this-alias
      lastMockWS = this
    }) as unknown as typeof WebSocket
    vi.stubGlobal('WebSocket', MockWS)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('MSG_RESIZE constant is 0x02', () => {
    expect(MSG_RESIZE).toBe(0x02)
  })

  it('fires onResize(cols, rows) when a 5-byte 0x02 frame arrives', () => {
    const onResizeFn = vi.fn()
    const callbacks: RelayClientCallbacks = { onOutput: vi.fn(), onResize: onResizeFn }
    const client = new RelayClient(34115, 'sess-resize', callbacks)

    // Build a 0x02 frame: [0x02, 0, 80, 0, 24] → cols=80, rows=24
    const frame = new Uint8Array([0x02, 0, 80, 0, 24])
    const buf = frame.buffer as ArrayBuffer
    lastMockWS.onmessage?.({ data: buf })

    expect(onResizeFn).toHaveBeenCalledTimes(1)
    expect(onResizeFn).toHaveBeenCalledWith(80, 24)

    client.close()
  })

  it('fires onResize with large col/row values (big-endian decode)', () => {
    const onResizeFn = vi.fn()
    const callbacks: RelayClientCallbacks = { onOutput: vi.fn(), onResize: onResizeFn }
    const client = new RelayClient(34115, 'sess-big', callbacks)

    // cols=256 (0x01, 0x00), rows=128 (0x00, 0x80)
    const frame = new Uint8Array([0x02, 0x01, 0x00, 0x00, 0x80])
    lastMockWS.onmessage?.({ data: frame.buffer as ArrayBuffer })

    expect(onResizeFn).toHaveBeenCalledWith(256, 128)

    client.close()
  })

  it('does not fire onResize when a short (< 5 byte) 0x02 frame arrives', () => {
    const onResizeFn = vi.fn()
    const callbacks: RelayClientCallbacks = { onOutput: vi.fn(), onResize: onResizeFn }
    const client = new RelayClient(34115, 'sess-short', callbacks)

    // Truncated frame — parseServerFrame returns { type: 'unknown' }
    const frame = new Uint8Array([0x02, 0, 80, 0]) // only 4 bytes
    lastMockWS.onmessage?.({ data: frame.buffer as ArrayBuffer })

    expect(onResizeFn).not.toHaveBeenCalled()

    client.close()
  })

  it('does not throw when onResize is omitted (host-path backward compat)', () => {
    // Host path: only onOutput provided — no onResize
    const callbacks: RelayClientCallbacks = { onOutput: vi.fn() }
    const client = new RelayClient(34115, 'sess-host', callbacks)

    const frame = new Uint8Array([0x02, 0, 80, 0, 24])
    expect(() => {
      lastMockWS.onmessage?.({ data: frame.buffer as ArrayBuffer })
    }).not.toThrow()

    client.close()
  })
})

// ─── Phase 175-06: onClose(code, reason) — BUG-02 (#125) ─────────────────────

describe('RelayClient onClose callback dispatch (Phase 175-06 / BUG-02)', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let lastMockWS: any

  beforeEach(() => {
    lastMockWS = null
    const MockWS = vi.fn(function (this: Record<string, unknown>) {
      this.binaryType = 'arraybuffer'
      this.readyState = 1 // OPEN
      this.onopen = null
      this.onmessage = null
      this.onclose = null
      this.onerror = null
      this.send = vi.fn()
      this.close = vi.fn()
      // eslint-disable-next-line @typescript-eslint/no-this-alias
      lastMockWS = this
    }) as unknown as typeof WebSocket
    ;(MockWS as any).OPEN = 1
    vi.stubGlobal('WebSocket', MockWS)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('fires onClose(code, reason) exactly once with the CloseEvent code/reason on ws close', () => {
    const onCloseFn = vi.fn()
    const client = new RelayClient(34115, 'sess-close', { onOutput: vi.fn(), onClose: onCloseFn })

    lastMockWS.onclose?.({ code: 1000, reason: 'session ended' } as CloseEvent)

    expect(onCloseFn).toHaveBeenCalledTimes(1)
    expect(onCloseFn).toHaveBeenCalledWith(1000, 'session ended')

    client.close()
  })

  it('does not throw when onClose is omitted and the ws closes', () => {
    const callbacks: RelayClientCallbacks = { onOutput: vi.fn() }
    const client = new RelayClient(34115, 'sess-noclose', callbacks)

    expect(() => {
      lastMockWS.onclose?.({ code: 1000, reason: 'session ended' } as CloseEvent)
    }).not.toThrow()

    client.close()
  })

  it('still clears the ping interval on close (pre-existing behavior preserved)', () => {
    vi.useFakeTimers()
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval')
    const client = new RelayClient(34115, 'sess-clearping', { onOutput: vi.fn() })

    lastMockWS.onopen?.()
    lastMockWS.onclose?.({ code: 1000, reason: 'session ended' } as CloseEvent)

    expect(clearIntervalSpy).toHaveBeenCalled()

    client.close()
    clearIntervalSpy.mockRestore()
    vi.useRealTimers()
  })
})

// ─── Phase 5 (SHAR-07): reconnect mode ───────────────────────────────────────

describe('parseServerFrame — pong and meta (SHAR-07)', () => {
  it('MSG_PONG is 0x04 (pinned with relay.MsgPong)', () => {
    expect(MSG_PONG).toBe(0x04)
  })
  it('0x04 parses to pong and 0x20 to meta with its payload', () => {
    expect(parseServerFrame(new Uint8Array([0x04]))).toEqual({ type: 'pong' })
    const m = parseServerFrame(new Uint8Array([0x20, 0x7b, 0x7d]))
    expect(m.type).toBe('meta')
    if (m.type === 'meta') expect(Array.from(m.payload)).toEqual([0x7b, 0x7d])
  })
})

describe('backoffDelayMs', () => {
  it('doubles from 1 s to a 30 s cap at rand 0.5', () => {
    expect([0, 1, 2, 3, 4, 5, 6].map((a) => backoffDelayMs(a, 0.5))).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000])
  })
  it('jitter stays within ±20 % and never exceeds 30 s', () => {
    for (let a = 0; a <= 8; a++) {
      const base = Math.min(30_000, 1000 * 2 ** a)
      for (const r of [0, 1]) {
        const d = backoffDelayMs(a, r)
        expect(d).toBeGreaterThanOrEqual(base * 0.8)
        expect(d).toBeLessThanOrEqual(Math.min(30_000, base * 1.2))
      }
    }
  })
})

describe('RelayClient reconnect mode (SHAR-07)', () => {
  class WS {
    static OPEN = 1
    static instances: WS[] = []
    static throwNext = 0 // constructions left that throw like a malformed URL
    binaryType = ''
    readyState = 0
    onopen: (() => void) | null = null
    onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
    onclose: ((e: { code: number; reason: string }) => void) | null = null
    onerror: (() => void) | null = null
    send = vi.fn()
    close = vi.fn()
    constructor(public url: string) {
      if (WS.throwNext > 0) {
        WS.throwNext--
        throw new SyntaxError(`Failed to construct 'WebSocket': The URL '${url}' is invalid.`)
      }
      WS.instances.push(this)
    }
  }
  let visibility = 'visible'
  const enc = new TextEncoder()
  const meta = (m: object) => new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))])
  const out = (t: string) => new Uint8Array([0x01, ...enc.encode(t)])
  const pong = new Uint8Array([MSG_PONG])
  const ws = (i: number) => WS.instances[i]
  const count = () => WS.instances.length
  const send = (i: number, f: Uint8Array) => ws(i).onmessage?.({ data: f.buffer as ArrayBuffer })
  // upgrade: the WebSocket (for a peer, the loopback proxy) accepted; nothing from the host yet.
  const upgrade = (i: number) => { ws(i).readyState = 1; ws(i).onopen?.() }
  // joined: upgraded and the host's first frame (its grid, sent at once on join) arrived (WR-03).
  const grid = new Uint8Array([MSG_RESIZE, 0, 80, 0, 24])
  const joined = (i: number) => { upgrade(i); send(i, grid) }
  // open: joined and the (empty) replay is done: its offset meta arrived (WR-01 it3).
  const open = (i: number) => { joined(i); send(i, meta({ replay: 'full' })); send(i, meta({ offset: 0 })) }
  const inputsSent = (i: number) => ws(i).send.mock.calls.filter((c) => (c[0] as Uint8Array)[0] === MSG_INPUT).length
  const drop = (i: number, code = 1006, reason = '') => { ws(i).readyState = 3; ws(i).onclose?.({ code, reason }) }
  const advance = (ms: number) => vi.advanceTimersByTimeAsync(ms)
  const pingsSent = (i: number) => ws(i).send.mock.calls.filter((c) => (c[0] as Uint8Array)[0] === MSG_PING).length

  const mocks = () => ({
    onOutput: vi.fn<(d: Uint8Array) => void>(),
    onClose: vi.fn<(code?: number, reason?: string) => void>(),
    onStateChange: vi.fn<(s: string) => void>(),
    onReset: vi.fn<() => void>(),
  })
  let cbs: ReturnType<typeof mocks>
  let client: RelayClient | null

  function make(beforeReconnect?: () => Promise<ReconnectVerdict>, wsURL?: string) {
    client = new RelayClient(1234, 's', cbs, { remote: true, wsURL, reconnect: { beforeReconnect } })
    return client
  }

  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    WS.instances = []
    WS.throwNext = 0
    vi.stubGlobal('WebSocket', WS)
    visibility = 'visible'
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
    cbs = mocks()
    client = null
  })

  afterEach(() => {
    client?.close()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (document as any).visibilityState
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  async function expectRedialAfter(ms: number) {
    const n = count()
    await advance(ms - 1)
    expect(count(), `no redial before ${ms} ms`).toBe(n)
    await advance(1)
    expect(count(), `redial at ${ms} ms`).toBe(n + 1)
  }

  it('a socket open 30 s or more restarts the schedule at 1 s; one open 5 s continues it', async () => {
    make()
    drop(0) // failed first connect: retried like a drop
    await expectRedialAfter(1000)
    drop(1)
    await expectRedialAfter(2000)
    open(2)
    await advance(5_000)
    drop(2)
    await expectRedialAfter(4000)
    open(3)
    await advance(RECONNECT.healthyResetMs)
    drop(3)
    await expectRedialAfter(1000)
  })

  for (const [code, reason] of [[1006, ''], [1008, 'too slow'], [1013, 'remote unreachable'], [1011, 'bad base url'], [1001, '']] as const) {
    it(`close ${code} '${reason}' schedules a reconnect`, async () => {
      make()
      open(0)
      drop(0, code, reason)
      expect(cbs.onStateChange).toHaveBeenLastCalledWith('reconnecting')
      await expectRedialAfter(1000)
      expect(cbs.onClose).not.toHaveBeenCalled()
    })
  }

  for (const reason of ['share stopped', 'session ended']) {
    it(`1000 '${reason}' is final`, async () => {
      make()
      open(0)
      drop(0, 1000, reason)
      expect(cbs.onClose).toHaveBeenCalledWith(1000, reason)
      expect(cbs.onStateChange).toHaveBeenLastCalledWith('stopped')
      await advance(120_000)
      expect(count()).toBe(1)
    })
  }

  it('a user close() schedules nothing and fires no callback', async () => {
    make()
    open(0)
    client!.close()
    client = null
    expect(ws(0).close).toHaveBeenCalled()
    expect(ws(0).onclose).toBeNull()
    await advance(120_000)
    expect(count()).toBe(1)
    expect(cbs.onClose).not.toHaveBeenCalled()
  })

  it('input is dropped, never queued, while the socket is down', async () => {
    make()
    open(0)
    drop(0)
    client!.sendInput('rm -rf ~\r')
    await advance(1000)
    open(1)
    expect(ws(0).send.mock.calls.some((c) => (c[0] as Uint8Array)[0] === MSG_INPUT)).toBe(false)
    expect(ws(1).send.mock.calls.some((c) => (c[0] as Uint8Array)[0] === MSG_INPUT)).toBe(false)
  })

  it('a large paste is sent as ordered frames once open, and nothing while reconnecting (HARD-02)', async () => {
    make()
    open(0)
    const text = 'a'.repeat(50_000) + 'b'.repeat(50_000)
    client!.sendInput(text)
    const sent = ws(0).send.mock.calls.map((c) => c[0] as Uint8Array).filter((f) => f[0] === MSG_INPUT)
    expect(sent).toHaveLength(4)
    expect(new TextDecoder().decode(concat(sent))).toBe(text)
    drop(0)
    client!.sendInput(text)
    await advance(1000)
    open(1)
    expect(inputsSent(0)).toBe(4)
    expect(inputsSent(1)).toBe(0)
  })

  it('an upgraded socket is not open until the host sends a frame: input is dropped (WR-03)', async () => {
    make()
    upgrade(0) // the loopback proxy accepted and is still dialing the peer
    client!.sendInput('ls\r')
    expect(cbs.onStateChange).not.toHaveBeenCalledWith('open')
    expect(ws(0).send.mock.calls.some((c) => (c[0] as Uint8Array)[0] === MSG_INPUT)).toBe(false)
    drop(0, 1013, 'remote unreachable')
    await advance(1000)
    upgrade(1)
    expect(cbs.onStateChange).toHaveBeenLastCalledWith('reconnecting') // banner stays up
    send(1, grid)
    send(1, meta({ replay: 'full' }))
    send(1, meta({ offset: 0 }))
    expect(cbs.onStateChange).toHaveBeenLastCalledWith('open')
    client!.sendInput('ls\r')
    expect(ws(1).send.mock.calls.some((c) => (c[0] as Uint8Array)[0] === MSG_INPUT)).toBe(true)
  })

  it('sendClaim is dropped while reconnecting or not yet open, and sent once open (INPT-06)', async () => {
    const claims = (i: number) => ws(i).send.mock.calls.filter((c) => (c[0] as Uint8Array).length === 6).length
    make()
    joined(0) // the grid arrived, the replay has not ended: not open
    client!.sendClaim(120, 30)
    expect(claims(0)).toBe(0)
    send(0, meta({ replay: 'full' }))
    send(0, meta({ offset: 0 }))
    client!.sendClaim(120, 30)
    expect(claims(0)).toBe(1)
    drop(0)
    client!.sendClaim(120, 30)
    await advance(1000)
    joined(1)
    client!.sendClaim(120, 30)
    expect(claims(0)).toBe(1)
    expect(claims(1)).toBe(0)
  })

  it('input is dropped until the replay ends: open comes at the offset meta, not the grid (WR-01 it3)', async () => {
    make()
    joined(0)
    send(0, meta({ replay: 'full' }))
    send(0, out('history'))
    client!.sendInput('y\r') // typed at a half-drawn screen
    expect(cbs.onStateChange).not.toHaveBeenCalledWith('open')
    expect(inputsSent(0)).toBe(0)
    await advance(RECONNECT.connectMs * 2) // a slow replay: the connect timer stopped at the grid
    expect(ws(0).close).not.toHaveBeenCalled()
    send(0, meta({ offset: 100 }))
    expect(cbs.onStateChange).toHaveBeenLastCalledWith('open')
    client!.sendInput('y\r')
    expect(inputsSent(0)).toBe(1)
  })

  it('an older host that sends no replay meta opens at its first output (WR-01 it3)', async () => {
    make()
    joined(0)
    expect(cbs.onStateChange).not.toHaveBeenCalledWith('open')
    send(0, out('$ '))
    expect(cbs.onStateChange).toHaveBeenLastCalledWith('open')
    client!.sendInput('ls\r')
    expect(inputsSent(0)).toBe(1)
  })

  it('a socket that goes silent mid-replay is probed and abandoned like an open one (WR-01 it3)', async () => {
    make()
    open(0)
    send(0, pong)
    drop(0)
    await advance(1000)
    joined(1)
    send(1, meta({ replay: 'full' }))
    send(1, out('part of the history'))
    await advance(RECONNECT.deadMs + RECONNECT.probeGraceMs + 2000)
    expect(ws(1).close).toHaveBeenCalled()
    expect(count()).toBe(3)
  })

  it('a socket with no host frame within connectMs is dropped and retried on the backoff, never stopped (WR-06)', async () => {
    make()
    await advance(RECONNECT.connectMs - 1)
    expect(ws(0).close).not.toHaveBeenCalled()
    await advance(1)
    expect(ws(0).close).toHaveBeenCalled()
    expect(ws(0).onclose).toBeNull()
    expect(cbs.onStateChange).toHaveBeenLastCalledWith('reconnecting')
    await expectRedialAfter(1000)
    upgrade(1) // a handshake that completes but the host never answers
    await advance(RECONNECT.connectMs)
    expect(ws(1).close).toHaveBeenCalled()
    await expectRedialAfter(2000)
    expect(cbs.onClose).not.toHaveBeenCalled()
  })

  it('a socket that opens in time is never dropped by the connect timer (WR-06)', async () => {
    make()
    open(0)
    await advance(RECONNECT.connectMs * 3)
    expect(ws(0).close).not.toHaveBeenCalled()
    drop(0) // a close cancels the timer: the pending backoff is not restarted
    await expectRedialAfter(1000)
  })

  it('a socket the proxy accepts and then refuses keeps the resume offset (WR-02)', async () => {
    make()
    open(0)
    send(0, meta({ replay: 'full' }))
    send(0, meta({ offset: 100 }))
    drop(0)
    await advance(1000)
    upgrade(1)
    drop(1, 1013, 'remote unreachable')
    await advance(2000)
    expect(ws(2).url).toContain('resume=100')
    joined(2) // only the grid arrives before the drop: nothing was rendered, the offset holds
    drop(2)
    await advance(4000)
    expect(ws(3).url).toContain('resume=100')
  })

  it('beforeReconnect retry schedules again; a rejected promise counts as retry', async () => {
    const hook = vi.fn<() => Promise<ReconnectVerdict>>()
      .mockResolvedValueOnce({ kind: 'retry' })
      .mockRejectedValueOnce(new Error('binding threw'))
      .mockResolvedValue({ kind: 'ok' })
    make(hook)
    open(0)
    drop(0)
    await advance(1000)
    expect(hook).toHaveBeenCalledTimes(1)
    expect(count()).toBe(1)
    await advance(2000)
    expect(hook).toHaveBeenCalledTimes(2)
    expect(count()).toBe(1)
    await advance(4000)
    expect(hook).toHaveBeenCalledTimes(3)
    expect(count()).toBe(2)
  })

  it("beforeReconnect stop share-stopped fires onClose(1000, 'share stopped') and state stopped", async () => {
    make(() => Promise.resolve({ kind: 'stop', reason: 'share-stopped' }))
    open(0)
    drop(0)
    await advance(1000)
    expect(cbs.onClose).toHaveBeenCalledWith(1000, 'share stopped')
    expect(cbs.onStateChange).toHaveBeenLastCalledWith('stopped')
    await advance(120_000)
    expect(count()).toBe(1)
  })

  it('beforeReconnect ok with port 4321 dials the new loopback port', async () => {
    make(() => Promise.resolve({ kind: 'ok', port: 4321 }))
    joined(0)
    drop(0)
    await advance(1000)
    expect(ws(1).url).toBe('ws://127.0.0.1:4321/api/relay/remote/s/ws')
  })

  it('a wsURL with ?cap= gets &resume= after a counted offset', async () => {
    make(undefined, 'wss://office.ts.net/sessions/s/ws?cap=abc')
    open(0)
    send(0, meta({ replay: 'full' }))
    send(0, meta({ offset: 10 }))
    send(0, out('xy'))
    drop(0)
    await advance(1000)
    expect(ws(1).url).toBe('wss://office.ts.net/sessions/s/ws?cap=abc&resume=13')
  })

  describe('watchdog', () => {
    it('before any pong, 60 s of silence keeps the socket (older host)', async () => {
      make()
      open(0)
      await advance(60_000)
      expect(ws(0).close).not.toHaveBeenCalled()
      expect(count()).toBe(1)
    })

    it('after a pong, 40 s of silence sends a probe and 5 s more abandons the socket and redials', async () => {
      make()
      open(0)
      send(0, pong)
      await advance(RECONNECT.deadMs + 1000)
      const probes = pingsSent(0)
      expect(probes).toBeGreaterThan(0)
      expect(ws(0).close).not.toHaveBeenCalled()
      await advance(RECONNECT.probeGraceMs + 1000)
      expect(ws(0).close).toHaveBeenCalled()
      expect(ws(0).onclose).toBeNull()
      expect(ws(0).onmessage).toBeNull()
      expect(cbs.onStateChange).toHaveBeenCalledWith('reconnecting')
      expect(count()).toBe(2)
    })

    it('a frame within the grace cancels the probe', async () => {
      make()
      open(0)
      send(0, pong)
      await advance(RECONNECT.deadMs + 1000)
      send(0, pong)
      await advance(RECONNECT.probeGraceMs + 2000)
      expect(ws(0).close).not.toHaveBeenCalled()
      expect(count()).toBe(1)
    })

    it('a hidden document never triggers it', async () => {
      make()
      open(0)
      send(0, pong)
      visibility = 'hidden'
      await advance(5 * 60_000)
      expect(ws(0).close).not.toHaveBeenCalled()
      expect(count()).toBe(1)
    })

    it('a late onclose or onmessage from an abandoned socket changes nothing', async () => {
      make()
      open(0)
      send(0, pong)
      const lateClose = ws(0).onclose!
      const lateMessage = ws(0).onmessage!
      await advance(RECONNECT.deadMs + RECONNECT.probeGraceMs + 2000)
      expect(count()).toBe(2)
      const states = cbs.onStateChange.mock.calls.length
      lateClose({ code: 1000, reason: 'share stopped' })
      lateMessage({ data: out('stale').buffer as ArrayBuffer })
      await advance(RECONNECT.connectMs - 1000) // past any backoff a late close could start, short of the connect timer
      expect(count()).toBe(2)
      expect(cbs.onClose).not.toHaveBeenCalled()
      expect(cbs.onOutput).not.toHaveBeenCalled()
      expect(cbs.onStateChange.mock.calls.length).toBe(states)
    })
  })

  describe('wake events', () => {
    it("'online' while reconnecting dials at once", async () => {
      make()
      open(0)
      drop(0)
      await advance(500)
      expect(count()).toBe(1)
      window.dispatchEvent(new Event('online'))
      await advance(0)
      expect(count()).toBe(2)
    })

    it("'visibilitychange' to visible while reconnecting dials at once", async () => {
      make()
      open(0)
      drop(0)
      await advance(500)
      document.dispatchEvent(new Event('visibilitychange'))
      await advance(0)
      expect(count()).toBe(2)
    })

    it("'online' while open sends a probe, and no answer within the grace abandons the socket", async () => {
      make()
      open(0)
      send(0, pong)
      const before = pingsSent(0)
      window.dispatchEvent(new Event('online'))
      expect(pingsSent(0)).toBe(before + 1)
      await advance(RECONNECT.probeGraceMs + 1000)
      expect(ws(0).close).toHaveBeenCalled()
      expect(count()).toBe(2)
    })
  })

  // 05-13: the Go wake/network detector reaches every reconnect-mode client as a window event.
  describe("system wake ('portty:wake')", () => {
    const wake = (reason = 'wake') => window.dispatchEvent(new CustomEvent('portty:wake', { detail: reason }))

    it('while reconnecting with a pending backoff it dials at once and restarts the backoff', async () => {
      make()
      drop(0)
      await advance(1000) // attempt 1 dialed
      drop(1) // next wait is the second step (2 s)
      await advance(500)
      expect(count()).toBe(2)
      wake()
      await advance(0)
      expect(count()).toBe(3)
      drop(2)
      await expectRedialAfter(1000) // first step again
    })

    it('a hidden window still redials (it is ready when shown)', async () => {
      make()
      open(0)
      drop(0)
      visibility = 'hidden'
      await advance(500)
      wake('network')
      await advance(0)
      expect(count()).toBe(2)
    })

    it('while open after a pong it sends one probe, and no answer within the grace abandons the socket', async () => {
      make()
      open(0)
      send(0, pong)
      const before = pingsSent(0)
      wake()
      expect(pingsSent(0)).toBe(before + 1)
      await advance(RECONNECT.probeGraceMs + 1000)
      expect(ws(0).close).toHaveBeenCalled()
      expect(count()).toBe(2)
    })

    it('two events 100 ms apart cause one redial', async () => {
      make()
      open(0)
      drop(0)
      await advance(200)
      wake()
      await advance(0)
      expect(count()).toBe(2)
      drop(1) // a new backoff wait is pending
      await advance(100)
      wake()
      await advance(0)
      expect(count(), 'the second event within wakeMinGapMs is ignored').toBe(2)
      await advance(1000)
      expect(count()).toBe(3)
    })

    it('a host-path client (no reconnect) ignores it', () => {
      client = new RelayClient(1234, 's', cbs, { remote: true })
      open(0)
      wake()
      expect(pingsSent(0)).toBe(0)
      expect(count()).toBe(1)
    })

    it('close() removes the listener', async () => {
      make()
      open(0)
      send(0, pong)
      client!.close()
      client = null
      wake()
      await advance(RECONNECT.probeGraceMs + 1000)
      expect(pingsSent(0)).toBe(0)
      expect(count()).toBe(1)
    })
  })

  describe('timer-gap wake (browser views)', () => {
    // A watchdog tick that lands long after the previous one: the machine slept.
    const sleep = async (ms: number) => { vi.setSystemTime(Date.now() + ms); await advance(1000) }
    const wake = () => window.dispatchEvent(new CustomEvent('portty:wake', { detail: 'wake' }))

    it('while reconnecting it dials at the tick, and a portty:wake right after does nothing extra', async () => {
      make()
      drop(0)
      await advance(1000)
      drop(1) // next wait is 2 s
      await sleep(60_000)
      expect(count(), 'dialed at the tick after the gap').toBe(3)
      drop(2) // backoff restarted: a 1 s wait is pending
      wake()
      await advance(0)
      expect(count(), 'debounced across sources').toBe(3)
      await advance(1000)
      expect(count()).toBe(4)
    })

    it('while open it sends a probe after a gap shorter than the 40 s silence limit', async () => {
      make()
      open(0)
      send(0, pong)
      const before = pingsSent(0)
      await sleep(10_000)
      expect(pingsSent(0)).toBe(before + 1)
    })
  })

  describe('WebSocket constructor throws (malformed URL)', () => {
    it('on a redial: stays reconnecting and dials again after the backoff', async () => {
      make()
      open(0)
      drop(0)
      WS.throwNext = 1
      await advance(1000) // this redial throws
      expect(count()).toBe(1)
      expect(cbs.onStateChange).toHaveBeenLastCalledWith('reconnecting')
      expect(cbs.onClose).not.toHaveBeenCalled()
      await expectRedialAfter(2000)
    })

    it('on the first dial: no throw out of the constructor, reconnecting, then a dial after 1 s', async () => {
      WS.throwNext = 1
      expect(() => make()).not.toThrow()
      expect(cbs.onStateChange).toHaveBeenLastCalledWith('reconnecting')
      await expectRedialAfter(1000)
    })

    it('without reconnect: onClose(1006) once on a later tick, and no method throws', async () => {
      WS.throwNext = 1
      expect(() => { client = new RelayClient(1234, 's', cbs, { remote: true }) }).not.toThrow()
      expect(cbs.onClose).not.toHaveBeenCalled()
      await advance(0)
      expect(cbs.onClose).toHaveBeenCalledTimes(1)
      expect(cbs.onClose).toHaveBeenCalledWith(1006, '')
      expect(() => { client!.sendInput('x'); client!.sendResize(80, 24); client!.close() }).not.toThrow()
      client = null
    })

    it('logs a fixed message without the URL', async () => {
      const spies = [vi.spyOn(console, 'error'), vi.spyOn(console, 'warn'), vi.spyOn(console, 'log')]
      spies.forEach((s) => s.mockImplementation(() => {}))
      WS.throwNext = 1
      make(undefined, 'wss://office.example.ts.net/ws?cap=SECRET123')
      const logged = spies.flatMap((s) => s.mock.calls).map((args) => args.map(String).join(' ')).join('\n')
      expect(logged).toContain('[RelayClient] could not open a WebSocket')
      expect(logged).not.toMatch(/wss?:\/\/|cap=/)
    })
  })

  describe('stream position', () => {
    it('output before any replay meta on a reconnect (older host) resets once', async () => {
      make()
      joined(0)
      send(0, out('a'))
      expect(cbs.onReset).not.toHaveBeenCalled()
      drop(0)
      await advance(1000)
      joined(1)
      send(1, out('b'))
      send(1, out('c'))
      expect(cbs.onReset).toHaveBeenCalledTimes(1)
    })

    it('a malformed meta frame makes the next redial carry no resume', async () => {
      make()
      open(0)
      send(0, meta({ offset: 100 }))
      send(0, new Uint8Array([0x20, ...enc.encode('{bad')]))
      drop(0)
      await advance(1000)
      expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s/ws')
    })

    it('a socket that dies before its offset meta makes the next redial ask for a full replay', async () => {
      make()
      open(0)
      send(0, meta({ offset: 100 }))
      drop(0)
      await advance(1000)
      expect(ws(1).url).toContain('resume=100')
      joined(1)
      send(1, meta({ replay: 'delta' }))
      send(1, out('partial'))
      drop(1)
      await advance(2000)
      expect(ws(2).url).not.toContain('resume=')
    })
  })

  // HARD-05: a restored peer-tab snapshot dials with the offset it shows.
  describe('resume option (HARD-05)', () => {
    const makeResume = (resume: number) => {
      client = new RelayClient(1234, 's', cbs, { remote: true, resume, reconnect: {} })
      return client
    }

    it('resume: 42 dials resume=42 and resumePoint() is 42 before any frame', () => {
      const c = makeResume(42)
      expect(ws(0).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s/ws?resume=42')
      expect(c.resumePoint()).toBe(42)
    })

    it('a delta answer: resumePoint() is null until the offset meta, then counts output; no reset', () => {
      const c = makeResume(42)
      joined(0)
      send(0, meta({ replay: 'delta' }))
      expect(c.resumePoint()).toBeNull()
      send(0, meta({ offset: 50 }))
      expect(c.resumePoint()).toBe(50)
      send(0, out('abc'))
      expect(c.resumePoint()).toBe(54)
      expect(cbs.onReset).not.toHaveBeenCalled()
    })

    it('a full answer resets the restored terminal once', () => {
      makeResume(42)
      joined(0)
      send(0, meta({ replay: 'full' }))
      send(0, out('everything'))
      send(0, meta({ offset: 60 }))
      expect(cbs.onReset).toHaveBeenCalledTimes(1)
    })

    it.each([-1, 1.5, NaN])('resume %s is ignored: plain URL, resumePoint() null', (resume) => {
      const c = makeResume(resume)
      expect(ws(0).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s/ws')
      expect(c.resumePoint()).toBeNull()
    })
  })
})

// DEBT-05 (IN-05): a paste split into frames and cut short by a socket drop says so.
// bufferedAmount keeps the bytes a closed socket never sent (WHATWG), so the client
// counts the whole paste frames that left; a pong to the ping sent right after the
// paste proves the host read them all and queued them for the PTY in order (INPT-01:
// queued bytes still reach the shell after a drop). Nothing is queued or resent here.
describe('partial paste on a socket drop (DEBT-05)', () => {
  class WS {
    static OPEN = 1
    static instances: WS[] = []
    binaryType = ''
    readyState: number
    bufferedAmount = 0
    onopen: (() => void) | null = null
    onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
    onclose: ((e: { code: number; reason: string }) => void) | null = null
    onerror: (() => void) | null = null
    send = vi.fn()
    close = vi.fn()
    constructor(public url: string) { this.readyState = WS.openAtOnce ? 1 : 0; WS.instances.push(this) }
    static openAtOnce = true
  }
  const enc = new TextEncoder()
  const PASTE = 'x'.repeat(100_000) // frames of 32767, 32767, 32767 and 1699 payload bytes
  const ws = (i: number) => WS.instances[i]
  const frames = (i: number) => ws(i).send.mock.calls.map((c) => c[0] as Uint8Array)
  const msg = (i: number, f: Uint8Array) => ws(i).onmessage?.({ data: f.buffer as ArrayBuffer })
  const pong = () => new Uint8Array([MSG_PONG])
  const drop = (i: number, bufferedAmount: number) => { ws(i).bufferedAmount = bufferedAmount; ws(i).readyState = 3; ws(i).onclose?.({ code: 1006, reason: '' }) }
  let onPasteInterrupted: ReturnType<typeof vi.fn>
  let client: RelayClient | null

  const host = () => {
    client = new RelayClient(34115, 's', { onOutput: vi.fn(), onPasteInterrupted } as RelayClientCallbacks)
    return client
  }

  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    WS.instances = []
    WS.openAtOnce = true
    vi.stubGlobal('WebSocket', WS)
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    onPasteInterrupted = vi.fn()
    client = null
  })
  afterEach(() => {
    client?.close()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (document as any).visibilityState
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('host path: 40,000 bytes still buffered at the drop leaves only frame 1 whole: (32767, 100000)', () => {
    host().sendInput(PASTE)
    expect(frames(0).slice(0, 4).map((f) => f.length)).toEqual([32768, 32768, 32768, 1700])
    drop(0, 40_000)
    expect(onPasteInterrupted).toHaveBeenCalledTimes(1)
    expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: 32767, totalBytes: 100_000 })
  })

  it('the split paste is followed by exactly one MSG_PING (the ack)', () => {
    host().sendInput(PASTE)
    const sent = frames(0)
    expect(sent.map((f) => f[0])).toEqual([MSG_INPUT, MSG_INPUT, MSG_INPUT, MSG_INPUT, MSG_PING])
  })

  it('nothing buffered and the ack pong received: no notice', () => {
    host().sendInput(PASTE)
    msg(0, pong())
    drop(0, 0)
    expect(onPasteInterrupted).not.toHaveBeenCalled()
  })

  // INPT-02: the host writes the pong ahead of its queued output, but a pong that
  // still lands after a burst of output frames acks the paste just the same.
  it('the ack pong arriving after a burst of output frames still clears the paste: no notice', () => {
    host().sendInput(PASTE)
    for (let i = 0; i < 300; i++) msg(0, new Uint8Array([0x01, ...enc.encode(`line ${i}\r\n`)]))
    msg(0, pong())
    drop(0, 0)
    expect(onPasteInterrupted).not.toHaveBeenCalled()
  })

  it('nothing buffered but no ack pong: one notice with sentBytes null', () => {
    host().sendInput(PASTE)
    drop(0, 0)
    expect(onPasteInterrupted).toHaveBeenCalledTimes(1)
    expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: null, totalBytes: 100_000 })
  })

  it('a pong to a ping sent before the paste is not the ack', async () => {
    host()
    ws(0).onopen?.()
    await vi.advanceTimersByTimeAsync(RECONNECT.pingMs) // the keep-alive ping goes out
    expect(frames(0).filter((f) => f[0] === MSG_PING)).toHaveLength(1)
    client!.sendInput(PASTE)
    msg(0, pong()) // answers the keep-alive, not the paste's ping
    drop(0, 0)
    expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: null, totalBytes: 100_000 })
  })

  it('keystrokes queued after the paste do not count against it: 20 buffered bytes and the ack pong, no notice', () => {
    host().sendInput(PASTE)
    for (let i = 0; i < 10; i++) client!.sendInput('k')
    msg(0, pong())
    drop(0, 20)
    expect(onPasteInterrupted).not.toHaveBeenCalled()
  })

  it('keystrokes buffered after an unacknowledged paste: unconfirmed (null), never a partial count', () => {
    host().sendInput(PASTE)
    for (let i = 0; i < 10; i++) client!.sendInput('k')
    drop(0, 20)
    expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: null, totalBytes: 100_000 })
  })

  it('two split pastes back to back, part of the second buffered: one notice covering both', () => {
    host().sendInput(PASTE)
    client!.sendInput(PASTE)
    drop(0, 40_000)
    expect(onPasteInterrupted).toHaveBeenCalledTimes(1)
    // The first paste, its ack ping, then frame 1 of the second left whole.
    expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: 100_000 + 32_767, totalBytes: 200_000 })
  })

  describe('reconnect mode', () => {
    const grid = new Uint8Array([0x02, 0, 80, 0, 24])
    const meta = (m: object) => new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))])
    const open = (i: number) => { ws(i).readyState = 1; ws(i).onopen?.(); msg(i, grid); msg(i, meta({ replay: 'full' })); msg(i, meta({ offset: 0 })) }
    const guest = () => {
      WS.openAtOnce = false
      client = new RelayClient(1234, 's', { onOutput: vi.fn(), onPasteInterrupted } as RelayClientCallbacks, { remote: true, reconnect: {} })
      return client
    }

    it('a drop mid-paste: one notice, and after the redial the new socket carries no input frame', async () => {
      guest()
      open(0)
      client!.sendInput(PASTE)
      drop(0, 40_000)
      expect(onPasteInterrupted).toHaveBeenCalledTimes(1)
      expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: 32767, totalBytes: 100_000 })
      await vi.advanceTimersByTimeAsync(1000)
      expect(WS.instances).toHaveLength(2)
      open(1)
      await vi.advanceTimersByTimeAsync(RECONNECT.pingMs)
      expect(frames(1).some((f) => f[0] === MSG_INPUT)).toBe(false)
      expect(onPasteInterrupted).toHaveBeenCalledTimes(1)
    })

    it('the watchdog abandoning a stale socket with paste bytes buffered: one notice', async () => {
      guest()
      open(0)
      msg(0, pong()) // arms the watchdog (no ping yet, so it acks nothing)
      client!.sendInput(PASTE)
      ws(0).bufferedAmount = 40_000
      await vi.advanceTimersByTimeAsync(RECONNECT.deadMs + RECONNECT.probeGraceMs + 2000)
      expect(ws(0).close).toHaveBeenCalled()
      expect(onPasteInterrupted).toHaveBeenCalledTimes(1)
      expect(onPasteInterrupted).toHaveBeenCalledWith({ sentBytes: 32767, totalBytes: 100_000 })
    })
  })

  it('a user close() mid-paste: no notice', () => {
    host().sendInput(PASTE)
    client!.close()
    drop(0, 40_000)
    expect(onPasteInterrupted).not.toHaveBeenCalled()
  })

  it('a single-frame input then a drop: no notice and no ack ping', () => {
    host().sendInput('ls\r')
    expect(frames(0).map((f) => f[0])).toEqual([MSG_INPUT])
    drop(0, 4)
    expect(onPasteInterrupted).not.toHaveBeenCalled()
  })
})

// Phase 11 (PREV-01): MSG_PREVIEW frames as the Go side builds them (internal/relay/preview_test.go
// writes preview-frames.json with -update-preview-fixture), so this parser cannot drift from Go.
describe('MSG_PREVIEW parsing against the Go fixture (PREV-01)', () => {
  type Case = { name: string; kind: string; frame: string; header?: Record<string, unknown>; id?: number; payload?: string; status?: number }
  const cases = (previewFixture as { cases: Case[] }).cases
  const b64 = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0))

  it('pins the type byte and the chunk size', () => {
    expect(MSG_PREVIEW).toBe(0x21)
    expect(previewFixture.chunkMax).toBe(PREVIEW_CHUNK_MAX)
    expect(cases.map((c) => c.kind).sort()).toEqual(['chunk', 'chunk', 'end', 'end', 'end', 'header', 'header'])
  })

  for (const c of cases) {
    it(`decodes ${c.name}`, () => {
      const f = parseServerFrame(b64(c.frame))
      expect(f.type).toBe('preview')
      if (f.type !== 'preview') return
      const p = f.preview
      expect(p.kind).toBe(c.kind)
      if (p.kind === 'header') expect(p).toEqual({ kind: 'header', ...c.header })
      if (p.kind === 'chunk') {
        expect(p.id).toBe(c.id)
        expect(Array.from(p.payload)).toEqual(Array.from(b64(c.payload!)))
      }
      if (p.kind === 'end') expect({ id: p.id, status: p.status }).toEqual({ id: c.id, status: c.status })
    })
  }

  it('drops a truncated chunk, an unknown sub-kind, broken header JSON, a mistyped header and a short end', () => {
    const enc = new TextEncoder()
    expect(parseServerFrame(new Uint8Array([0x21, 0x02, 0, 0, 1])).type).toBe('unknown')
    expect(parseServerFrame(new Uint8Array([0x21, 0x09, 0, 0, 0, 1, 0])).type).toBe('unknown')
    expect(parseServerFrame(new Uint8Array([0x21, 0x01, ...enc.encode('{"id":1,')])).type).toBe('unknown')
    expect(parseServerFrame(new Uint8Array([0x21, 0x01, ...enc.encode('{"id":"1","name":"a","mime":"image/png","size":1,"key":"k","focus":true}')])).type).toBe('unknown')
    expect(parseServerFrame(new Uint8Array([0x21, 0x01, ...enc.encode('null')])).type).toBe('unknown')
    expect(parseServerFrame(new Uint8Array([0x21, 0x03, 0, 0, 0, 1])).type).toBe('unknown')
    expect(parseServerFrame(new Uint8Array([0x21])).type).toBe('unknown')
  })

  describe('RelayClient delivery', () => {
    class WS {
      static OPEN = 1
      static instances: WS[] = []
      binaryType = ''
      readyState = 1
      onopen: (() => void) | null = null
      onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
      onclose: ((e: { code: number; reason: string }) => void) | null = null
      onerror: (() => void) | null = null
      send = vi.fn()
      close = vi.fn()
      constructor(public url: string) { WS.instances.push(this) }
    }
    const enc = new TextEncoder()
    const send = (i: number, f: Uint8Array) => WS.instances[i].onmessage?.({ data: f.slice().buffer as ArrayBuffer })
    let client: RelayClient | null = null
    beforeEach(() => { vi.useFakeTimers(); WS.instances = []; vi.stubGlobal('WebSocket', WS) })
    afterEach(() => { client?.close(); client = null; vi.unstubAllGlobals(); vi.useRealTimers() })

    it('passes preview frames to onPreview in order and never moves the resume offset', () => {
      const onPreview = vi.fn()
      client = new RelayClient(1, 's', { onOutput: vi.fn(), onPreview }, { remote: true, reconnect: {} })
      WS.instances[0].onopen?.()
      send(0, new Uint8Array([0x20, ...enc.encode(JSON.stringify({ replay: 'full' }))]))
      send(0, new Uint8Array([0x20, ...enc.encode(JSON.stringify({ offset: 10 }))]))
      send(0, new Uint8Array([0x01, 0x61]))
      expect(client.resumePoint()).toBe(12)
      for (const c of cases) send(0, b64(c.frame))
      expect(client.resumePoint()).toBe(12)
      expect(onPreview.mock.calls.map((c) => (c[0] as PreviewFrame).kind)).toEqual(cases.map((c) => c.kind))
    })

    it('fires onPreviewReset when the socket closes, before the reconnect', () => {
      const onPreviewReset = vi.fn()
      client = new RelayClient(1, 's', { onOutput: vi.fn(), onPreviewReset }, { remote: true, reconnect: {} })
      WS.instances[0].onopen?.()
      WS.instances[0].onclose?.({ code: 1006, reason: '' })
      expect(onPreviewReset).toHaveBeenCalledTimes(1)
      expect(WS.instances).toHaveLength(1) // the redial waits for its backoff
    })

    it('fires onPreviewReset on a plain (non-reconnect) close too', () => {
      const onPreviewReset = vi.fn()
      client = new RelayClient(1, 's', { onOutput: vi.fn(), onPreviewReset })
      WS.instances[0].onclose?.({ code: 1000, reason: '' })
      expect(onPreviewReset).toHaveBeenCalledTimes(1)
    })
  })
})
