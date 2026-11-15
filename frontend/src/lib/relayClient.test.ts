import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  MSG_INPUT,
  MSG_OUTPUT,
  MSG_RESIZE,
  MSG_RESIZE2,
  encodeInputFrame,
  encodeResizeFrame,
  parseServerFrame,
  RelayClient,
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
