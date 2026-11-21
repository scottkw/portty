/**
 * HARD-05 (Phase 7) — switching away from a peer tab and back gets a delta replay.
 *
 * App renders only the active peer tab, so a tab switch unmounts TerminalPanel.
 * A desktop peer panel (remote, no wsURL) saves its xterm content and stream offset
 * on unmount and restores them on mount, then dials with ?resume=<offset>.
 * REAL RelayClient over a controllable mock WebSocket; xterm/addon mocks copied
 * from TerminalPanel.reconnect.test.tsx, with serialize() returning SNAPSHOT-DATA.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

const hoisted = vi.hoisted(() => ({
  // DEBT-04: when a write callback runs. 'sync' parses at once; a number defers it
  // that many ms; 'never' models xterm never calling back (W2).
  flush: 'sync' as 'sync' | 'never' | number,
  state: {
    term: null as {
      options: Record<string, unknown>
      reset: { mock: { calls: unknown[]; invocationCallOrder: number[] } }
      write: { mock: { calls: unknown[][]; invocationCallOrder: number[] } }
      dispose: { mock: { calls: unknown[][] } }
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
    // xterm runs a write callback once that chunk is parsed; the mock parses per hoisted.flush.
    public write = vi.fn((_d: unknown, cb?: () => void) => {
      if (!cb || hoisted.flush === 'never') return
      if (hoisted.flush === 'sync') cb()
      else setTimeout(cb, hoisted.flush)
    })
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
  SerializeAddon: class { activate(_t: unknown) {} dispose() {} serialize() { return 'SNAPSHOT-DATA' } },
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
import { PEER_FLUSH_TIMEOUT_MS, PEER_SAVE_WAIT_MS, dropPeerSnapshot, peerSnapshotKey, putPeerSnapshot, takePeerSnapshot } from '../../lib/peerSnapshots'

const enc = new TextEncoder()
const metaFrame = (m: object) => new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))])
const outFrame = (s: string) => new Uint8Array([0x01, ...enc.encode(s)])

const RIS = '\x1bc'
type MockTerm = NonNullable<typeof hoisted.state.term>
// Non-empty writes in order, decoded (applyHostGrid's in-order flush writes '').
const writes = (t: MockTerm) => t.write.mock.calls
  .map((c) => (typeof c[0] === 'string' ? c[0] : new TextDecoder().decode(c[0] as Uint8Array)))
  .filter((d) => d !== '')

describe('TerminalPanel peer tab switch (HARD-05)', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    MockWS.instances = []
    vi.stubGlobal('WebSocket', MockWS)
    hoisted.state.term = null
    hoisted.flush = 'sync'
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { act(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    takePeerSnapshot(peerSnapshotKey('', 's1')) // never leak a snapshot into the next case
    takePeerSnapshot(peerSnapshotKey('h', 's1'))
    takePeerSnapshot(peerSnapshotKey('host-a', 's1'))
    takePeerSnapshot(peerSnapshotKey('host-b', 's1'))
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  function mount(props: Record<string, unknown> = { remote: true }): MockTerm {
    act(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, hostLabel: 'office-air', ...props,
      }))
    })
    return hoisted.state.term!
  }
  // A tab switch: App stops rendering this panel.
  const unmount = () => act(() => root.render(null))

  const ws = (i: number) => MockWS.instances[i]
  const send = (s: MockWS, frame: Uint8Array) => act(() => { s.onmessage?.({ data: frame.buffer as ArrayBuffer }) })
  const joined = (s: MockWS) => { act(() => { s.readyState = 1; s.onopen?.() }); send(s, new Uint8Array([0x02, 0, 80, 0, 24])) }

  // Socket reaches offset 104: full replay, "hello", offset meta 100, then "abc" (4 frame bytes).
  function liveAt104(s: MockWS) {
    joined(s)
    send(s, metaFrame({ replay: 'full' }))
    send(s, outFrame('hello'))
    send(s, metaFrame({ offset: 100 }))
    send(s, outFrame('abc'))
  }

  it('switch away and back restores the snapshot, dials resume=104, and a delta keeps the content', () => {
    const first = mount()
    liveAt104(ws(0))
    unmount()
    expect(ws(0).close).toHaveBeenCalled() // no socket stays open for the hidden tab
    expect(first.dispose).toHaveBeenCalled()

    const second = mount()
    expect(second).not.toBe(first)
    expect(writes(second)[0]).toBe('SNAPSHOT-DATA')
    expect(MockWS.instances).toHaveLength(2)
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws?resume=104')

    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    send(ws(1), outFrame('def'))
    send(ws(1), metaFrame({ offset: 108 }))
    expect(writes(second)).toEqual(['SNAPSHOT-DATA', 'def'])
    expect(writes(second)).not.toContain(RIS)
  })

  it('offset gone on the host: the full replay resets the restored content (RIS after the snapshot)', () => {
    mount()
    liveAt104(ws(0))
    unmount()

    const second = mount()
    expect(ws(1).url).toContain('?resume=104')
    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'full' }))
    send(ws(1), outFrame('everything'))
    send(ws(1), metaFrame({ offset: 200 }))
    expect(writes(second)).toEqual(['SNAPSHOT-DATA', RIS, 'everything'])
  })

  it('a last frame ending mid-escape stores nothing (WR-01): the next mount does a full replay', () => {
    mount()
    liveAt104(ws(0))
    send(ws(0), outFrame('$ \x1b[3')) // the next frame would start with "2m"
    unmount()

    const second = mount()
    expect(writes(second)).toEqual([])
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws')
  })

  it('a full replay after a torn drop re-syncs the tracker (IN-09): the tab still snapshots', async () => {
    mount()
    liveAt104(ws(0))
    send(ws(0), outFrame('$ \x1b]0;ti')) // the socket drops inside an OSC
    act(() => { ws(0).readyState = 3; ws(0).onclose?.({ code: 1006, reason: '' }) })
    await act(async () => { await vi.advanceTimersByTimeAsync(2999) })
    expect(MockWS.instances).toHaveLength(2)
    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'full' }))
    send(ws(1), outFrame('plain')) // no ESC or BEL that would end the OSC on its own
    send(ws(1), metaFrame({ offset: 200 }))
    unmount()

    const second = mount()
    expect(writes(second)[0]).toBe('SNAPSHOT-DATA')
    expect(ws(2).url).toContain('?resume=200')
  })

  it('an ended session stores nothing: the next mount has no snapshot and no resume', () => {
    mount()
    liveAt104(ws(0))
    act(() => { ws(0).readyState = 3; ws(0).onclose?.({ code: 1000, reason: 'session ended' }) })
    unmount()

    const second = mount()
    expect(writes(second)).toEqual([])
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws')
  })

  it('a host (local) tab stores nothing: a later remote mount has no snapshot and no resume', () => {
    mount({})
    joined(ws(0))
    send(ws(0), metaFrame({ replay: 'full' }))
    send(ws(0), outFrame('hello'))
    send(ws(0), metaFrame({ offset: 100 }))
    unmount()

    const second = mount()
    expect(writes(second)).toEqual([])
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws')
  })
  // DEBT-04 W2: xterm may never run the flush callback; the terminal is still disposed.
  it('a flush callback that never fires: the terminal is disposed at PEER_FLUSH_TIMEOUT_MS and nothing is stored', async () => {
    const first = mount()
    liveAt104(ws(0))
    hoisted.flush = 'never'
    unmount()
    expect(first.dispose).not.toHaveBeenCalled()
    await act(async () => { await vi.advanceTimersByTimeAsync(PEER_FLUSH_TIMEOUT_MS - 1) })
    expect(first.dispose).not.toHaveBeenCalled()
    await act(async () => { await vi.advanceTimersByTimeAsync(1) })
    expect(first.dispose).toHaveBeenCalledTimes(1)

    hoisted.flush = 'sync'
    const second = mount()
    expect(writes(second)).toEqual([])
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws')
  })

  // DEBT-04 IN-03: a quick switch back waits for the save still in flight.
  it('a remount while the save is pending waits for it, restores the snapshot and dials resume=104', async () => {
    mount()
    liveAt104(ws(0))
    hoisted.flush = 100
    unmount()
    hoisted.flush = 'sync'
    const second = mount()
    expect(MockWS.instances).toHaveLength(1) // not dialled yet: waiting on the save
    await act(async () => { await vi.advanceTimersByTimeAsync(100) })
    expect(MockWS.instances).toHaveLength(2)
    expect(writes(second)[0]).toBe('SNAPSHOT-DATA')
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws?resume=104')
  })

  it('a save that never finishes holds the remount PEER_SAVE_WAIT_MS at most; it then dials a full replay', async () => {
    const first = mount()
    liveAt104(ws(0))
    hoisted.flush = 'never'
    unmount()
    hoisted.flush = 'sync'
    const second = mount()
    await act(async () => { await vi.advanceTimersByTimeAsync(PEER_SAVE_WAIT_MS - 1) })
    expect(MockWS.instances).toHaveLength(1)
    await act(async () => { await vi.advanceTimersByTimeAsync(1) })
    expect(MockWS.instances).toHaveLength(2)
    expect(ws(1).url).toBe('ws://127.0.0.1:1234/api/relay/remote/s1/ws')
    expect(writes(second)).toEqual([])
    expect(first.dispose).not.toHaveBeenCalled()
    await act(async () => { await vi.advanceTimersByTimeAsync(PEER_FLUSH_TIMEOUT_MS - PEER_SAVE_WAIT_MS) })
    expect(first.dispose).toHaveBeenCalledTimes(1)
  })

  // DEBT-04 close race: App drops the entry before React unmounts the active tab's panel.
  it('a drop before the unmount: the late flush stores nothing', () => {
    mount({ remote: true, snapshotHost: 'h' })
    liveAt104(ws(0))
    dropPeerSnapshot('h', 's1')
    unmount()
    expect(takePeerSnapshot(peerSnapshotKey('h', 's1'))).toBeUndefined()
  })

  // INPT-04 (SC4): App renders one peer tab at a time, so two hosts' tabs with one
  // session id mount one after the other. The tab's snapshot host picks the snapshot
  // key, which picks the restored content and the resume offset.
  it('two hosts with one session id each rejoin with their own delta (resume=104 for host-a, resume=204 for host-b)', async () => {
    const base = 'ws://127.0.0.1:1234/api/relay/remote/s1/ws'
    const drop = async (s: MockWS) => {
      act(() => { s.readyState = 3; s.onclose?.({ code: 1006, reason: '' }) })
      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
    }
    putPeerSnapshot(peerSnapshotKey('host-a', 's1'), { data: 'A-SNAP', cols: 80, rows: 24, offset: 104 })
    putPeerSnapshot(peerSnapshotKey('host-b', 's1'), { data: 'B-SNAP', cols: 80, rows: 24, offset: 204 })

    const a = mount({ remote: true, snapshotHost: 'host-a' })
    expect(writes(a)[0]).toBe('A-SNAP')
    expect(ws(0).url).toBe(`${base}?resume=104`)
    joined(ws(0))
    send(ws(0), metaFrame({ replay: 'delta' }))
    send(ws(0), outFrame('a1'))
    send(ws(0), metaFrame({ offset: 150 }))
    await drop(ws(0))
    expect(MockWS.instances).toHaveLength(2)
    expect(ws(1).url).toBe(`${base}?resume=150`)
    joined(ws(1))
    send(ws(1), metaFrame({ replay: 'delta' }))
    send(ws(1), outFrame('a2'))
    send(ws(1), metaFrame({ offset: 160 }))
    expect(writes(a)).toEqual(['A-SNAP', 'a1', 'a2'])
    unmount()

    const b = mount({ remote: true, snapshotHost: 'host-b' })
    expect(writes(b)[0]).toBe('B-SNAP')
    expect(writes(b)).not.toContain('A-SNAP')
    expect(ws(2).url).toBe(`${base}?resume=204`)
    joined(ws(2))
    send(ws(2), metaFrame({ replay: 'delta' }))
    send(ws(2), outFrame('b1'))
    send(ws(2), metaFrame({ offset: 250 }))
    await drop(ws(2))
    expect(MockWS.instances).toHaveLength(4)
    expect(ws(3).url).toBe(`${base}?resume=250`)
    joined(ws(3))
    send(ws(3), metaFrame({ replay: 'delta' }))
    send(ws(3), outFrame('b2'))
    expect(writes(b)).toEqual(['B-SNAP', 'b1', 'b2'])
    expect(writes(a)).not.toContain(RIS)
    expect(writes(b)).not.toContain(RIS)
    // host-a's own tab state is still under its own key; host-b's mount never took it.
    expect(takePeerSnapshot(peerSnapshotKey('host-a', 's1'))).toMatchObject({ data: 'SNAPSHOT-DATA', offset: 160 })
  })

  it('unmounting while still waiting on a pending save creates no RelayClient and disposes the terminal', async () => {
    mount()
    liveAt104(ws(0))
    hoisted.flush = 100
    unmount()
    hoisted.flush = 'sync'
    const second = mount()
    unmount()
    expect(second.dispose).toHaveBeenCalledTimes(1)
    await act(async () => { await vi.advanceTimersByTimeAsync(PEER_FLUSH_TIMEOUT_MS) })
    expect(MockWS.instances).toHaveLength(1)
  })
})
