/**
 * Portty 04-06 (SHAR-04) — guest close path of TerminalPanel.
 *
 * A guest socket closing with the share-stopped reason shows the stopped-sharing
 * banner with the local host label and calls onShareStopped; any other close keeps
 * the session-ended banner. Either way xterm stdin is disabled (the view keeps its
 * scrollback and takes no more input). Mocks copied from TerminalPanel.contextLoss.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

const hoisted = vi.hoisted(() => ({
  state: {
    onClose: null as ((code: number, reason: string) => void) | null,
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
    constructor(_p: number, _id: string, cbs: { onClose?: (code: number, reason: string) => void }) { hoisted.state.onClose = cbs.onClose ?? null }
    sendInput() {}
    sendResize() {}
    close() {}
  },
  MSG_OUTPUT: 0x01,
  MSG_RESIZE: 0x02,
  MSG_INPUT: 0x10,
  MSG_RESIZE2: 0x11,
  MSG_PING: 0x12,
  encodeInputFrame: () => new Uint8Array(),
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

  beforeEach(() => {
    hoisted.state.onClose = null
    hoisted.state.term = null
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    try { flushSync(() => root.unmount()) } catch { /* ignore */ }
    container.remove()
    vi.clearAllMocks()
  })

  function mountGuest(onShareStopped: () => void) {
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, remote: true,
        hostLabel: 'office-air', onShareStopped,
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
    expect(hoisted.state.term!.options.disableStdin).toBe(true)
  })

  it("'session ended' keeps the session-ended banner, does not call onShareStopped and disables stdin", () => {
    const onShareStopped = vi.fn()
    mountGuest(onShareStopped)
    close('session ended')
    expect(container.textContent).toContain('Session ended — the owner stopped this session.')
    expect(onShareStopped).not.toHaveBeenCalled()
    expect(hoisted.state.term!.options.disableStdin).toBe(true)
  })
})
