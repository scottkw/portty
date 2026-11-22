/**
 * Phase 12 (PATH-02, gap 1): a desktop peer tab (a remote WebShareSessionView) gets path links and URL
 * links from this Mac's own link settings, which App passes as `pluginConfig`, and never reads the host's
 * config: from the app's wails:// origin /api/plugin-config is cross-origin and the share server sends no
 * CORS header. WebShareSessionView is mounted for real; RelayClient and xterm are mocked as in
 * TerminalPanel.pathClick.test.tsx, with a counter on the WebLinksAddon constructor.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

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
    webLinksCtor: 0,
    webLinksHandler: null as null | ((e: MouseEvent, uri: string) => void),
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
  WebLinksAddon: class { constructor(h?: (e: MouseEvent, uri: string) => void, _o?: unknown) { hoisted.state.webLinksCtor++; hoisted.state.webLinksHandler = h ?? null } activate(_t: unknown) {} dispose() {} },
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

import { WebShareSessionView } from '../WebShareSessionView'

const PEER = 'https://office-air.tail1234.ts.net:7444'
const linkSettings = { webLinks: true, webLinksConfig: { modifier: 'platform' } }

describe('desktop peer tab links (PATH-02 gap 1)', () => {
  let container: HTMLElement
  let root: Root
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')
  const fetchMock = vi.fn()
  const eventSource = vi.fn()

  beforeEach(() => {
    Object.assign(hoisted.state, { cbs: null, sent: [], provider: null, registered: 0, disposed: 0, rows: ['docs/diagram.png  notes.md'], webLinksCtor: 0, webLinksHandler: null })
    Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true })
    fetchMock.mockReset().mockRejectedValue(new TypeError('cross-origin request refused'))
    eventSource.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('EventSource', class { constructor(url: string) { eventSource(url) } addEventListener() {} close() {} })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(async () => {
    await act(async () => root.unmount())
    container.remove()
    vi.unstubAllGlobals()
    if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform)
  })

  async function mount(props: object) {
    await act(async () => {
      root.render(React.createElement(WebShareSessionView, {
        sessionId: 's1', capToken: 'CAP', relayPort: 1234, ...props,
      } as never))
    })
    await act(async () => { await Promise.resolve() })
  }

  const peerProps = (extra: object = {}) => ({ remote: true, baseURL: PEER, ...extra })
  const configFetches = () => fetchMock.mock.calls.filter((c) => String(c[0]).includes('/api/plugin-config'))
  const links = () => {
    let out: Link[] | undefined
    hoisted.state.provider!.provideLinks(1, (l) => { out = l })
    return out
  }

  it("a peer tab with this Mac's link settings registers path links and URL links and fetches nothing cross-origin", async () => {
    await mount(peerProps({ pluginConfig: linkSettings }))
    expect(hoisted.state.registered).toBe(1)
    expect(hoisted.state.webLinksCtor).toBe(1)
    expect(configFetches()).toEqual([])
    expect(eventSource).not.toHaveBeenCalled()
  })

  it('a peer tab never requests /api/plugin-config or its stream, even with no config', async () => {
    await mount(peerProps())
    expect(configFetches()).toEqual([])
    expect(eventSource).not.toHaveBeenCalled()
    expect(hoisted.state.registered).toBe(0)
    expect(hoisted.state.webLinksCtor).toBe(0)
  })

  it('a modifier-click in a peer tab sends the path once the host offers path clicks', async () => {
    await mount(peerProps({ pluginConfig: linkSettings }))
    expect(links()).toBeUndefined() // the host has not said path clicks are available yet
    act(() => hoisted.state.cbs!.onPathClickCapability!(true))
    const [link] = links()!
    link.activate(new MouseEvent('mouseup', { metaKey: true }), link.text)
    expect(hoisted.state.sent).toEqual(['docs/diagram.png'])
  })

  it('the browser viewer still reads the host config from its own origin', async () => {
    fetchMock.mockImplementation(() => Promise.resolve({ ok: true, json: () => Promise.resolve({ webLinks: true }) }))
    await mount({}) // not remote, no baseURL, no config
    await act(async () => { await Promise.resolve() })
    await act(async () => { await Promise.resolve() })
    expect(configFetches().map((c) => String(c[0]))).toEqual([`${window.location.origin}/api/plugin-config?cap=CAP`])
    expect(eventSource).toHaveBeenCalledTimes(1)
    expect(hoisted.state.registered).toBe(1)
    expect(hoisted.state.webLinksCtor).toBe(1)
  })
})
