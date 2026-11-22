/**
 * Phase 12 (PATH-02): a modifier-click on path-like text in a TerminalPanel sends one path-click
 * frame through the session's RelayClient, only after the host said path clicks are available on
 * this socket. RelayClient and xterm are mocked; the MockTerminal captures the link provider and
 * serves a fake buffer. Mocks follow TerminalPanel.preview.test.tsx.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'

type Link = { range: unknown; text: string; activate: (e: MouseEvent, text: string) => void }
type Provider = { provideLinks: (y: number, cb: (links: Link[] | undefined) => void) => void }

const hoisted = vi.hoisted(() => ({
  state: {
    cbs: null as null | {
      onPathClickCapability?: (enabled: boolean) => void
      onPathNotice?: (kind: 'path-unsupported' | 'path-too-large') => void
      onStateChange?: (s: string) => void
      onPasteInterrupted?: (i: { sentBytes: number | null; totalBytes: number }) => void
      onClose?: (code?: number, reason?: string) => void
    },
    sent: [] as string[],
    provider: null as Provider | null,
    registered: 0,
    disposed: 0,
    rows: [] as string[],
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
    public registerLinkProvider = vi.fn((p: Provider) => {
      hoisted.state.provider = p
      hoisted.state.registered++
      return { dispose: () => { hoisted.state.disposed++; hoisted.state.provider = null } }
    })
    public buffer = {
      active: {
        getLine: (i: number) => {
          const row = hoisted.state.rows[i]
          if (row === undefined) return undefined
          return {
            isWrapped: false,
            length: 80,
            getCell: (x: number) => ({ getChars: () => row[x] ?? '', getWidth: () => 1 }),
          }
        },
      },
    }
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
    constructor(_p: number, _id: string, cbs: NonNullable<typeof hoisted.state.cbs>) {
      hoisted.state.cbs = cbs
    }
    sendInput() {}
    sendResize() {}
    sendClaim() {}
    sendPathClick(text: string) { hoisted.state.sent.push(text) }
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

const click = (mods: Partial<MouseEvent>) => ({ metaKey: false, ctrlKey: false, ...mods }) as MouseEvent

describe('path link gating (PATH-02)', () => {
  let container: HTMLElement
  let root: Root
  let mounted: boolean
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')

  beforeEach(() => {
    Object.assign(hoisted.state, { cbs: null, sent: [], provider: null, registered: 0, disposed: 0, rows: ['docs/diagram.png  notes.md'] })
    Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    mounted = false
  })

  afterEach(() => {
    if (mounted) flushSync(() => root.unmount())
    container.remove()
    if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform)
  })

  function render(extra: object = {}) {
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, pluginConfig: { webLinks: true }, ...extra,
      } as never))
    })
    mounted = true
  }

  const links = () => {
    let out: Link[] | undefined
    hoisted.state.provider!.provideLinks(1, (l) => { out = l })
    return out
  }

  it('offers no links until the host says path clicks are available, and none after the socket goes', () => {
    render()
    expect(hoisted.state.registered).toBe(1)
    expect(links()).toBeUndefined()
    flushSync(() => hoisted.state.cbs!.onPathClickCapability!(true))
    expect(links()!.map((l) => l.text)).toEqual(['docs/diagram.png'])
    hoisted.state.cbs!.onPathClickCapability!(false)
    expect(links()).toBeUndefined()
  })

  it('a modifier-click sends the path to the host; a plain click sends nothing', () => {
    render()
    hoisted.state.cbs!.onPathClickCapability!(true)
    const [link] = links()!
    link.activate(click({}), link.text)
    expect(hoisted.state.sent).toEqual([])
    link.activate(click({ metaKey: true }), link.text)
    expect(hoisted.state.sent).toEqual(['docs/diagram.png'])
  })

  it('turning web links off disposes the provider; unmount disposes it', () => {
    render()
    expect(hoisted.state.provider).not.toBeNull()
    render({ pluginConfig: { webLinks: false } })
    expect(hoisted.state.provider).toBeNull()
    expect(hoisted.state.disposed).toBe(1)
    render({ pluginConfig: { webLinks: true } })
    expect(hoisted.state.registered).toBe(2)
    flushSync(() => root.unmount())
    mounted = false
    expect(hoisted.state.disposed).toBe(2)
  })
})
