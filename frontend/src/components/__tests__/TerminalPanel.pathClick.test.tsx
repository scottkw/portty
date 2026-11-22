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
import terminalPanelSource from '../TerminalPanel.tsx?raw'

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

  // 12-08 gap 2: xterm's Linkifier caches a row's answer, so every capability report must refresh it.
  it('the capability callback refreshes the link state', () => {
    const cb = /onPathClickCapability:\s*\(enabled\)\s*=>\s*\{([^}]*)\}/.exec(terminalPanelSource)
    expect(cb, 'onPathClickCapability callback found').not.toBeNull()
    expect(cb![1]).toContain('pathClickEnabledRef.current = enabled')
    expect(cb![1]).toContain('pathLinksRef.current?.invalidate()')
  })
})

const COPY = {
  'path-unsupported': "Can't preview this file type. Portty previews images, PDFs and Markdown files.",
  'path-too-large': 'Too large to preview. The limit is 20 MB.',
} as const

describe('path click adds no announcement (PATH-02)', () => {
  let container: HTMLElement
  let root: Root
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')

  beforeEach(() => {
    Object.assign(hoisted.state, { cbs: null, sent: [], provider: null, registered: 0, disposed: 0, rows: ['docs/diagram.png'] })
    Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, pluginConfig: { webLinks: true },
      } as never))
    })
  })
  afterEach(() => {
    flushSync(() => root.unmount())
    container.remove()
    if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform)
  })

  it('sending a click adds no aria-live element; only an arriving notice renders one, the banner itself', () => {
    const live = () => container.querySelectorAll('[aria-live]').length
    const before = live()
    hoisted.state.cbs!.onPathClickCapability!(true)
    let links: Link[] | undefined
    hoisted.state.provider!.provideLinks(1, (l) => { links = l })
    links![0].activate(click({ metaKey: true }), links![0].text)
    expect(hoisted.state.sent).toEqual(['docs/diagram.png'])
    expect(live()).toBe(before)
    flushSync(() => hoisted.state.cbs!.onPathNotice!('path-unsupported'))
    expect(live()).toBe(before + 1)
    expect(container.querySelector('[aria-live]')?.getAttribute('role')).toBe('status')
  })
})

describe('path notice (PATH-02)', () => {
  let container: HTMLElement
  let root: Root
  let mounted: boolean

  beforeEach(() => {
    vi.useFakeTimers()
    Object.assign(hoisted.state, { cbs: null, sent: [], provider: null, registered: 0, disposed: 0, rows: [] })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    mounted = false
  })
  afterEach(() => {
    if (mounted) flushSync(() => root.unmount())
    container.remove()
    vi.useRealTimers()
  })

  function render(extra: object = {}) {
    flushSync(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, ...extra,
      } as never))
    })
    mounted = true
  }
  const notice = (k: keyof typeof COPY) => flushSync(() => hoisted.state.cbs!.onPathNotice!(k))
  const shown = () => container.querySelector('.banner__message')?.textContent ?? null
  const tick = (ms: number) => flushSync(() => { vi.advanceTimersByTime(ms) })

  it('shows the fixed copy and goes after 5000 ms', () => {
    render()
    notice('path-too-large')
    expect(shown()).toBe(COPY['path-too-large'])
    tick(4999)
    expect(shown()).toBe(COPY['path-too-large'])
    tick(1)
    expect(shown()).toBeNull()
  })

  it('a second notice replaces the first and restarts the timer; they never stack', () => {
    render()
    notice('path-unsupported')
    tick(3000)
    notice('path-too-large')
    expect(container.querySelectorAll('.banner')).toHaveLength(1)
    expect(shown()).toBe(COPY['path-too-large'])
    tick(4000) // 7000 ms after the first
    expect(shown()).toBe(COPY['path-too-large'])
    tick(1000) // 8000 ms
    expect(shown()).toBeNull()
  })

  it('Dismiss removes it at once', () => {
    render()
    notice('path-unsupported')
    flushSync(() => { (container.querySelector('[aria-label="Dismiss"]') as HTMLButtonElement).click() })
    expect(shown()).toBeNull()
  })

  it('is dropped, not queued, while the paste-interrupted banner holds the slot', () => {
    render()
    flushSync(() => hoisted.state.cbs!.onPasteInterrupted!({ sentBytes: 10, totalBytes: 50_000 }))
    expect(shown()).toContain('Paste interrupted')
    notice('path-unsupported')
    expect(shown()).toContain('Paste interrupted')
    flushSync(() => { (container.querySelector('[aria-label="Dismiss"]') as HTMLButtonElement).click() })
    expect(shown()).toBeNull()
  })

  it('is dropped while the reconnecting banner shows and does not appear after it goes', () => {
    render()
    flushSync(() => hoisted.state.cbs!.onStateChange!('reconnecting'))
    tick(3000) // RECONNECT.bannerDelayMs in the mock
    expect(shown()).toContain('reconnecting')
    notice('path-too-large')
    expect(shown()).toContain('reconnecting')
    flushSync(() => hoisted.state.cbs!.onStateChange!('open'))
    expect(shown()).toBeNull()
  })

  it('is removed when another banner takes the slot', () => {
    render()
    notice('path-unsupported')
    flushSync(() => hoisted.state.cbs!.onPasteInterrupted!({ sentBytes: 10, totalBytes: 50_000 }))
    expect(shown()).toContain('Paste interrupted')
    flushSync(() => { (container.querySelector('[aria-label="Dismiss"]') as HTMLButtonElement).click() })
    expect(shown()).toBeNull()
  })

  it('is removed when the pane becomes inactive and never takes focus', () => {
    render()
    const focused = document.activeElement
    notice('path-unsupported')
    expect(document.activeElement).toBe(focused)
    render({ isActive: false })
    expect(shown()).toBeNull()
  })
})
