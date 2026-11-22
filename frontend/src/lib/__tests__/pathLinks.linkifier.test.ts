/**
 * Phase 12 (PATH-02, gap 2): a viewer that survives a reconnect or a grid change keeps its path links.
 *
 * These tests drive xterm 6's OWN Linkifier through DOM mouse events on a real Terminal fed by a real
 * RelayClient, never by calling the link provider directly: the loss lived in the Linkifier's per-row
 * reply cache and last-cell memory, which a direct provider call cannot see. They stand for VM rows
 * U-01 (cursor stayed `text` on 12 x positions of one row after a drop) and C-03 conn=3.
 *
 * jsdom rig (the planner's probe at 81ada526):
 *  - xterm's DomMeasureStrategy reads offsetWidth/offsetHeight of `.xterm-char-measure-element`; jsdom has
 *    no layout and no OffscreenCanvas, so those getters return 8 * 32 and 16 for that element, else 0.
 *  - `.xterm-screen` gets `style.padding = '0px'` (jsdom leaves it empty; xterm's getCoords would read NaN).
 *  - jsdom's getBoundingClientRect is all zeros, so cell (c, r), 1-based, has its centre at
 *    clientX (c - 0.5) * 8 and clientY (r - 0.5) * 16.
 *  - Mouse events are dispatched on `.xterm-screen`, where the Linkifier listens, with metaKey set;
 *    navigator.platform is MacIntel so the 'platform' modifier is Cmd.
 *  - Real timers (xterm renders on animation frames). After an option change or a write the tests wait
 *    and assert that term.onRender fired, so a poisoning step cannot be skipped silently.
 *  - Every test asserts the rig first: a broken rig fails loudly instead of passing.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { Terminal, type IDisposable } from '@xterm/xterm'
import { RelayClient } from '../relayClient'
import { registerPathLinks } from '../pathLinks'
import fixture from './fixtures/path-click.json'

class WS {
  static OPEN = 1
  static instances: WS[] = []
  binaryType = ''
  readyState = 1
  onopen: (() => void) | null = null
  onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
  onclose: ((e: { code: number; reason: string }) => void) | null = null
  onerror: (() => void) | null = null
  send = vi.fn()
  close = vi.fn()
  constructor(public url: string) { WS.instances.push(this) }
}

const hex = (h: string) => Uint8Array.from(h.match(/../g)!.map((b) => parseInt(b, 16)))
const enc = new TextEncoder()
const dec = new TextDecoder()
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

const PATH = 'docs/diagram.png'

describe('real xterm Linkifier (PATH-02 gap 2)', () => {
  const origWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth')
  const origHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')
  const origPlatform = Object.getOwnPropertyDescriptor(navigator, 'platform')

  let term: Terminal
  let host: HTMLDivElement
  let screen: HTMLElement
  let client: RelayClient
  let enabled = false
  let handle: (IDisposable & { invalidate?: () => void }) | null = null
  let renders = 0
  const subs: IDisposable[] = []

  const measure = (el: HTMLElement) => el.classList.contains('xterm-char-measure-element')

  // Mirrors the TerminalPanel.tsx RelayClient callbacks that matter here.
  const start = () => {
    client = new RelayClient(1, 's', {
      onOutput: (data) => { term.write(data) },
      onPathClickCapability: (on) => { enabled = on; handle?.invalidate?.() },
      onStateChange: (state) => {
        if (state === 'reconnecting') term.options.disableStdin = true
        if (state === 'open') term.options.disableStdin = false
      },
      onResize: (cols, rows) => {
        term.write('', () => { term.resize(Math.max(1, cols), Math.max(1, rows)) })
      },
    }, { remote: true, reconnect: {} })
    WS.instances[0].onopen?.()
  }
  const sock = () => WS.instances[WS.instances.length - 1]
  const feed = (f: Uint8Array) => sock().onmessage?.({ data: f.slice().buffer as ArrayBuffer })
  const meta = (m: object) => feed(new Uint8Array([0x20, ...enc.encode(JSON.stringify(m))]))
  const openSession = (replay: 'full' | 'delta', pathClick: boolean) => {
    meta(pathClick ? { replay, pathClick: true } : { replay })
    meta({ offset: 0 })
  }
  // The 0x13 payloads sent on any socket.
  const pathClicks = () =>
    WS.instances.flatMap((w) => w.send.mock.calls.map((c) => c[0] as Uint8Array))
      .filter((b) => b[0] === 0x13)
      .map((b) => dec.decode(b.slice(1)))

  const renderOnce = async () => {
    const before = renders
    term.refresh(0, term.rows - 1)
    await sleep(100)
    expect(renders, 'term.onRender fired').toBeGreaterThan(before)
  }
  const write = (s: string) => new Promise<void>((r) => term.write(s, () => r()))

  const at = (c: number, r: number) => ({ clientX: (c - 0.5) * 8, clientY: (r - 0.5) * 16 })
  const mouse = (type: string, c: number, r: number, meta = true) =>
    screen.dispatchEvent(new MouseEvent(type, { bubbles: true, metaKey: meta, ...at(c, r) }))
  const hover = (c: number, r: number) => mouse('mousemove', c, r)
  const click = (c: number, r: number) => { mouse('mousedown', c, r); mouse('mouseup', c, r) }

  const assertRig = () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const core = (term as any)._core
    expect(core._charSizeService.hasValidSize).toBe(true)
    expect(core._charSizeService.width).toBe(8)
    expect(core._charSizeService.height).toBe(16)
  }

  beforeEach(async () => {
    WS.instances = []
    vi.stubGlobal('WebSocket', WS)
    Object.defineProperty(navigator, 'platform', { value: 'MacIntel', configurable: true })
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
      configurable: true, get(this: HTMLElement) { return measure(this) ? 8 * 32 : 0 },
    })
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true, get(this: HTMLElement) { return measure(this) ? 16 : 0 },
    })
    enabled = false
    handle = null
    renders = 0
    host = document.createElement('div')
    document.body.appendChild(host)
    term = new Terminal({ cols: 40, rows: 10 })
    term.open(host)
    screen = term.element!.querySelector('.xterm-screen') as HTMLElement
    screen.style.padding = '0px'
    subs.push(term.onRender(() => { renders++ }))
    handle = registerPathLinks(term, {
      enabled: () => enabled,
      modifier: () => 'platform',
      onClick: (t) => client.sendPathClick(t),
    })
    start()
    openSession('full', true)
    await write(`${PATH}\r\n$ `)
    await sleep(100)
  })

  afterEach(() => {
    handle?.dispose()
    client.close()
    subs.splice(0).forEach((s) => s.dispose())
    term.dispose()
    host.remove()
    vi.unstubAllGlobals()
    if (origPlatform) Object.defineProperty(navigator, 'platform', origPlatform)
    if (origWidth) Object.defineProperty(HTMLElement.prototype, 'offsetWidth', origWidth)
    if (origHeight) Object.defineProperty(HTMLElement.prototype, 'offsetHeight', origHeight)
  })

  // The socket drops with the hub's restart close (1012, "service restart"), the harness disables stdin as
  // TerminalPanel does on `reconnecting`, a render runs during the gap, and the redial gets a delta replay.
  const dropAndReturn = async () => {
    sock().onclose?.({ code: 1012, reason: 'service restart' })
    expect(term.options.disableStdin).toBe(true)
    await renderOnce()
    window.dispatchEvent(new Event('portty:wake'))
    await sleep(20)
    expect(WS.instances.length, 'the client redialed').toBe(2)
    sock().onopen?.()
    openSession('delta', true)
    await sleep(100)
    expect(term.options.disableStdin).toBe(false)
  }

  it('control: a Cmd-click on a printed path sends one path click', () => {
    assertRig()
    expect(hex(fixture.frames.replayMetaHex)[0]).toBe(0x20)
    hover(3, 1)
    click(3, 1)
    expect(pathClicks()).toEqual([PATH])
  })

  it('after a reconnect, a Cmd-click at the same cell sends the path click with no pointer move', async () => {
    assertRig()
    hover(3, 1)
    await dropAndReturn()
    click(3, 1) // no mousemove: the pointer rested on the path the whole time
    expect(pathClicks()).toEqual([PATH])
  })

  it('after a reconnect, another cell of the same row offers the link', async () => {
    assertRig()
    hover(3, 1)
    await dropAndReturn()
    hover(9, 1)
    click(9, 1)
    expect(pathClicks()).toEqual([PATH])
  })

  it('while the capability is false, no link is offered and nothing is sent', async () => {
    assertRig()
    client.close()
    handle?.dispose()
    WS.instances = []
    enabled = false
    start()
    openSession('full', false)
    handle = registerPathLinks(term, {
      enabled: () => enabled,
      modifier: () => 'platform',
      onClick: (t) => client.sendPathClick(t),
    })
    await renderOnce()
    hover(3, 1)
    click(3, 1)
    hover(9, 1)
    click(9, 1)
    expect(pathClicks()).toEqual([])
    expect(term.element!.classList.contains('xterm-cursor-pointer')).toBe(false)
  })
})
