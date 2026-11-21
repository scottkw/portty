/**
 * Phase 11 (PREV-01): every TerminalPanel (local tab, peer tab, browser viewer) runs one
 * preview assembler per RelayClient and reports its events through onPreview. A dropped
 * socket (onPreviewReset) or an unmount fails a partial preview as incomplete.
 * RelayClient is mocked to capture its callbacks; xterm mocks copied from TerminalPanel.guestClose.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'
import type { PreviewFrame } from '../../lib/relayClient'
import type { PreviewEvent } from '../../lib/previewAssembler'

const hoisted = vi.hoisted(() => ({
  state: {
    onPreview: null as ((f: PreviewFrame) => void) | null,
    onPreviewReset: null as (() => void) | null,
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
      this._core = {
        _renderService: { dimensions: { css: { cell: { width: 9, height: 17 } } }, clear: vi.fn() },
        _createRenderer: vi.fn(),
      }
    }
  }
  return { Terminal: MockTerminal }
})
vi.mock('@xterm/addon-fit', () => ({
  FitAddon: class { activate(_t: unknown) {} proposeDimensions() { return { cols: 80, rows: 24 } } },
}))
vi.mock('@xterm/addon-image', () => ({ ImageAddon: class { activate(_t: unknown) {} dispose() {} } }))
vi.mock('@xterm/addon-unicode11', () => ({ Unicode11Addon: class { activate(_t: unknown) {} } }))
vi.mock('@xterm/addon-clipboard', () => ({ ClipboardAddon: class { activate(_t: unknown) {} dispose() {} } }))
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
vi.mock('../../lib/relayClient', () => ({
  RelayClient: class {
    constructor(_p: number, _id: string, cbs: { onPreview?: (f: PreviewFrame) => void; onPreviewReset?: () => void }) {
      hoisted.state.onPreview = cbs.onPreview ?? null
      hoisted.state.onPreviewReset = cbs.onPreviewReset ?? null
    }
    sendInput() {}
    sendResize() {}
    sendClaim() {}
    resumePoint() { return null }
    close() {}
  },
  RECONNECT: { bannerDelayMs: 3000 },
}))
vi.mock('../../lib/webglProbe', () => ({ isSoftwareWebGL: () => false }))
vi.mock('../../wailsjs/go/main/App', () => ({ SetSearchConfig: () => Promise.resolve() }))
vi.mock('../../wailsjs/go/models', () => ({ daemon: { SearchConfig: class { constructor(_o: unknown) {} } } }))

class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof MockResizeObserver }).ResizeObserver = MockResizeObserver

import { TerminalPanel } from '../TerminalPanel'

const KEY = '0123456789abcdef0123456789abcdef'
const header: PreviewFrame = { kind: 'header', id: 3, name: 'a.pdf', mime: 'application/pdf', size: 4, key: KEY, focus: false }
const chunk = (n: number): PreviewFrame => ({ kind: 'chunk', id: 3, payload: new Uint8Array(n) })

const viewers = [
  ['local tab', {}],
  ['peer tab', { remote: true }],
  ['browser viewer', { wsURL: 'wss://host/sessions/s1/ws?cap=x' }],
] as const

describe('TerminalPanel onPreview (PREV-01)', () => {
  let container: HTMLElement
  let root: Root
  let events: PreviewEvent[]
  let mounted: boolean

  beforeEach(() => {
    hoisted.state.onPreview = null
    hoisted.state.onPreviewReset = null
    events = []
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    mounted = false
  })

  afterEach(() => {
    if (mounted) flushSync(() => root.unmount())
    container.remove()
  })

  function mount(extra: object) {
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, onPreview: (e: PreviewEvent) => events.push(e), ...extra,
      }))
    })
    mounted = true
    expect(hoisted.state.onPreview, 'RelayClient got onPreview').not.toBeNull()
    expect(hoisted.state.onPreviewReset, 'RelayClient got onPreviewReset').not.toBeNull()
  }

  for (const [label, extra] of viewers) {
    it(`${label}: frames on the socket reach onPreview as start, progress and complete`, () => {
      mount(extra)
      hoisted.state.onPreview!(header)
      hoisted.state.onPreview!(chunk(4))
      hoisted.state.onPreview!({ kind: 'end', id: 3, status: 0 })
      expect(events.map((e) => e.type)).toEqual(['start', 'progress', 'complete'])
      const done = events[2] as Extract<PreviewEvent, { type: 'complete' }>
      expect(done.blob.size).toBe(4)
      expect(done.blob.type).toBe('application/pdf')
    })
  }

  it('a socket reset mid-stream fails the preview as incomplete', () => {
    mount({ remote: true })
    hoisted.state.onPreview!(header)
    hoisted.state.onPreview!(chunk(2))
    hoisted.state.onPreviewReset!()
    expect(events[events.length - 1]).toEqual({ type: 'failed', id: 3, reason: 'incomplete' })
  })

  it('unmounting mid-stream fails the preview as incomplete', () => {
    mount({})
    hoisted.state.onPreview!(header)
    flushSync(() => root.unmount())
    mounted = false
    expect(events[events.length - 1]).toEqual({ type: 'failed', id: 3, reason: 'incomplete' })
  })
})
