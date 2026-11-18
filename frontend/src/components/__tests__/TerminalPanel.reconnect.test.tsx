/**
 * Phase 5 (SHAR-07) — a guest terminal survives a dropped socket.
 *
 * TerminalPanel with the REAL RelayClient over a controllable mock WebSocket:
 * a drop redials with ?resume=<counted offset>, a delta replay keeps the xterm
 * buffer, a full replay resets it, the reconnecting banner appears only after
 * 3 s, a final close stops, and host-path tabs never reconnect.
 * xterm/addon mocks copied from TerminalPanel.guestClose.test.tsx.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const hoisted = vi.hoisted(() => ({
  state: {
    term: null as {
      options: Record<string, unknown>
      reset: { mock: { calls: unknown[]; invocationCallOrder: number[] } }
      write: { mock: { calls: unknown[][]; invocationCallOrder: number[] } }
    } | null,
  },
}))

vi.mock('@xterm/xterm', async () => {
  const { vi } = await import('vitest')
  class MockTerminal {
    public cols = 80
    public rows = 24
    public options: Record<string, unknown> = {}
    public unicode = { activeVersion: '6' as string }
    public element: HTMLElement | null = null
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    public _core: any
    public loadAddon = vi.fn((addon: { activate?: (t: unknown) => void }) => {
      if (typeof addon?.activate === 'function') addon.activate(this)
    })
    public open = vi.fn((container: HTMLElement) => {
      this.element = document.createElement('div')
      container.appendChild(this.element)
    })
    public attachCustomKeyEventHandler = vi.fn()
    public onData = vi.fn(() => ({ dispose: () => {} }))
    public onResize = vi.fn(() => ({ dispose: () => {} }))
    public write = vi.fn()
    public reset = vi.fn()
    public dispose = vi.fn()
    public clearTextureAtlas = vi.fn()
    public refresh = vi.fn()
    public resize = vi.fn()
    constructor(_opts: Record<string, unknown>) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      hoisted.state.term = this as any
      this._core = {
        _renderService: {
          dimensions: { css: { cell: { width: 9, height: 17 } } },
          clear: vi.fn(),
        },
        _createRenderer: vi.fn(),
      }
    }
  }
  return { Terminal: MockTerminal }
})

vi.mock('@xterm/addon-fit', () => ({
  FitAddon: class { activate(_t: unknown) {} proposeDimensions() { return { cols: 80, rows: 24 } } },
}))
vi.mock('@xterm/addon-image', () => ({
  ImageAddon: class { activate(_t: unknown) {} dispose() {} },
}))
vi.mock('@xterm/addon-unicode11', () => ({
  Unicode11Addon: class { activate(_t: unknown) {} },
}))
vi.mock('@xterm/addon-clipboard', () => ({
  ClipboardAddon: class { activate(_t: unknown) {} dispose() {} },
}))
vi.mock('@xterm/addon-search', () => ({
  SearchAddon: class {
    activate(_t: unknown) {}
    dispose() {}
    onDidChangeResults() { return { dispose: () => {} } }
    findNext() {}
    findPrevious() {}
    clearDecorations() {}
  },
}))
vi.mock('@xterm/addon-web-links', () => ({
  WebLinksAddon: class { constructor(_h?: unknown, _o?: unknown) {} activate(_t: unknown) {} dispose() {} },
}))
vi.mock('@xterm/addon-serialize', () => ({
  SerializeAddon: class { activate(_t: unknown) {} dispose() {} serialize() { return '' } },
}))
vi.mock('@xterm/addon-progress', () => ({
  ProgressAddon: class { activate(_t: unknown) {} dispose() {} onChange() { return { dispose: () => {} } } },
}))
vi.mock('@xterm/addon-webgl', () => ({
  WebglAddon: class { activate(_t: unknown) {} dispose() {} onContextLoss() { return { dispose: () => {} } } },
}))
vi.mock('../../lib/webglProbe', () => ({ isSoftwareWebGL: () => false }))
vi.mock('../../wailsjs/go/main/App', () => ({ SetSearchConfig: () => Promise.resolve() }))
vi.mock('../../wailsjs/go/models', () => ({
  daemon: { SearchConfig: class { constructor(_o: unknown) {} } },
}))

class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof MockResizeObserver }).ResizeObserver = MockResizeObserver

// Controllable WebSocket: every instance and URL is recorded; tests drive the events.
class MockWS {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static instances: MockWS[] = []
  binaryType = ''
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
  onclose: ((e: { code: number; reason: string }) => void) | null = null
  onerror: (() => void) | null = null
  send = vi.fn()
  close = vi.fn()
  constructor(public url: string) {
    MockWS.instances.push(this)
  }
}

import { TerminalPanel } from '../TerminalPanel'

const enc = new TextEncoder()
const metaFrame = (m: object) => new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))])
const outFrame = (s: string) => new Uint8Array([0x01, ...enc.encode(s)])

const BANNER = 'Connection lost — reconnecting…'

describe('TerminalPanel guest reconnect (SHAR-07)', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    MockWS.instances = []
    vi.stubGlobal('WebSocket', MockWS)
    hoisted.state.term = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { act(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  function mount(props: Record<string, unknown> = { remote: true }) {
    act(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, hostLabel: 'office-air', ...props,
      }))
    })
    expect(MockWS.instances).toHaveLength(1)
  }

  const ws = (i: number) => MockWS.instances[i]
  const open = (s: MockWS) => act(() => { s.readyState = 1; s.onopen?.() })
  const send = (s: MockWS, frame: Uint8Array) => act(() => { s.onmessage?.({ data: frame.buffer as ArrayBuffer }) })
  const drop = (s: MockWS, code = 1006, reason = '') => act(() => { s.readyState = 3; s.onclose?.({ code, reason }) })
  const advance = (ms: number) => act(async () => { await vi.advanceTimersByTimeAsync(ms) })

  // Socket 1 reaches a counted offset: full replay, offset meta 100, then live "world" (6 frame bytes).
  async function liveThenDrop() {
    open(ws(0))
    send(ws(0), metaFrame({ replay: 'full' }))
    send(ws(0), outFrame('hello'))
    send(ws(0), metaFrame({ offset: 100 }))
    send(ws(0), outFrame('world'))
    drop(ws(0))
  }

  it('redials with resume=<counted offset>, shows the banner only after 3 s, and a delta keeps the buffer', async () => {
    mount()
    await liveThenDrop()
    expect(hoisted.state.term!.options.disableStdin).toBe(true)

    await advance(2999)
    expect(MockWS.instances).toHaveLength(2)
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws?resume=106')
    expect(container.textContent).not.toContain(BANNER)

    await advance(1)
    expect(container.textContent).toContain(BANNER)
    expect(MockWS.instances).toHaveLength(2)

    open(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    expect(container.textContent).not.toContain(BANNER)
    expect(hoisted.state.term!.reset.mock.calls).toHaveLength(0)
    expect(hoisted.state.term!.options.disableStdin).toBe(false)
  })

  it('a full replay on the new socket resets xterm once before the next write', async () => {
    mount()
    await liveThenDrop()
    await advance(1000)
    open(ws(1))
    send(ws(1), metaFrame({ replay: 'full' }))
    send(ws(1), outFrame('again'))
    const term = hoisted.state.term!
    expect(term.reset.mock.calls).toHaveLength(1)
    const lastWrite = term.write.mock.invocationCallOrder[term.write.mock.invocationCallOrder.length - 1]
    expect(term.reset.mock.invocationCallOrder[0]).toBeLessThan(lastWrite)
  })

  it('a blip that reopens within 1 s never shows the banner', async () => {
    mount()
    await liveThenDrop()
    await advance(1000)
    open(ws(1))
    await advance(10_000)
    expect(container.textContent).not.toContain(BANNER)
  })

  it("1000 'share stopped' is final: no new socket and today's stopped-sharing banner", async () => {
    mount()
    open(ws(0))
    drop(ws(0), 1000, 'share stopped')
    await advance(60_000)
    expect(MockWS.instances).toHaveLength(1)
    expect(container.textContent).toContain('office-air stopped sharing this session.')
    expect(container.textContent).not.toContain(BANNER)
  })

  it('a host-path tab (no remote, no wsURL) does not reconnect after a 1006 close', async () => {
    mount({})
    open(ws(0))
    drop(ws(0))
    await advance(60_000)
    expect(MockWS.instances).toHaveLength(1)
  })
})
