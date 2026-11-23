/**
 * Phase 14-11 (MOB-01, T-14-26) — a pinch makes ONE grid claim.
 *
 * TerminalPanel hands attachPinchZoom getSize/setSize/onEnd. setSize drives the parent's font size
 * (as WebShareSessionView does), so the real font effect runs: it lays out and reports but must not
 * claim while pinching. onEnd makes the single claim at the final size. The mock terminal and relay
 * client mirror TerminalPanel.scale.test.tsx; the container is sized for a 100x30 fit at 9x17 cells.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

// ─── Hoisted shared state (shared between mock factories and test body) ───────

const hoisted = vi.hoisted(() => {
  return {
    // Captured from the most-recent RelayClient constructor call
    lastCallbacks: null as Record<string, unknown> | null,
    // Track sendResize (report) and sendClaim (INPT-06) calls
    sendResizeCalls: [] as Array<[number, number]>,
    sendClaimCalls: [] as Array<[number, number]>,
    // Cell metrics every mock terminal reads; mutate the fields (a font change)
    cell: { width: 9, height: 17 },
    // Track term.resize calls
    termResizeCalls: [] as Array<[number, number]>,
    // The most recent terminal element (to inspect style.transform)
    lastElement: null as HTMLElement | null,
    // When set, write callbacks queue here instead of running at once (xterm parses async)
    pendingWrites: null as Array<() => void> | null,
    // ResizeObserver callbacks, newest last (INPT-06: a test fires a container change)
    roCallbacks: [] as Array<() => void>,
    // The handlers TerminalPanel hands attachPinchZoom (the gesture itself is pinchZoom.test.ts)
    pinch: null as null | { getSize(): number; setSize(px: number): void; onEnd(): void },
  }
})

// ─── Module mocks ─────────────────────────────────────────────────────────────

vi.mock('../../lib/relayClient', async () => {
  // Capture callbacks from the most recent RelayClient construction.
  // The onOpen callback simulates the WS-open event; tests call it directly.
  class MockRelayClient {
    constructor(_p: number, _id: string, cbs: Record<string, unknown>) {
      hoisted.lastCallbacks = cbs
      hoisted.sendResizeCalls = []
      hoisted.sendClaimCalls = []
    }
    sendInput() {}
    sendResize(cols: number, rows: number) { hoisted.sendResizeCalls.push([cols, rows]) }
    sendClaim(cols: number, rows: number) { hoisted.sendClaimCalls.push([cols, rows]) }
    resumePoint() { return null } // HARD-05: no offset known, so no tab-switch snapshot
    close() {}
  }
  return {
    RelayClient: MockRelayClient,
    MSG_OUTPUT: 0x01,
    MSG_RESIZE: 0x02,
    MSG_INPUT: 0x10,
    MSG_RESIZE2: 0x11,
    MSG_PING: 0x12,
    encodeInputFrames: () => [],
    encodeResizeFrame: () => new Uint8Array(),
    parseServerFrame: () => ({ type: 'unknown' }),
  }
})

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
    public loadAddon = vi.fn()
    public open = vi.fn((container: HTMLElement) => {
      const el = document.createElement('div')
      container.appendChild(el)
      this.element = el
      hoisted.lastElement = el
    })
    public attachCustomKeyEventHandler = vi.fn()
    public onData = vi.fn(() => ({ dispose: () => {} }))
    public onResize = vi.fn(() => ({ dispose: () => {} }))
    public write = vi.fn((_d: unknown, cb?: () => void) => {
      if (!cb) return
      if (hoisted.pendingWrites) hoisted.pendingWrites.push(cb)
      else cb()
    })
    public dispose = vi.fn()
    public clearTextureAtlas = vi.fn()
    public refresh = vi.fn()
    public resize = vi.fn((cols: number, rows: number) => {
      // xterm updates cols/rows on resize — replicate for recomputeScale reads
      this.cols = cols
      this.rows = rows
      hoisted.termResizeCalls.push([cols, rows])
    })
    constructor(_opts: Record<string, unknown>) {
      this._core = {
        _renderService: {
          dimensions: { css: { cell: hoisted.cell } },
          clear: vi.fn(),
        },
      }
    }
  }
  return { Terminal: MockTerminal }
})

vi.mock('@xterm/addon-fit', () => ({
  FitAddon: class {
    proposeDimensions() { return { cols: 80, rows: 24 } }
    activate() {}
    fit() {}
  },
}))
vi.mock('@xterm/addon-image', () => ({
  ImageAddon: class { activate() {} dispose() {} },
}))
vi.mock('@xterm/addon-unicode11', () => ({
  Unicode11Addon: class { activate() {} },
}))
vi.mock('@xterm/addon-webgl', () => ({
  WebglAddon: class {
    onContextLoss() { return { dispose: () => {} } }
    activate() {}
    dispose() {}
  },
}))
vi.mock('@xterm/addon-clipboard', () => ({
  ClipboardAddon: class { activate() {} dispose() {} },
}))
vi.mock('@xterm/addon-search', () => ({
  SearchAddon: class {
    activate() {}
    dispose() {}
    onDidChangeResults() { return { dispose: () => {} } }
    findNext() {}
    findPrevious() {}
    clearDecorations() {}
  },
}))
vi.mock('@xterm/addon-web-links', () => ({
  WebLinksAddon: class { constructor(_h?: unknown, _o?: unknown) {} activate() {} dispose() {} },
}))
vi.mock('@xterm/addon-serialize', () => ({
  SerializeAddon: class { activate() {} dispose() {} serialize() { return '' } },
}))
vi.mock('@xterm/addon-progress', () => ({
  ProgressAddon: class { activate() {} dispose() {} onChange() { return { dispose: () => {} } } },
}))

vi.mock('../../lib/webglProbe', () => ({ isSoftwareWebGL: () => false }))
vi.mock('../../lib/isXtermFocused', () => ({ isXtermFocused: () => false }))
vi.mock('../../lib/touchScrollHandler', () => ({ attachTouchScroll: () => () => {} }))
vi.mock('../../lib/pinchZoom', () => ({
  attachPinchZoom: (_c: unknown, h: NonNullable<typeof hoisted.pinch>) => { hoisted.pinch = h; return () => { hoisted.pinch = null } },
}))
vi.mock('../../lib/urlSafety', () => ({
  isAllowedScheme: () => true,
  getRisk: () => null,
}))
vi.mock('../../lib/openLink', () => ({
  openLink: () => {},
  isModifierPressed: () => false,
}))
vi.mock('../../wailsjs/go/main/App', () => ({
  SetSearchConfig: () => Promise.resolve(),
}))
vi.mock('../../wailsjs/go/models', () => ({
  daemon: { SearchConfig: class { constructor(_o: unknown) {} } },
}))

// jsdom has no ResizeObserver. Stub that records its callback so a test can fire it.
class MockResizeObserver {
  constructor(cb: () => void) { hoisted.roCallbacks.push(cb) }
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof MockResizeObserver }).ResizeObserver =
  MockResizeObserver

// ─── Import SUT after all mocks ───────────────────────────────────────────────
import { TerminalPanel } from '../TerminalPanel'

const THEME = { background: '#000', foreground: '#fff' }

describe('14-11 TerminalPanel pinch: one claim, at the end', () => {
  let container: HTMLElement
  let root: Root
  let size: number
  let deltas: number[]

  beforeEach(() => {
    hoisted.pinch = null
    hoisted.sendResizeCalls = []
    hoisted.sendClaimCalls = []
    hoisted.cell.width = 9
    hoisted.cell.height = 17
    size = 13
    deltas = []
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.clearAllMocks()
  })

  // The parent applies each delta like WebShareSessionView's fallback does.
  function render() {
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's', isActive: true, relayPort: 0, fontSize: size, theme: THEME,
        wsURL: 'wss://host/sessions/s/ws?cap=x',
        onFontSizeChange: (d: number) => { deltas.push(d); size += d; render() },
      } as React.ComponentProps<typeof TerminalPanel>))
    })
  }
  function mount() {
    render()
    const node = container.querySelector('.terminal-session-container') as HTMLElement
    Object.defineProperty(node, 'clientWidth', { value: 900, configurable: true })
    Object.defineProperty(node, 'clientHeight', { value: 510, configurable: true })
  }
  // A bigger font fits fewer columns: the cell grows with the size, as xterm's would.
  const setSize = (px: number) => {
    hoisted.cell.width = px * 0.7
    hoisted.pinch!.setSize(px)
  }

  it('attaches in the touch effect and reads the rendered size', () => {
    mount()
    expect(hoisted.pinch).not.toBeNull()
    expect(hoisted.pinch!.getSize()).toBe(13)
  })

  it('five size changes while pinching send zero claims; the end sends exactly one, at the final fit', () => {
    mount()
    for (const px of [14, 15, 16, 17, 18]) setSize(px)
    expect(size).toBe(18)
    expect(deltas).toEqual([1, 1, 1, 1, 1]) // each against the size the pinch last asked for
    expect(hoisted.sendClaimCalls).toEqual([])
    expect(hoisted.sendResizeCalls.length).toBeGreaterThan(0) // the font effect laid out and reported
    hoisted.pinch!.onEnd()
    expect(hoisted.sendClaimCalls).toHaveLength(1)
    expect(hoisted.sendClaimCalls[0][0]).toBe(Math.floor(900 / (18 * 0.7))) // columns at the final font
  })

  it('a font change after the pinch has ended claims as before (the gate is cleared)', () => {
    mount()
    setSize(14)
    hoisted.pinch!.onEnd()
    hoisted.sendClaimCalls = []
    hoisted.cell.width = 9
    flushSync(() => { size = 20; render() })
    expect(hoisted.sendClaimCalls).toHaveLength(1)
  })

  it('a pinch that changed nothing still ends with one claim and no font change', () => {
    mount()
    hoisted.pinch!.onEnd()
    expect(deltas).toEqual([])
    expect(hoisted.sendClaimCalls).toHaveLength(1)
  })
})
