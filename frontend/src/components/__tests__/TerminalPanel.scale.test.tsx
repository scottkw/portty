/**
 * Phase 157 VIEW-04/05 — TerminalPanel guest honor + scale behavioral tests.
 * Phase 175-03 BUG-01 — extended with floor-aware guest viewport (readability
 * floor + horizontal-scroll fallback) behavioral tests.
 *
 * INPT-06 (Phase 10): every panel (local tab, peer tab, browser) follows the server
 *   grid: a RelayClient onResize(cols,rows) callback calls term.resize(cols,rows)
 *   then applies a CSS transform via computeGuestViewport, toggling the
 *   .terminal-guest--scroll-x class when the natural scale falls below the
 *   readability floor. Every panel reports its own measured fit (sendResize) on
 *   open, never the grid it shows; a font change on the active panel claims
 *   (sendClaim). A local tab before any grid frame fits itself, with no transform.
 *
 * Source-inspection assertions pin the isGuest gate and computeGuestViewport import;
 * behavioral assertions use mock-captured RelayClient callbacks to drive onResize.
 *
 * jsdom limitation: clientWidth/clientHeight default to 0, so the natural scale is
 * always 0 (min(0/gridW, 0/gridH)) unless a test explicitly overrides clientWidth/
 * clientHeight via Object.defineProperty. Without an override, computeGuestViewport
 * clamps to DEFAULT_GUEST_MIN_SCALE with overflowX=true (0 is below any floor) — a
 * test-env artifact, not a bug. Tests that need to exercise the above-floor /
 * below-floor branches explicitly override clientWidth/clientHeight on the
 * .terminal-session-container node before invoking onResize.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'
import src from '../TerminalPanel.tsx?raw'
import helperSrc from '../../lib/guestTerminal.ts?raw'

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

// jsdom has no ResizeObserver. Stub with a no-op.
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof MockResizeObserver }).ResizeObserver =
  MockResizeObserver

// ─── Import SUT after all mocks ───────────────────────────────────────────────
import { TerminalPanel } from '../TerminalPanel'
import { DEFAULT_GUEST_MIN_SCALE } from '../../lib/terminalScale'

// ─── Shared test helpers ──────────────────────────────────────────────────────

function makeBaseProps(overrides: Partial<React.ComponentProps<typeof TerminalPanel>> = {}) {
  return {
    sessionId: 'test-session',
    isActive: true,
    relayPort: 0,
    fontSize: 14,
    onFontSizeChange: () => {},
    theme: { background: '#000', foreground: '#fff' },
    ...overrides,
  } as React.ComponentProps<typeof TerminalPanel>
}

// ─── Source-inspection tests ──────────────────────────────────────────────────

describe('Phase 157 TerminalPanel — source gates (VIEW-04/05)', () => {
  it('imports computeGuestViewport from terminalScale (BUG-01 floor-aware helper)', () => {
    expect(src).toContain("from '../lib/terminalScale'")
    expect(src).toContain('computeGuestViewport')
  })

  it('computes isGuest = remote || !!wsURL', () => {
    expect(src).toMatch(/isGuest\s*=\s*remote\s*\|\|\s*!!wsURL/)
  })

  it('every panel\'s onResize stores the server grid and goes through applyHostGrid with recomputeScale (05-12, INPT-06)', () => {
    expect(src).toContain("from '../lib/guestTerminal'")
    const cb = src.slice(src.indexOf('onResize: (cols, rows) => {'), src.indexOf('onReset:'))
    expect(cb).toContain('serverGridRef.current = { cols, rows }')
    expect(cb).toContain('applyHostGrid(term, cols, rows, recomputeScale)')
  })

  it('applyHostGrid clamps and resizes inside a write callback (guest honor, stream order)', () => {
    expect(helperSrc).toMatch(/term\.write\('', \(\) => \{/)
    expect(helperSrc).toContain('term.resize(Math.max(1, cols), Math.max(1, rows))')
  })

  it('reports carry the measured fit, never the grid shown (INPT-06 invariant)', () => {
    expect(src).not.toContain('sendResize(term.cols')
    expect(src).not.toMatch(/term\.onResize\(/)
    expect(src).toContain('function measureFit(')
  })

  it('guest path: applyHostGrid calls its follow-up (recomputeScale) after term.resize (Pitfall 5 order)', () => {
    const resizeIdx = helperSrc.indexOf('term.resize')
    const afterIdx = helperSrc.indexOf('after?.()')
    expect(resizeIdx).toBeGreaterThanOrEqual(0)
    expect(afterIdx).toBeGreaterThanOrEqual(0)
    expect(resizeIdx).toBeLessThan(afterIdx)
  })

  it('relayout fits only before any grid is known, otherwise rescales, then reports', () => {
    const body = src.slice(src.indexOf('const relayout = useCallback('), src.indexOf('}, [recomputeScale, reportFit])'))
    expect(body).toContain('if (serverGridRef.current) recomputeScale()')
    expect(body).toContain('else fitTerminal(termRef.current!)')
    expect(body).toContain('reportFit()')
  })

  it('CSS contains transform-origin: top left for .xterm', () => {
    // This is checked via a separate style.css read in style tests;
    // here we just confirm the recomputeScale output uses scale()
    expect(src).toContain("term.element.style.transform = `scale(${s})`")
  })
})

// ─── Behavioral tests — guest path ───────────────────────────────────────────

describe('Phase 157 TerminalPanel — guest path behavioral (VIEW-04/05)', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    hoisted.lastCallbacks = null
    hoisted.sendResizeCalls = []
    hoisted.termResizeCalls = []
    hoisted.lastElement = null
    hoisted.pendingWrites = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    hoisted.pendingWrites = null
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.clearAllMocks()
  })

  function mountGuest(remote?: boolean, wsURL?: string) {
    flushSync(() => {
      root.render(
        React.createElement(TerminalPanel, makeBaseProps({ remote, wsURL })),
      )
    })
  }

  // VIEW-04: guest honors 0x02 → term.resize
  it('guest (remote=true): onResize callback calls term.resize with correct cols/rows', () => {
    mountGuest(true, undefined)
    expect(hoisted.lastCallbacks).not.toBeNull()
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined
    expect(onResize).toBeTypeOf('function')

    flushSync(() => { onResize?.(80, 24) })

    expect(hoisted.termResizeCalls).toContainEqual([80, 24])
  })

  it('guest (wsURL set): onResize callback calls term.resize with correct cols/rows', () => {
    mountGuest(undefined, 'wss://host/sessions/s1/ws?cap=tok')
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined
    expect(onResize).toBeTypeOf('function')

    flushSync(() => { onResize?.(132, 50) })

    expect(hoisted.termResizeCalls).toContainEqual([132, 50])
  })

  // VIEW-04: clamp to >= 1 (T-157-03 — zero-dim guard, xterm rejects resize(0,0))
  it('guest: clamps cols/rows to min 1 when zero-dim frame arrives', () => {
    mountGuest(true)
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined

    flushSync(() => { onResize?.(0, 0) })

    expect(hoisted.termResizeCalls).toContainEqual([1, 1])
  })

  // 05-12: a host grid change waits for output already queued in xterm, then
  // resizes, then rescales (Pitfall 5).
  it('guest: onResize waits for the queued write callback, then resizes, then rescales', () => {
    mountGuest(true)
    const onResize = hoisted.lastCallbacks?.['onResize'] as (cols: number, rows: number) => void
    hoisted.termResizeCalls = [] // ignore resizes made while mounting
    hoisted.lastElement!.style.transform = ''
    hoisted.pendingWrites = []

    flushSync(() => { onResize(100, 30) })
    expect(hoisted.termResizeCalls).toHaveLength(0)
    expect(hoisted.lastElement?.style.transform ?? '').toBe('')

    const queued = hoisted.pendingWrites
    hoisted.pendingWrites = null
    flushSync(() => { queued.forEach((cb) => cb()) })
    expect(hoisted.termResizeCalls).toEqual([[100, 30]])
    expect(hoisted.lastElement?.style.transform).toMatch(/^scale\(/)
    expect(hoisted.sendResizeCalls).toHaveLength(0)
  })

  // VIEW-05: guest applies CSS transform after onResize
  it('guest: onResize applies a scale(…) transform to term.element', () => {
    mountGuest(true)
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined

    flushSync(() => { onResize?.(80, 24) })

    expect(hoisted.lastElement?.style.transform).toMatch(/^scale\(/)
  })

  // Scale cap: transform never exceeds scale(1)
  it('guest: scale cap — transform never exceeds scale(1) even if container > grid', () => {
    mountGuest(true)
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined

    // In jsdom clientWidth=0 so scale=0; just assert transform is set, not undefined
    flushSync(() => { onResize?.(80, 24) })

    const t = hoisted.lastElement?.style.transform ?? ''
    expect(t).toMatch(/^scale\(/)
    // Extract the scale value from 'scale(X)' — must be <= 1
    const match = t.match(/^scale\(([\d.]+)\)/)
    if (match) {
      const s = parseFloat(match[1])
      expect(s).toBeLessThanOrEqual(1)
    }
  })

  // BUG-01: narrow guest viewport — natural scale falls below the readability
  // floor, so the scale clamps at the floor and the horizontal-scroll fallback
  // class is applied instead of shrinking further.
  it('guest: narrow container clamps to the readability floor and adds the scroll-x class', () => {
    mountGuest(true)
    const node = container.querySelector('.terminal-session-container') as HTMLElement
    // grid = 80cols*9px x 24rows*17px = 720x408. A 50x50 container is far below
    // the floor at any scale > 0.
    Object.defineProperty(node, 'clientWidth', { value: 50, configurable: true })
    Object.defineProperty(node, 'clientHeight', { value: 50, configurable: true })
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined

    flushSync(() => { onResize?.(80, 24) })

    expect(hoisted.lastElement?.style.transform).toBe(`scale(${DEFAULT_GUEST_MIN_SCALE})`)
    expect(node.classList.contains('terminal-guest--scroll-x')).toBe(true)
    expect(hoisted.sendResizeCalls).toHaveLength(0)
  })

  // BUG-01: wide guest viewport (container >= grid) — natural scale caps at 1.0
  // (never upscale, VIEW-04/05 invariant), which is always >= the floor, so no
  // horizontal-scroll fallback is applied.
  it('guest: wide container caps at scale(1) with no scroll-x class', () => {
    mountGuest(true)
    const node = container.querySelector('.terminal-session-container') as HTMLElement
    // grid = 720x408; a 2000x2000 container is far larger than the grid.
    Object.defineProperty(node, 'clientWidth', { value: 2000, configurable: true })
    Object.defineProperty(node, 'clientHeight', { value: 2000, configurable: true })
    const onResize = hoisted.lastCallbacks?.['onResize'] as
      | ((cols: number, rows: number) => void)
      | undefined

    flushSync(() => { onResize?.(80, 24) })

    expect(hoisted.lastElement?.style.transform).toBe('scale(1)')
    expect(node.classList.contains('terminal-guest--scroll-x')).toBe(false)
  })
})

// ─── 05-12: guest scrollbar at the pane edge ─────────────────────────────────
//
// xterm 6 puts its vertical scrollbar at the right edge of .xterm-scrollable-
// element, a block child of term.element. When the scale is below 1 the element
// shrinks with it, so the scrollbar sat mid-pane. The guest stretches the
// element to contentWidth / scale; the grid (screen canvas) keeps host cols.

describe('05-12 TerminalPanel — guest element spans the pane (scrollbar at the edge)', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    hoisted.lastCallbacks = null
    hoisted.sendResizeCalls = []
    hoisted.termResizeCalls = []
    hoisted.lastElement = null
    hoisted.pendingWrites = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.clearAllMocks()
  })

  function mount(props: Partial<React.ComponentProps<typeof TerminalPanel>>) {
    flushSync(() => { root.render(React.createElement(TerminalPanel, makeBaseProps(props))) })
    return container.querySelector('.terminal-session-container') as HTMLElement
  }
  function size(node: HTMLElement, w: number, h: number) {
    Object.defineProperty(node, 'clientWidth', { value: w, configurable: true })
    Object.defineProperty(node, 'clientHeight', { value: h, configurable: true })
  }
  const grid = (cols: number, rows: number) => {
    const onResize = hoisted.lastCallbacks?.['onResize'] as (c: number, r: number) => void
    flushSync(() => { onResize(cols, rows) })
  }
  const widthPx = () => parseFloat(hoisted.lastElement!.style.width)

  // grid = 100 cols x 9px by 24 rows x 17px = 900 x 408
  it('scale below 1 (height-limited): the element is stretched to contentWidth / scale', () => {
    const node = mount({ remote: true })
    size(node, 1000, 306) // scale = min(1000/900, 306/408) = 0.75
    grid(100, 24)
    expect(hoisted.lastElement!.style.transform).toBe('scale(0.75)')
    expect(widthPx()).toBeCloseTo(1000 / 0.75, 3)
  })

  it('subtracts the container padding so the padding stays', () => {
    const node = mount({ remote: true })
    node.style.padding = '12px 16px'
    size(node, 1032, 330) // content box 1000 x 306; scale = min(1000/900, 306/408) = 0.75
    grid(100, 24)
    expect(hoisted.lastElement!.style.transform).toBe('scale(0.75)')
    expect(widthPx()).toBeCloseTo(1000 / 0.75, 3)
  })

  // Width-limited: the scaled grid must fit the content box, so the stretched
  // element is exactly the grid's width and the scrollbar sits at the grid's
  // right edge instead of over its last columns.
  it('width-limited with padding: the grid fits the content box and the element is not narrower than the grid', () => {
    const node = mount({ remote: true })
    node.style.padding = '12px 16px'
    size(node, 752, 800) // content box 720 x 776; scale = 720/900 = 0.8
    grid(100, 24)
    expect(hoisted.lastElement!.style.transform).toBe('scale(0.8)')
    expect(widthPx()).toBeCloseTo(900, 3)
  })

  it('scale 1: the element is as wide as the content box', () => {
    const node = mount({ remote: true })
    size(node, 1200, 800)
    grid(100, 24)
    expect(hoisted.lastElement!.style.transform).toBe('scale(1)')
    expect(widthPx()).toBeCloseTo(1200, 3)
  })

  it('overflow-x branch, grid wider than the pane: the element keeps the grid width (scrollbar at the grid end)', () => {
    const node = mount({ wsURL: 'wss://host/sessions/s/ws?cap=x' })
    size(node, 1200, 800)
    grid(100, 24)
    expect(widthPx()).toBeCloseTo(1200, 3)
    size(node, 50, 50)
    grid(100, 24)
    expect(node.classList.contains('terminal-guest--scroll-x')).toBe(true)
    expect(hoisted.lastElement!.style.transform).toBe(`scale(${DEFAULT_GUEST_MIN_SCALE})`)
    expect(widthPx()).toBeCloseTo(900, 3)
    expect(hoisted.sendResizeCalls).toHaveLength(0)
  })

  // Seen in the VM: a tall host grid puts the natural scale under the floor
  // through the height alone, while the floored grid is still narrower than
  // the pane. The element must still reach the pane edge, not stop mid-grid.
  it('floor reached through the height: the element still spans the pane', () => {
    const node = mount({ remote: true })
    size(node, 1000, 200) // natural = min(1000/900, 200/408) = 0.49 -> floor 0.7; grid 630 px wide
    grid(100, 24)
    expect(hoisted.lastElement!.style.transform).toBe(`scale(${DEFAULT_GUEST_MIN_SCALE})`)
    expect(widthPx()).toBeCloseTo(1000 / DEFAULT_GUEST_MIN_SCALE, 3)
  })

  it('host panels get no stretch', () => {
    const node = mount({ remote: false })
    size(node, 1000, 306)
    const onOpen = hoisted.lastCallbacks?.['onOpen'] as () => void
    flushSync(() => { onOpen() })
    expect(hoisted.lastElement!.style.width).toBe('')
  })
})

// ─── Behavioral tests — host path ────────────────────────────────────────────

describe('Phase 157 TerminalPanel — local tab before any grid frame (VIEW-04/05)', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    hoisted.lastCallbacks = null
    hoisted.sendResizeCalls = []
    hoisted.termResizeCalls = []
    hoisted.lastElement = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.clearAllMocks()
  })

  function mountHost() {
    flushSync(() => {
      root.render(
        React.createElement(TerminalPanel, makeBaseProps({ remote: false, wsURL: undefined })),
      )
    })
  }

  // Host: no transform applied (host renders natively, never scaled)
  it('host: no CSS transform applied to term.element after open', () => {
    mountHost()
    // Simulate WS open
    const onOpen = hoisted.lastCallbacks?.['onOpen'] as (() => void) | undefined
    flushSync(() => { onOpen?.() })

    const t = hoisted.lastElement?.style.transform ?? ''
    // Host element must have NO scale transform
    expect(t).toBe('')
  })

  // BUG-01: host path never toggles the guest-only scroll-x fallback class —
  // the host PTY grid and its container are unaffected by the guest fix.
  it('host: never gains the terminal-guest--scroll-x class', () => {
    mountHost()
    const node = container.querySelector('.terminal-session-container') as HTMLElement
    const onOpen = hoisted.lastCallbacks?.['onOpen'] as (() => void) | undefined
    flushSync(() => { onOpen?.() })
    expect(node.classList.contains('terminal-guest--scroll-x')).toBe(false)
  })
})

// ─── INPT-06: report the fit, follow the grid, claim on zoom ─────────────────
//
// The container is sized to a 100x30 fit at the mock's 9x17 cells (900 x 510,
// no padding in jsdom).

describe('INPT-06 TerminalPanel — report the fit, follow the grid, claim on zoom', () => {
  let container: HTMLElement
  let root: Root

  beforeEach(() => {
    hoisted.lastCallbacks = null
    hoisted.sendResizeCalls = []
    hoisted.sendClaimCalls = []
    hoisted.termResizeCalls = []
    hoisted.lastElement = null
    hoisted.pendingWrites = null
    hoisted.cell.width = 9
    hoisted.cell.height = 17
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    hoisted.cell.width = 9
    hoisted.cell.height = 17
    vi.clearAllMocks()
  })

  function render(props: Partial<React.ComponentProps<typeof TerminalPanel>>) {
    flushSync(() => { root.render(React.createElement(TerminalPanel, makeBaseProps(props))) })
    return container.querySelector('.terminal-session-container') as HTMLElement
  }
  function mountSized(props: Partial<React.ComponentProps<typeof TerminalPanel>>) {
    const node = render(props)
    Object.defineProperty(node, 'clientWidth', { value: 900, configurable: true })
    Object.defineProperty(node, 'clientHeight', { value: 510, configurable: true })
    return node
  }
  const cb = <T,>(name: string) => hoisted.lastCallbacks?.[name] as T
  const grid = (cols: number, rows: number) => flushSync(() => { cb<(c: number, r: number) => void>('onResize')(cols, rows) })
  const open = () => flushSync(() => { cb<() => void>('onOpen')() })
  const nextFrame = () => new Promise<void>((r) => requestAnimationFrame(() => r()))
  const scaleOf = () => parseFloat(/^scale\(([\d.]+)\)/.exec(hoisted.lastElement!.style.transform)?.[1] ?? 'NaN')

  const kinds: Array<[string, Partial<React.ComponentProps<typeof TerminalPanel>>]> = [
    ['local tab', { remote: false }],
    ['peer tab (remote)', { remote: true }],
    ['browser viewer (wsURL)', { wsURL: 'wss://host/sessions/s/ws?cap=x' }],
  ]
  for (const [kind, props] of kinds) {
    it(`${kind}: onOpen reports the measured fit (100x30), not the 120x40 grid it shows`, () => {
      mountSized(props)
      grid(120, 40)
      expect(hoisted.termResizeCalls).toContainEqual([120, 40])
      open()
      expect(hoisted.sendResizeCalls).toEqual([[100, 30]])
      expect(hoisted.sendClaimCalls).toEqual([])
    })
  }

  it('local tab follows a 120x40 grid frame: term.resize(120, 40) and a scale below 1', () => {
    mountSized({ remote: false })
    grid(120, 40)
    expect(hoisted.termResizeCalls).toContainEqual([120, 40])
    expect(scaleOf()).toBeLessThan(1)
    expect(scaleOf()).toBeCloseTo(0.75, 3) // min(900/1080, 510/680)
  })

  it('local tab with no grid frame yet fits itself and reports (the hub bootstraps from it)', async () => {
    mountSized({ remote: false })
    await nextFrame() // the activation fit runs in a rAF
    expect(hoisted.termResizeCalls).toContainEqual([100, 30])
    expect(hoisted.lastElement!.style.transform).toBe('')
    open()
    expect(hoisted.sendResizeCalls).toContainEqual([100, 30])
  })

  it('a font size change on the active panel claims its new fit once, without resizing xterm before the next grid frame', () => {
    mountSized({ remote: true, fontSize: 14 })
    grid(100, 30)
    hoisted.termResizeCalls = []
    hoisted.cell.width = 10 // a bigger font: the fit is now 90x30
    render({ remote: true, fontSize: 15 })
    expect(hoisted.sendClaimCalls).toEqual([[90, 30]])
    expect(hoisted.termResizeCalls).toEqual([])
    render({ remote: true, fontSize: 15 }) // unchanged font: nothing
    expect(hoisted.sendClaimCalls).toEqual([[90, 30]])
    grid(90, 30) // the claim's echo: the panel follows
    expect(hoisted.termResizeCalls).toEqual([[90, 30]])
  })
})

// ─── CSS gate (style.css source inspection) ───────────────────────────────────

describe('Phase 157 style.css — guest scale CSS', () => {
  it('style.css contains transform-origin: top left for .xterm', () => {
    const { readFileSync } = require('node:fs')
    const { resolve, dirname } = require('node:path')
    const { fileURLToPath } = require('node:url')
    const __dir = dirname(fileURLToPath(import.meta.url))
    const css = readFileSync(resolve(__dir, '../../style.css'), 'utf-8')
    // Find the .xterm rule and verify it contains transform-origin: top left
    const xtermRuleIdx = css.indexOf('.xterm {')
    expect(xtermRuleIdx).toBeGreaterThan(-1)
    const ruleBlock = css.slice(xtermRuleIdx, css.indexOf('}', xtermRuleIdx) + 1)
    expect(ruleBlock).toContain('transform-origin: top left')
  })

  // BUG-01: the horizontal-scroll fallback class must enable overflow-x.
  it('style.css contains .terminal-guest--scroll-x with overflow-x: auto', () => {
    const { readFileSync } = require('node:fs')
    const { resolve, dirname } = require('node:path')
    const { fileURLToPath } = require('node:url')
    const __dir = dirname(fileURLToPath(import.meta.url))
    const css = readFileSync(resolve(__dir, '../../style.css'), 'utf-8')
    const ruleIdx = css.indexOf('.terminal-guest--scroll-x')
    expect(ruleIdx).toBeGreaterThan(-1)
    const ruleBlock = css.slice(ruleIdx, css.indexOf('}', ruleIdx) + 1)
    expect(ruleBlock).toContain('overflow-x: auto')
  })
})
