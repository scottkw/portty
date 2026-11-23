/**
 * Phase 5 (SHAR-07) — a guest terminal survives a dropped socket.
 *
 * TerminalPanel with the REAL RelayClient over a controllable mock WebSocket:
 * a drop redials with ?resume=<counted offset>, a delta replay keeps the xterm
 * buffer, a full replay resets it, the reconnecting banner appears only after
 * 3 s, and a final close stops. 09-06: host-path tabs reconnect after a
 * handoff's 1012 while the daemon still lists the session, and stop when it
 * does not (a crash and respawn). B5: a browser
 * view whose /info probes fail for 2+ min says the host can't be reached and keeps retrying.
 * xterm/addon mocks copied from TerminalPanel.guestClose.test.tsx.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const hoisted = vi.hoisted(() => ({
  state: {
    // 09-06: what the host tab's re-auth (hostReconnectVerdict) reads from the bindings.
    relayPort: 5555,
    sessions: [{ id: 's1', state: 'running' }] as { id: string; state: string }[],
    term: null as {
      options: Record<string, unknown>
      reset: { mock: { calls: unknown[]; invocationCallOrder: number[] } }
      write: { mock: { calls: unknown[][]; invocationCallOrder: number[] } }
      onData: { mock: { calls: unknown[][] } }
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
    public parser = { registerCsiHandler: vi.fn(() => ({ dispose: () => {} })), registerEscHandler: vi.fn(() => ({ dispose: () => {} })) }
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
    // xterm runs a write callback once that chunk is parsed; the mock parses at once.
    public write = vi.fn((_d: unknown, cb?: () => void) => { cb?.() })
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
vi.mock('../../wailsjs/go/main/App', () => ({
  SetSearchConfig: () => Promise.resolve(),
  GetRelayPort: () => Promise.resolve(hoisted.state.relayPort),
  ListSessions: () => Promise.resolve(hoisted.state.sessions),
  GetRelayPortChecked: () => Promise.resolve(hoisted.state.relayPort),
  ListSessionsChecked: () => Promise.resolve(hoisted.state.sessions),
}))
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
  bufferedAmount = 0
  send = vi.fn()
  close = vi.fn()
  constructor(public url: string) {
    MockWS.instances.push(this)
  }
}

import { TerminalPanel } from '../TerminalPanel'
import { WebShareSessionView } from '../WebShareSessionView'
import { peerSnapshotKey, takePeerSnapshot } from '../../lib/peerSnapshots'

const enc = new TextEncoder()
const metaFrame = (m: object) => new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))])
const outFrame = (s: string) => new Uint8Array([0x01, ...enc.encode(s)])

const BANNER = 'Connection lost — reconnecting…'
const RIS = '\x1bc'
const risWrites = () => hoisted.state.term!.write.mock.calls.filter((c) => c[0] === RIS)

describe('TerminalPanel guest reconnect (SHAR-07)', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    MockWS.instances = []
    hoisted.state.relayPort = 5555
    hoisted.state.sessions = [{ id: 's1', state: 'running' }]
    vi.stubGlobal('WebSocket', MockWS)
    hoisted.state.term = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { act(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    takePeerSnapshot(peerSnapshotKey('', 's1')) // HARD-05: an unmounted peer panel leaves a snapshot; each case starts fresh
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
  const send = (s: MockWS, frame: Uint8Array) => act(() => { s.onmessage?.({ data: frame.buffer as ArrayBuffer }) })
  // Upgraded and the host's first frame (its grid, sent at once on join) arrived (WR-03).
  // Input opens later, at the replay's offset meta (WR-01 it3).
  const joined = (s: MockWS) => { act(() => { s.readyState = 1; s.onopen?.() }); send(s, new Uint8Array([0x02, 0, 80, 0, 24])) }
  const drop = (s: MockWS, code = 1006, reason = '') => act(() => { s.readyState = 3; s.onclose?.({ code, reason }) })
  const advance = (ms: number) => act(async () => { await vi.advanceTimersByTimeAsync(ms) })

  // Socket 1 reaches a counted offset: full replay, offset meta 100, then live "world" (6 frame bytes).
  async function liveThenDrop() {
    joined(ws(0))
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

    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    send(ws(1), outFrame('missed'))
    // WR-01 it3: still replaying, so the host reads no input yet: stdin stays off.
    expect(hoisted.state.term!.options.disableStdin).toBe(true)
    send(ws(1), metaFrame({ offset: 113 }))
    expect(container.textContent).not.toContain(BANNER)
    expect(hoisted.state.term!.reset.mock.calls).toHaveLength(0)
    expect(risWrites()).toHaveLength(0)
    expect(hoisted.state.term!.options.disableStdin).toBe(false)
  })

  // 05-12: the reset goes through the write queue (RIS), never the synchronous
  // reset(), so output still queued in xterm cannot be drawn after it.
  it('a full replay on the new socket writes RIS once, in stream order before the next output', async () => {
    mount()
    await liveThenDrop()
    await advance(1000)
    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'full' }))
    send(ws(1), outFrame('again'))
    const term = hoisted.state.term!
    expect(term.reset.mock.calls).toHaveLength(0)
    expect(risWrites()).toHaveLength(1)
    const calls = term.write.mock.calls
    const risIdx = calls.findIndex((c) => c[0] === RIS)
    expect(risIdx).toBe(calls.length - 2)
    expect(new TextDecoder().decode(calls[calls.length - 1][0] as Uint8Array)).toBe('again')
  })

  // DEBT-05: a paste cut short by the drop shows a dismissible notice with client-computed numbers.
  const paste = (text: string) => act(() => { (hoisted.state.term!.onData.mock.calls[0][0] as (d: string) => void)(text) })
  function openAt100() {
    joined(ws(0))
    send(ws(0), metaFrame({ replay: 'full' }))
    send(ws(0), metaFrame({ offset: 100 }))
  }

  it('a drop with paste bytes buffered shows "Paste interrupted: 31 of 98 KB sent", and Dismiss removes it', async () => {
    mount()
    openAt100()
    paste('x'.repeat(100_000))
    ws(0).bufferedAmount = 40_000
    drop(ws(0))
    expect(container.textContent).toContain('Paste interrupted: 31 of 98 KB sent')
    await advance(3000)
    expect(container.textContent).toContain(BANNER) // shows together with the reconnecting banner
    const notice = Array.from(container.querySelectorAll('[role="status"]')).find((n) => n.textContent?.includes('Paste interrupted'))!
    await act(async () => { notice.querySelector<HTMLButtonElement>('button[aria-label="Dismiss"]')!.click() })
    expect(container.textContent).not.toContain('Paste interrupted')
    expect(container.textContent).toContain(BANNER)
  })

  it('a drop with nothing buffered and no ack pong shows "Paste may have been cut short (98 KB)"', () => {
    mount()
    openAt100()
    paste('x'.repeat(100_000))
    drop(ws(0))
    expect(container.textContent).toContain('Paste may have been cut short (98 KB)')
  })

  // INPT-03 (SC3): the browser viewer is WebShareSessionView -> TerminalPanel
  // with wsURL (the share server's socket, not the daemon proxy); its partial-
  // paste notice is React text in a role="status" element (no innerHTML).
  function mountBrowser() {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 200 })))
    act(() => {
      root.render(React.createElement(WebShareSessionView, {
        sessionId: 's1', capToken: 'tok', relayPort: 0, pluginConfig: null, hostLabel: 'office-air',
      }))
    })
    expect(MockWS.instances).toHaveLength(1)
    expect(ws(0).url).toMatch(/^wss?:\/\/[^/]+\/sessions\/s1\/ws\?cap=tok$/) // the wsURL path
  }
  const statusWith = (text: string) =>
    Array.from(container.querySelectorAll('[role="status"]')).find((n) => n.textContent?.includes(text))

  it('browser view (wsURL): a drop with nothing buffered and no ack pong shows "Paste may have been cut short (98 KB)" in role=status', () => {
    mountBrowser()
    openAt100()
    paste('x'.repeat(100_000))
    drop(ws(0))
    expect(statusWith('Paste may have been cut short (98 KB)')).toBeDefined()
  })

  it('browser view (wsURL): a drop with 40,000 bytes buffered shows "Paste interrupted: 31 of 98 KB sent" in role=status', () => {
    mountBrowser()
    openAt100()
    paste('x'.repeat(100_000))
    ws(0).bufferedAmount = 40_000
    drop(ws(0))
    expect(statusWith('Paste interrupted: 31 of 98 KB sent')).toBeDefined()
  })

  it('UI polish: onReconnectingChange follows the banner (true at 3 s, false on reopen), and false on unmount', async () => {
    const onReconnectingChange = vi.fn()
    mount({ remote: true, onReconnectingChange })
    await liveThenDrop()
    await advance(2999)
    expect(onReconnectingChange).not.toHaveBeenCalled()
    await advance(1)
    expect(onReconnectingChange.mock.calls).toEqual([[true]])
    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    send(ws(1), metaFrame({ offset: 106 }))
    expect(onReconnectingChange.mock.calls).toEqual([[true], [false]])

    // Unmounted mid-reconnect (the user switched tabs): it still reports false.
    drop(ws(1))
    await advance(3000)
    expect(onReconnectingChange.mock.calls).toEqual([[true], [false], [true]])
    act(() => root.unmount())
    expect(onReconnectingChange.mock.calls).toEqual([[true], [false], [true], [false]])
  })

  it('a blip that reopens within 1 s never shows the banner', async () => {
    mount()
    await liveThenDrop()
    await advance(1000)
    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    send(ws(1), metaFrame({ offset: 106 }))
    await advance(10_000)
    expect(container.textContent).not.toContain(BANNER)
  })

  it("1000 'share stopped' is final: no new socket and today's stopped-sharing banner", async () => {
    mount()
    joined(ws(0))
    drop(ws(0), 1000, 'share stopped')
    await advance(60_000)
    expect(MockWS.instances).toHaveLength(1)
    expect(container.textContent).toContain('office-air stopped sharing this session.')
    expect(container.textContent).not.toContain(BANNER)
  })

  // Phase 14 (MOB-03): Reload on a phone. Reconnecting shows it only once two connect timeouts say stuck.
  describe('Reload (MOB-03)', () => {
    const realMatchMedia = window.matchMedia
    const phoneMedia = (phone: boolean) => {
      window.matchMedia = ((media: string) => ({
        matches: phone && media === '(pointer: coarse) and (hover: none)', media,
        addEventListener() {}, removeEventListener() {},
      })) as unknown as typeof window.matchMedia
    }
    afterEach(() => { window.matchMedia = realMatchMedia })
    const reloadBtn = () => Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Reload')

    // Socket 0 joins and drops; each redial then never gets a host frame, so connectMs (15 s) fires per dial.
    async function dropThenStall() {
      mount()
      joined(ws(0))
      send(ws(0), metaFrame({ offset: 0 }))
      drop(ws(0))
      await advance(3_000)
      expect(container.textContent).toContain(BANNER)
    }

    it('reconnecting: no Reload until two connect timeouts, and none again once a host frame arrives', async () => {
      phoneMedia(true)
      await dropThenStall()
      expect(reloadBtn()).toBeUndefined()
      await advance(40_000) // dial at 1 s, timeout at 16 s, redial at 18 s, timeout at 33 s
      expect(container.querySelector('.terminal-session-container')!.getAttribute('data-connect-stuck')).toBe('true')
      expect(container.textContent).toContain(BANNER)
      expect(reloadBtn()).not.toBeUndefined()
      await advance(20_000)
      joined(ws(MockWS.instances.length - 1)) // the host's first frame clears stuck
      expect(reloadBtn()).toBeUndefined()
    })

    it('reconnecting stuck without the phone query shows no Reload', async () => {
      phoneMedia(false)
      await dropThenStall()
      await advance(40_000)
      expect(container.querySelector('.terminal-session-container')!.getAttribute('data-connect-stuck')).toBe('true')
      expect(reloadBtn()).toBeUndefined()
    })

    it('ended (guest final close) on a phone carries Reload, and on desktop does not', () => {
      phoneMedia(true)
      mount()
      joined(ws(0))
      drop(ws(0), 1000, 'session ended')
      expect(container.textContent).toContain('Session ended')
      expect(reloadBtn()).not.toBeUndefined()
      act(() => root.unmount())
      container.remove()
      container = document.createElement('div')
      document.body.appendChild(container)
      root = createRoot(container)
      MockWS.instances = []
      phoneMedia(false)
      mount()
      joined(ws(0))
      drop(ws(0), 1000, 'session ended')
      expect(container.textContent).toContain('Session ended')
      expect(reloadBtn()).toBeUndefined()
    })

    it('unreachable (browser view, probes failing 2+ min) on a phone carries Reload', async () => {
      phoneMedia(true)
      vi.stubGlobal('fetch', vi.fn(async (): Promise<Response> => { throw new TypeError('Failed to fetch') }))
      act(() => {
        root.render(React.createElement(WebShareSessionView, {
          sessionId: 's1', capToken: 'tok', relayPort: 0, pluginConfig: null, hostLabel: 'office-air',
        }))
      })
      joined(ws(0))
      send(ws(0), metaFrame({ offset: 0 }))
      drop(ws(0))
      await advance(121_000)
      expect(container.textContent).toContain("Can't reach office-air.")
      expect(reloadBtn()).not.toBeUndefined()
    })
  })

  // B5: the browser cannot tell a refused probe (host daemon dead) from a sleeping
  // host, so it never stops on one; after 2 min failing the banner says so.
  it('browser view: probes refused for 2+ min show the long-outage banner, never stop, and a success restores it', async () => {
    const LONG = "Can't reach office-air. The session may have ended or the Mac may be asleep. Still retrying…"
    const fetchMock = vi.fn(async (): Promise<Response> => { throw new TypeError('Failed to fetch') })
    vi.stubGlobal('fetch', fetchMock)
    act(() => {
      root.render(React.createElement(WebShareSessionView, {
        sessionId: 's1', capToken: 'tok', relayPort: 0, pluginConfig: null, hostLabel: 'office-air',
      }))
    })
    joined(ws(0))
    send(ws(0), metaFrame({ offset: 0 }))
    drop(ws(0))

    // Backoff 1, 2, 4, 8, 16, 30, 30 s: probes at 1 … 91 s, the first failure at 1 s.
    await advance(91_000)
    expect(container.textContent).toContain(BANNER)
    expect(container.textContent).not.toContain(LONG)
    await advance(30_000) // the probe at 121 s: failing for 120 s
    expect(container.textContent).toContain(LONG)
    expect(container.textContent).not.toContain(BANNER)

    await advance(10 * 60_000)
    expect(container.textContent).toContain(LONG)
    expect(container.textContent).not.toContain('stopped sharing')
    expect(MockWS.instances).toHaveLength(1) // every refused probe retried; none dialed
    expect(hoisted.state.term!.options.disableStdin).toBe(true)

    fetchMock.mockImplementation(async () => new Response(null, { status: 200 }))
    await advance(30_000)
    expect(MockWS.instances).toHaveLength(2)
    joined(ws(1))
    send(ws(1), metaFrame({ offset: 0 }))
    expect(container.textContent).not.toContain(LONG)
    expect(container.textContent).not.toContain(BANNER)

    // The outage clock restarted: a new drop shows the normal banner again.
    drop(ws(1))
    await advance(3_000)
    expect(container.textContent).toContain(BANNER)
    expect(container.textContent).not.toContain(LONG)
  })

  it('browser view: a 403 after a long outage still stops with the stopped-sharing banner', async () => {
    const fetchMock = vi.fn(async (): Promise<Response> => { throw new TypeError('Failed to fetch') })
    vi.stubGlobal('fetch', fetchMock)
    act(() => {
      root.render(React.createElement(WebShareSessionView, {
        sessionId: 's1', capToken: 'tok', relayPort: 0, pluginConfig: null, hostLabel: 'office-air',
      }))
    })
    joined(ws(0))
    drop(ws(0))
    await advance(121_000)
    fetchMock.mockImplementation(async () => new Response(null, { status: 403 }))
    await advance(30_000)
    expect(container.textContent).toContain('office-air stopped sharing this session.')
    expect(container.textContent).not.toContain("Can't reach")
    await advance(60_000)
    expect(MockWS.instances).toHaveLength(1)
  })

  // 09-06 (UPGR-02): the old premise "a local relay drop means the daemon and
  // session died" is false once a handoff can close viewers with 1012.
  it('a host-path tab reconnects after 1012 with a delta from the re-read port while its session is listed', async () => {
    mount({})
    expect(ws(0).url).toBe('ws://127.0.0.1:1234/sessions/s1/ws')
    joined(ws(0))
    send(ws(0), metaFrame({ replay: 'full' }))
    send(ws(0), outFrame('hello'))
    send(ws(0), metaFrame({ offset: 100 }))
    send(ws(0), outFrame('world'))
    drop(ws(0), 1012, 'service restart')
    expect(hoisted.state.term!.options.disableStdin).toBe(true)

    await advance(2999)
    expect(MockWS.instances).toHaveLength(2)
    expect(ws(1).url).toBe('ws://127.0.0.1:5555/sessions/s1/ws?resume=106')
    await advance(1)
    expect(container.textContent).toContain(BANNER)

    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    send(ws(1), outFrame('missed'))
    send(ws(1), metaFrame({ offset: 113 }))
    expect(container.textContent).not.toContain(BANNER)
    expect(risWrites()).toHaveLength(0)
    expect(hoisted.state.term!.options.disableStdin).toBe(false)
  })

  it('a host-path tab stops quietly when the daemon no longer lists its session (a crash and respawn)', async () => {
    hoisted.state.sessions = [{ id: 'other', state: 'running' }]
    mount({})
    joined(ws(0))
    drop(ws(0), 1012, 'service restart')
    await advance(60_000)
    expect(MockWS.instances).toHaveLength(1)
    expect(container.textContent).not.toContain(BANNER)
    expect(container.textContent).not.toContain('ended')
  })
})
