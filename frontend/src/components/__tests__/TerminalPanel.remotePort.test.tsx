/**
 * Phase 13 (URL-01..03): a modifier-click on a printed loopback URL in a desktop peer tab goes through the
 * OpenRemotePort binding; only the binding's own answer is ever opened; the consent sheet and the five
 * fixed notices; every other link keeps today's path. xterm, the addons, RelayClient and the App binding
 * are mocked (mocks follow TerminalPanel.pathClick.test.tsx); the WebLinksAddon mock hands over the click
 * handler the panel builds.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'

type Handler = (event: MouseEvent, uri: string) => void
type PortResult = { status: string; url: string; host: string; port: number; errorCode: string }

const hoisted = vi.hoisted(() => ({
  state: { handler: null as null | ((event: MouseEvent, uri: string) => void) },
  openRemotePort: vi.fn(),
  openLink: vi.fn(),
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
    public registerLinkProvider = vi.fn(() => ({ dispose: () => {} }))
    public buffer = { active: { getLine: () => undefined } }
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
  WebLinksAddon: class {
    constructor(h: (event: MouseEvent, uri: string) => void, _o?: unknown) { hoisted.state.handler = h }
    activate(_t: unknown) {}
    dispose() {}
  },
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
    sendInput() {}
    sendResize() {}
    sendClaim() {}
    sendPathClick() {}
    resumePoint() { return null }
    close() {}
  },
  RECONNECT: { bannerDelayMs: 3000 },
}))
vi.mock('../../lib/webglProbe', () => ({ isSoftwareWebGL: () => false }))
vi.mock('../../wailsjs/go/main/App', () => ({
  SetSearchConfig: () => Promise.resolve(),
  OpenRemotePort: (...args: unknown[]) => hoisted.openRemotePort(...args),
}))
vi.mock('../../wailsjs/go/models', () => ({ daemon: { SearchConfig: class { constructor(_o: unknown) {} } } }))
vi.mock('../../lib/openLink', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/openLink')>()),
  openLink: (url: string) => hoisted.openLink(url),
}))

class MockResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
;(globalThis as unknown as { ResizeObserver: typeof MockResizeObserver }).ResizeObserver = MockResizeObserver
;(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true

import { TerminalPanel } from '../TerminalPanel'

const NOTICE_MS = 5000
const CONSENT_BODY = 'Pages from this port open in your browser through Portty over your tailnet. Other ports stay closed.'
const META = { metaKey: true, ctrlKey: false } as MouseEvent
const PLAIN = { metaKey: false, ctrlKey: false } as MouseEvent

const answer = (over: Partial<PortResult>): PortResult => ({ status: '', url: '', host: '', port: 0, errorCode: '', ...over })
const opened = (port: number) => answer({ status: 'open', url: `http://localhost:${port}/`, host: 'office-air', port })
const consentFor = (port: number) => answer({ status: 'consent', host: 'office-air', port })

describe('desktop peer tab: remote localhost URLs (URL-01, URL-02, URL-03)', () => {
  let container: HTMLElement
  let root: Root
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')

  beforeEach(() => {
    hoisted.state.handler = null
    hoisted.openRemotePort.mockReset()
    hoisted.openLink.mockReset()
    Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true })
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
  })

  afterEach(() => {
    act(() => root.unmount())
    container.remove()
    vi.useRealTimers()
    if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform)
  })

  function render(extra: object = { remote: true }) {
    act(() => {
      root.render(React.createElement(TerminalPanel, {
        sessionId: 's1', isActive: true, relayPort: 1234, fontSize: 13,
        onFontSizeChange: () => {}, theme: {}, hostLabel: 'office-air',
        pluginConfig: { webLinks: true, webLinksConfig: { modifier: 'cmd' } }, ...extra,
      } as never))
    })
  }

  const click = async (uri: string, event: MouseEvent = META) => {
    await act(async () => { (hoisted.state.handler as Handler)(event, uri) })
  }
  const notice = () => container.querySelector('[role="status"] .banner__message')?.textContent
  const dialog = () => document.querySelector('[role="dialog"]')
  const buttonNamed = (name: string) =>
    Array.from(document.querySelectorAll('[role="dialog"] button')).find((b) => b.textContent === name) as HTMLButtonElement

  describe('routing (URL-01)', () => {
    it('a meta-click on http://[::]:8000/) asks the binding once with the trimmed URL and opens only what it returns', async () => {
      hoisted.openRemotePort.mockResolvedValue(opened(8000))
      render()
      await click('http://[::]:8000/)')
      expect(hoisted.openRemotePort).toHaveBeenCalledTimes(1)
      expect(hoisted.openRemotePort).toHaveBeenCalledWith('s1', 'http://[::]:8000/', false)
      expect(hoisted.openLink).toHaveBeenCalledTimes(1)
      expect(hoisted.openLink).toHaveBeenCalledWith('http://localhost:8000/')
    })

    it('a click without the modifier calls nothing', async () => {
      render()
      await click('http://localhost:8000/', PLAIN)
      expect(hoisted.openRemotePort).not.toHaveBeenCalled()
      expect(hoisted.openLink).not.toHaveBeenCalled()
    })

    it('each of the five loopback forms goes through the binding', async () => {
      hoisted.openRemotePort.mockResolvedValue(opened(3000))
      render()
      for (const host of ['localhost', '127.0.0.1', '[::1]', '0.0.0.0', '[::]']) await click(`http://${host}:3000`)
      expect(hoisted.openRemotePort.mock.calls.map((c) => c[1])).toEqual([
        'http://localhost:3000', 'http://127.0.0.1:3000', 'http://[::1]:3000', 'http://0.0.0.0:3000', 'http://[::]:3000',
      ])
    })
  })

  describe('consent sheet (URL-02)', () => {
    beforeEach(() => {
      hoisted.openRemotePort.mockResolvedValueOnce(consentFor(8000))
      render()
    })

    it('asks first in a portal sheet with Don\'t Allow focused', async () => {
      await click('http://localhost:8000/')
      const d = dialog()!
      expect(d.querySelector('.dialog__title')?.textContent).toBe("Allow access to office-air's localhost:8000?")
      expect(d.querySelector('.dialog__body p')?.textContent).toBe(CONSENT_BODY)
      expect(Array.from(d.querySelectorAll('button')).map((b) => b.textContent)).toEqual(["Don't Allow", 'Allow'])
      expect(document.activeElement).toBe(buttonNamed("Don't Allow"))
      expect(d.closest('.dialog-scrim')?.parentElement).toBe(document.body)
      expect(hoisted.openLink).not.toHaveBeenCalled()
    })

    it('Allow calls again with consent and opens the returned URL', async () => {
      await click('http://localhost:8000/')
      hoisted.openRemotePort.mockResolvedValueOnce(opened(8000))
      await act(async () => { buttonNamed('Allow').click() })
      expect(hoisted.openRemotePort).toHaveBeenCalledTimes(2)
      expect(hoisted.openRemotePort).toHaveBeenLastCalledWith('s1', 'http://localhost:8000/', true)
      expect(dialog()).toBeNull()
      expect(hoisted.openLink).toHaveBeenCalledTimes(1)
      expect(hoisted.openLink).toHaveBeenCalledWith('http://localhost:8000/')
    })

    it("Don't Allow closes, calls nothing more, opens nothing and shows no notice", async () => {
      await click('http://localhost:8000/')
      await act(async () => { buttonNamed("Don't Allow").click() })
      expect(dialog()).toBeNull()
      expect(hoisted.openRemotePort).toHaveBeenCalledTimes(1)
      expect(hoisted.openLink).not.toHaveBeenCalled()
      expect(notice()).toBeUndefined()
    })

    it('Esc does the same as Don\'t Allow', async () => {
      await click('http://localhost:8000/')
      await act(async () => {
        document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      })
      expect(dialog()).toBeNull()
      expect(hoisted.openRemotePort).toHaveBeenCalledTimes(1)
      expect(hoisted.openLink).not.toHaveBeenCalled()
      expect(notice()).toBeUndefined()
    })

    it('Allow then port_in_use closes the sheet, shows the notice and opens nothing', async () => {
      await click('http://localhost:8000/')
      hoisted.openRemotePort.mockResolvedValueOnce(answer({ errorCode: 'port_in_use' }))
      await act(async () => { buttonNamed('Allow').click() })
      expect(dialog()).toBeNull()
      expect(notice()).toBe('Port 8000 is in use on this Mac.')
      expect(hoisted.openLink).not.toHaveBeenCalled()
    })

    it('Allow answered with consent again ends in a notice, not another sheet', async () => {
      await click('http://localhost:8000/')
      hoisted.openRemotePort.mockResolvedValueOnce(consentFor(8000))
      await act(async () => { buttonNamed('Allow').click() })
      expect(dialog()).toBeNull()
      expect(notice()).toBe("Couldn't reach office-air.")
      expect(hoisted.openLink).not.toHaveBeenCalled()
    })
  })

  describe('notices (URL-02)', () => {
    it.each([
      ['host_outdated', 'office-air needs a Portty update to open its local pages.'],
      ['daemon_outdated', "Portty's background service on this Mac is too old to open remote pages. Relaunch Portty to update it."],
      ['port_in_use', 'Port 8000 is in use on this Mac.'],
      ['refused', 'office-air refused access to port 8000.'],
      ['unreachable', "Couldn't reach office-air."],
      ['no_cap', "Couldn't reach office-air."],
      ['bad_url', "Couldn't reach office-air."],
      ['something_new', "Couldn't reach office-air."],
    ])('errorCode %s shows its fixed notice for %i ms and opens nothing', async (errorCode, copy) => {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      hoisted.openRemotePort.mockResolvedValue(answer({ errorCode }))
      render()
      await click('http://localhost:8000/')
      expect(notice()).toBe(copy)
      expect(container.querySelector('[role="status"]')?.getAttribute('aria-live')).toBe('polite')
      expect(container.querySelector('[aria-label="Dismiss"]')).not.toBeNull()
      act(() => { vi.advanceTimersByTime(NOTICE_MS) })
      expect(notice()).toBeUndefined()
      expect(hoisted.openLink).not.toHaveBeenCalled()
    })

    it('a rejected binding is the unreachable notice', async () => {
      hoisted.openRemotePort.mockRejectedValue(new Error('boom'))
      render()
      await click('http://localhost:8000/')
      expect(notice()).toBe("Couldn't reach office-air.")
      expect(hoisted.openLink).not.toHaveBeenCalled()
    })

    it('a notice shows in the clicking pane only', async () => {
      hoisted.openRemotePort.mockResolvedValue(answer({ errorCode: 'refused' }))
      render()
      await click('http://localhost:8000/')
      expect(notice()).toBe('office-air refused access to port 8000.')
      render({ remote: true, isActive: false })
      expect(notice()).toBeUndefined()
    })
  })

  describe('in flight', () => {
    it('a second meta-click before the first answer makes no second call', async () => {
      let resolve!: (r: PortResult) => void
      hoisted.openRemotePort.mockReturnValue(new Promise<PortResult>((r) => { resolve = r }))
      render()
      await click('http://localhost:8000/')
      await click('http://localhost:8000/')
      expect(hoisted.openRemotePort).toHaveBeenCalledTimes(1)
      await act(async () => { resolve(opened(8000)) })
      expect(hoisted.openLink).toHaveBeenCalledTimes(1)
      hoisted.openRemotePort.mockResolvedValue(opened(8000))
      await click('http://localhost:8000/')
      expect(hoisted.openRemotePort).toHaveBeenCalledTimes(2)
    })
  })

  describe('everything else keeps today\'s path (URL-03)', () => {
    it('a local tab opens http://localhost:8000/ directly', async () => {
      render({ remote: false })
      await click('http://localhost:8000/')
      expect(hoisted.openLink).toHaveBeenCalledWith('http://localhost:8000/')
      expect(hoisted.openRemotePort).not.toHaveBeenCalled()
    })

    it('the browser share view (wsURL) opens it directly', async () => {
      render({ remote: false, wsURL: 'ws://127.0.0.1:1/api/relay/remote/s1/ws' })
      await click('http://localhost:8000/')
      expect(hoisted.openLink).toHaveBeenCalledWith('http://localhost:8000/')
      expect(hoisted.openRemotePort).not.toHaveBeenCalled()
    })

    it.each(['https://example.com/', 'https://localhost:8000/', 'http://example.com:8000/', 'http://LOCALHOST:8000/'])(
      'a peer tab opens %s directly with the clicked URL',
      async (url) => {
        render()
        await click(url)
        expect(hoisted.openLink).toHaveBeenCalledTimes(1)
        expect(hoisted.openLink).toHaveBeenCalledWith(url)
        expect(hoisted.openRemotePort).not.toHaveBeenCalled()
      },
    )

    it('a peer tab shows the link confirm popover for a punycode URL and never calls the binding', async () => {
      render()
      await click('https://xn--exmple-cua.com/docs')
      expect(hoisted.openRemotePort).not.toHaveBeenCalled()
      expect(hoisted.openLink).not.toHaveBeenCalled()
      expect(document.body.textContent).toContain('xn--exmple-cua.com')
      expect(document.body.textContent).toContain('internationalized characters')
    })
  })
})
