/**
 * Portty 04-06 (SHAR-04) + Phase 5 (SHAR-07) — guest close path of TerminalPanel.
 *
 * Only final closes reach onClose now: share-stopped shows the stopped-sharing
 * banner with the local host label and calls onShareStopped, session-ended shows
 * the session-ended banner, and either way xterm stdin is disabled (the view keeps
 * its scrollback and takes no more input). A network drop reconnects instead
 * (RelayClient onStateChange): the reconnecting banner appears after 3 s and goes
 * away on 'open', which re-enables stdin. Mocks copied from TerminalPanel.contextLoss.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

const hoisted = vi.hoisted(() => ({
  state: {
    onClose: null as ((code: number, reason: string) => void) | null,
    onStateChange: null as ((state: string) => void) | null,
    onReset: null as (() => void) | null,
    onConnectStuck: null as ((stuck: boolean) => void) | null,
    term: null as { options: Record<string, unknown> } | null,
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
    public write = vi.fn()
    public dispose = vi.fn()
    public clearTextureAtlas = vi.fn()
    public refresh = vi.fn()
    public resize = vi.fn()
    constructor(_opts: Record<string, unknown>) {
      hoisted.state.term = this as unknown as { options: Record<string, unknown> }
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

// Stub the other addons the hot-swap effect may construct/dispose.
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

// Stub RelayClient (it tries to open a real WebSocket).
vi.mock('../../lib/relayClient', () => ({
  RelayClient: class {
    constructor(_p: number, _id: string, cbs: {
      onClose?: (code: number, reason: string) => void
      onStateChange?: (state: string) => void
      onReset?: () => void
      onConnectStuck?: (stuck: boolean) => void
    }) {
      hoisted.state.onClose = cbs.onClose ?? null
      hoisted.state.onStateChange = cbs.onStateChange ?? null
      hoisted.state.onReset = cbs.onReset ?? null
      hoisted.state.onConnectStuck = cbs.onConnectStuck ?? null
    }
    sendInput() {}
    sendResize() {}
    sendClaim() {} // INPT-06
    resumePoint() { return null } // HARD-05: no offset known, so no tab-switch snapshot
    close() {}
  },
  MSG_OUTPUT: 0x01,
  MSG_RESIZE: 0x02,
  MSG_INPUT: 0x10,
  MSG_RESIZE2: 0x11,
  MSG_PING: 0x12,
  RECONNECT: { bannerDelayMs: 3000 },
  encodeInputFrames: () => [],
  encodeResizeFrame: () => new Uint8Array(),
  parseServerFrame: () => ({ type: 0 }),
}))

vi.mock('../../lib/webglProbe', () => ({
  isSoftwareWebGL: () => false,
}))

vi.mock('../../wailsjs/go/main/App', () => ({
  SetSearchConfig: () => Promise.resolve(),
}))

vi.mock('../../wailsjs/go/models', () => ({
  daemon: {
    SearchConfig: class { constructor(_o: unknown) {} },
  },
}))

// jsdom doesn't ship ResizeObserver.
class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof MockResizeObserver }).ResizeObserver = MockResizeObserver

// ---------- SUT ----------
// Import AFTER the vi.mock() calls above so the SUT picks up the mocks.
import { TerminalPanel } from '../TerminalPanel'
vi.mock('@xterm/addon-webgl', () => ({
  WebglAddon: class { activate(_t: unknown) {} dispose() {} onContextLoss() { return { dispose: () => {} } } },
}))

describe('TerminalPanel guest close (SHAR-04)', () => {
  let container: HTMLElement
  let root: Root
  // vi.clearAllMocks in afterEach resets its calls between cases.
  const onSessionEnded = vi.fn()

  beforeEach(() => {
    hoisted.state.onClose = null
    hoisted.state.onStateChange = null
    hoisted.state.onReset = null
    hoisted.state.onConnectStuck = null
    hoisted.state.term = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.clearAllMocks()
    vi.useRealTimers()
  })

  function mountGuest(onShareStopped: () => void, hostLabel = 'office-air') {
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, remote: true,
        hostLabel, onShareStopped, onSessionEnded,
      }))
    })
    expect(hoisted.state.onClose, 'RelayClient got an onClose callback').not.toBeNull()
  }

  function close(reason: string) {
    flushSync(() => { hoisted.state.onClose!(1000, reason) })
  }

  it("'share stopped' shows '<host> stopped sharing this session.', calls onShareStopped and disables stdin", () => {
    const onShareStopped = vi.fn()
    mountGuest(onShareStopped)
    close('share stopped')
    expect(container.textContent).toContain('office-air stopped sharing this session.')
    expect(container.textContent).not.toContain('Session ended')
    expect(onShareStopped).toHaveBeenCalledTimes(1)
    expect(onSessionEnded).not.toHaveBeenCalled()
    expect(hoisted.state.term!.options.disableStdin).toBe(true)
  })

  it("'session ended' keeps the session-ended banner, does not call onShareStopped and disables stdin", () => {
    const onShareStopped = vi.fn()
    mountGuest(onShareStopped)
    close('session ended')
    expect(container.textContent).toContain('Session ended — the owner stopped this session.')
    expect(onShareStopped).not.toHaveBeenCalled()
    expect(onSessionEnded).toHaveBeenCalledTimes(1)
    expect(hoisted.state.term!.options.disableStdin).toBe(true)
  })

  it("'other account' calls onOtherAccount, disables stdin and shows no ended banner (MOB-05)", () => {
    const onShareStopped = vi.fn()
    const onOtherAccount = vi.fn()
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, remote: true,
        hostLabel: 'office-air', onShareStopped, onOtherAccount, onSessionEnded,
      }))
    })
    close('other account')
    expect(onOtherAccount).toHaveBeenCalledTimes(1)
    expect(onShareStopped).not.toHaveBeenCalled()
    expect(onSessionEnded).not.toHaveBeenCalled()
    expect(container.textContent).not.toContain('stopped sharing')
    expect(container.textContent).not.toContain('Session ended')
    expect(hoisted.state.term!.options.disableStdin).toBe(true)
  })

  it('onConnectStuck is kept as connectStuck on the panel root (MOB-03, read by 14-08)', () => {
    mountGuest(vi.fn())
    const root0 = container.querySelector('.terminal-session-container')!
    expect(root0.getAttribute('data-connect-stuck')).toBeNull()
    flushSync(() => { hoisted.state.onConnectStuck!(true) })
    expect(root0.getAttribute('data-connect-stuck')).toBe('true')
    flushSync(() => { hoisted.state.onConnectStuck!(false) })
    expect(root0.getAttribute('data-connect-stuck')).toBeNull()
  })

  // Phase 14 (MOB-03): Reload only on a phone ((pointer: coarse) and (hover: none)).
  describe('Reload (MOB-03)', () => {
    const realMatchMedia = window.matchMedia
    const phoneMedia = (phone: boolean) => {
      window.matchMedia = ((media: string) => ({
        matches: phone && media === '(pointer: coarse) and (hover: none)', media,
        addEventListener() {}, removeEventListener() {},
      })) as unknown as typeof window.matchMedia
    }
    const reloadBtn = () => Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Reload')
    afterEach(() => { window.matchMedia = realMatchMedia })

    it("a phone's share-stopped banner carries Reload", () => {
      phoneMedia(true)
      mountGuest(vi.fn())
      close('share stopped')
      expect(container.textContent).toContain('office-air stopped sharing this session.')
      expect(reloadBtn()).not.toBeUndefined()
    })

    it('without the phone query there is no Reload', () => {
      phoneMedia(false)
      mountGuest(vi.fn())
      close('share stopped')
      expect(container.textContent).toContain('office-air stopped sharing this session.')
      expect(reloadBtn()).toBeUndefined()
    })
  })

  it("'share stopped' with an empty host label (web page, host unknown) shows the fallback", () => {
    mountGuest(vi.fn(), '')
    close('share stopped')
    expect(container.textContent).toContain('This session is no longer shared.')
  })

  describe('network drop (SHAR-07)', () => {
    const BANNER = 'Connection lost — reconnecting…'
    const setState = (st: string) => flushSync(() => { hoisted.state.onStateChange!(st) })
    const advance = (ms: number) => flushSync(() => { vi.advanceTimersByTime(ms) })

    it("'reconnecting' shows no banner at 2999 ms and the banner at 3000 ms; stdin disabled", () => {
      vi.useFakeTimers()
      mountGuest(vi.fn())
      setState('reconnecting')
      expect(hoisted.state.term!.options.disableStdin).toBe(true)
      advance(2999)
      expect(container.textContent).not.toContain(BANNER)
      advance(1)
      expect(container.textContent).toContain(BANNER)
      expect(container.querySelector('[aria-label="Dismiss"]')).toBeNull()
    })

    it("'open' removes the banner and re-enables stdin", () => {
      vi.useFakeTimers()
      mountGuest(vi.fn())
      setState('reconnecting')
      advance(3000)
      setState('open')
      expect(container.textContent).not.toContain(BANNER)
      expect(hoisted.state.term!.options.disableStdin).toBe(false)
    })

    it("'open' before 3 s cancels the banner (a blip stays silent)", () => {
      vi.useFakeTimers()
      mountGuest(vi.fn())
      setState('reconnecting')
      advance(1000)
      setState('open')
      advance(10_000)
      expect(container.textContent).not.toContain(BANNER)
    })
  })
})
