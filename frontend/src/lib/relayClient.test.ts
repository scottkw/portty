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
  encodeInputFrame,
  encodeResizeFrame,
  parseServerFrame,
  RelayClient,
  type ReconnectVerdict,
  type RelayClientCallbacks,
} from './relayClient'

describe('encodeInputFrame', () => {
  it('prepends MSG_INPUT byte (0x10) to UTF-8 encoded text', () => {
    const result = encodeInputFrame('hello')
    expect(result).toBeInstanceOf(Uint8Array)
    expect(result[0]).toBe(MSG_INPUT) // 0x10
    expect(Array.from(result)).toEqual([0x10, 104, 101, 108, 108, 111])
  })

  it('handles empty string', () => {
    const result = encodeInputFrame('')
    expect(result).toBeInstanceOf(Uint8Array)
    expect(result.length).toBe(1)
    expect(result[0]).toBe(MSG_INPUT)
  })

  it('handles multi-byte unicode characters', () => {
    const result = encodeInputFrame('A')
    expect(result[0]).toBe(MSG_INPUT)
    expect(result[1]).toBe(65) // 'A' = 0x41
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
    binaryType = ''
    readyState = 0
    onopen: (() => void) | null = null
    onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
    onclose: ((e: { code: number; reason: string }) => void) | null = null
    onerror: (() => void) | null = null
    send = vi.fn()
    close = vi.fn()
    constructor(public url: string) { WS.instances.push(this) }
  }
  let visibility = 'visible'
  const enc = new TextEncoder()
  const meta = (m: object) => new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))])
  const out = (t: string) => new Uint8Array([0x01, ...enc.encode(t)])
  const pong = new Uint8Array([MSG_PONG])
  const ws = (i: number) => WS.instances[i]
  const count = () => WS.instances.length
  const open = (i: number) => { ws(i).readyState = 1; ws(i).onopen?.() }
  const send = (i: number, f: Uint8Array) => ws(i).onmessage?.({ data: f.buffer as ArrayBuffer })
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
    open(0)
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
      await advance(60_000)
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

  describe('stream position', () => {
    it('output before any replay meta on a reconnect (older host) resets once', async () => {
      make()
      open(0)
      send(0, out('a'))
      expect(cbs.onReset).not.toHaveBeenCalled()
      drop(0)
      await advance(1000)
      open(1)
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
      open(1)
      send(1, meta({ replay: 'delta' }))
      send(1, out('partial'))
      drop(1)
      await advance(2000)
      expect(ws(2).url).not.toContain('resume=')
    })
  })
})
