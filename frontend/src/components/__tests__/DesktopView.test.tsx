// Phase 19-06 Task 3 — the /desktop/ viewer's states and look (19-UI-SPEC Surface 5).
// The client is mocked with emitters; the view is rendered into jsdom.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { ClientStatus, DesktopEvents } from '../../lib/desktopClient'
import { DesktopView, videoContentRect, type VideoRect } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  start: ReturnType<typeof vi.fn>
  stop: ReturnType<typeof vi.fn>
  wake: ReturnType<typeof vi.fn>
  on: (ev: string, cb: Listener) => () => void
  emit: <K extends keyof DesktopEvents>(ev: K, v: DesktopEvents[K]) => void
}

const h = vi.hoisted(() => ({ clients: [] as unknown[] }))

vi.mock('../../lib/desktopClient', () => ({
  createDesktopClient: vi.fn(() => {
    const ls = new Map<string, Set<Listener>>()
    const client = {
      start: vi.fn(),
      stop: vi.fn(),
      wake: vi.fn(),
      on: (ev: string, cb: Listener) => {
        if (!ls.has(ev)) ls.set(ev, new Set())
        ls.get(ev)!.add(cb)
        return () => ls.get(ev)!.delete(cb)
      },
      emit: (ev: string, v: unknown) => {
        for (const cb of [...(ls.get(ev) ?? [])]) (cb as (x: unknown) => void)(v)
      },
    }
    h.clients.push(client)
    return client
  }),
}))

let container: HTMLDivElement
let root: Root
let play: ReturnType<typeof vi.spyOn>

const client = () => h.clients[h.clients.length - 1] as MockClient
const video = () => container.querySelector('video') as HTMLVideoElement
const chip = () => container.querySelector('.desktop-chip')
const card = () => container.querySelector('.desktop-card')
const scrim = () => container.querySelector('.desktop-scrim')
const region = () => container.querySelector('[role="status"]') as HTMLElement

function render(props: { onVideoRect?: (r: VideoRect) => void; onReload?: () => void } = {}) {
  act(() => root.render(<DesktopView {...props} />))
}
function status(s: ClientStatus) {
  act(() => client().emit('status', s))
}
function state(st: DesktopEvents['state']) {
  act(() => client().emit('state', st))
}
function track() {
  act(() => client().emit('track', {} as MediaStream))
}
async function firstFrame() {
  status({ kind: 'streaming' })
  track()
  await act(async () => {
    video().dispatchEvent(new Event('playing'))
  })
}
const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.useFakeTimers()
  h.clients.length = 0
  play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('Connecting', () => {
  it('shows the Connecting card with its body and no scrim before the first frame', () => {
    render()
    expect(card()?.querySelector('h1')?.textContent).toBe('Connecting…')
    expect(card()?.textContent).toContain(
      'Waiting for the first picture from your Mac. If your browser asks to find devices nearby, choose Allow.',
    )
    expect(scrim()).toBeNull()
    expect(client().start).toHaveBeenCalledTimes(1)
  })
})

describe('streaming chip and live region', () => {
  it('clears the card at the first frame, shows Streaming to 1 viewer for 3 s, then hides it', async () => {
    render()
    state({ state: 'streaming', viewers: 1, locked: false })
    await firstFrame()
    expect(card()).toBeNull()
    expect(chip()?.textContent).toBe('Streaming to 1 viewer')
    advance(2999)
    expect(chip()).not.toBeNull()
    advance(1 + 150)
    expect(chip()).toBeNull()
    expect(region().textContent).toBe('Streaming')
    expect(region().querySelector('.visually-hidden')).not.toBeNull()
  })

  it('shows Streaming to 2 viewers again when the count changes', async () => {
    render()
    state({ state: 'streaming', viewers: 1, locked: false })
    await firstFrame()
    advance(4000)
    expect(chip()).toBeNull()
    state({ state: 'streaming', viewers: 2, locked: false })
    expect(chip()?.textContent).toBe('Streaming to 2 viewers')
    state({ state: 'streaming', viewers: 2, locked: false })
    advance(3150)
    expect(chip()).toBeNull()
  })

  it('keeps a Locked chip with the lock icon for as long as the Mac is locked', async () => {
    render()
    await firstFrame()
    state({ state: 'locked', viewers: 1, locked: true })
    expect(chip()?.textContent).toBe('Locked')
    expect(chip()?.querySelector('svg')).not.toBeNull()
    advance(60000)
    expect(chip()?.textContent).toBe('Locked')
    state({ state: 'streaming', viewers: 1, locked: false })
    expect(chip()?.textContent ?? '').not.toBe('Locked')
  })
})

describe('state cards over the scrim', () => {
  it.each([
    ['needs_permission', 'Needs permission', "Portty doesn't have permission to show this Mac's screen."],
    ['awaiting_approval', 'Waiting for approval at the Mac', "macOS is asking someone at the Mac to approve Portty's screen access."],
    ['display_asleep', 'Display asleep', "The Mac's display is off. The picture comes back when it wakes."],
    ['capture_paused', 'Desktop paused', "Portty stopped showing this Mac's screen after repeated errors. On the Mac, open Portty, go to Settings, then Sharing, and run Test Capture."],
  ] as const)('%s shows the %s card with its icon', async (st, heading, body) => {
    render()
    await firstFrame()
    state({ state: st, viewers: 1, locked: false })
    expect(card()?.querySelector('h1')?.textContent).toBe(heading)
    expect(card()?.textContent).toContain(body)
    expect(card()?.querySelector('svg')).not.toBeNull()
    expect(scrim()).not.toBeNull()
  })
})

describe('Reconnecting and unreachable', () => {
  it('shows a static Reconnecting chip over the scrim, keeps the one video element, and clears on return', async () => {
    render()
    await firstFrame()
    const v = video()
    status({ kind: 'reconnecting' })
    expect(chip()?.textContent).toBe('Reconnecting…')
    expect(chip()?.querySelector('svg')?.getAttribute('class') ?? '').not.toMatch(/spin|rotate|animate/)
    expect(scrim()).not.toBeNull()
    expect(container.querySelectorAll('video')).toHaveLength(1)
    expect(video()).toBe(v)
    track()
    status({ kind: 'streaming' })
    expect(chip()?.textContent ?? '').not.toContain('Reconnecting')
    expect(scrim()).toBeNull()
    expect(container.textContent).not.toMatch(/reconnected/i)
    expect(video()).toBe(v)
  })

  it("shows the Can't reach this Mac card and a Reload Page button that reloads", async () => {
    const onReload = vi.fn()
    render({ onReload })
    await firstFrame()
    status({ kind: 'unreachable' })
    expect(card()?.querySelector('h1')?.textContent).toBe("Can't reach this Mac")
    expect(card()?.textContent).toContain("Check that it's awake and on your tailnet. This page keeps trying.")
    expect(card()?.querySelector('svg')).not.toBeNull()
    const btn = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Reload Page')!
    act(() => btn.click())
    expect(onReload).toHaveBeenCalledTimes(1)
    status({ kind: 'streaming' })
    expect(card()).toBeNull()
  })
})

describe('final errors', () => {
  it('token_rejected shows its own heading and body', () => {
    render()
    status({ kind: 'token_rejected' })
    expect(card()?.querySelector('h1')?.textContent).toBe('This desktop link no longer works')
    expect(card()?.textContent).toContain('The link is incomplete or has been replaced.')
  })

  it('desktop_off shows its own heading and body', () => {
    render()
    status({ kind: 'desktop_off' })
    expect(card()?.querySelector('h1')?.textContent).toBe('Desktop sharing is off on this Mac')
    expect(card()?.textContent).toContain("Turn on Always-available desktop in Portty's Settings")
  })

  it('other_account shows the existing UserRefused screen', () => {
    render()
    status({ kind: 'other_account' })
    expect(container.textContent).toContain('This link only works on your own devices')
    expect(container.querySelector('.link-dead')).not.toBeNull()
    expect(card()).toBeNull()
  })

  it('the three errors read differently and none is generic', () => {
    const seen = new Set<string>()
    for (const kind of ['token_rejected', 'desktop_off', 'unreachable'] as const) {
      render()
      status({ kind })
      seen.add(card()!.textContent!)
      act(() => root.render(null))
    }
    expect(seen.size).toBe(3)
    for (const t of seen) expect(t).not.toMatch(/something went wrong|oops|error:/i)
  })
})

describe('Tap to start', () => {
  it('shows a focused Tap to Start button when play() is rejected, and starting removes the card', async () => {
    play.mockRejectedValueOnce(new DOMException('blocked', 'NotAllowedError'))
    render()
    status({ kind: 'streaming' })
    await act(async () => client().emit('track', {} as MediaStream))
    expect(card()?.querySelector('h1')?.textContent).toBe('Tap to start')
    expect(card()?.textContent).toContain('Your browser needs a tap to start the picture.')
    const btn = container.querySelector('button')!
    expect(btn.textContent).toBe('Tap to Start')
    expect(document.activeElement).toBe(btn)
    expect(scrim()).not.toBeNull()
    const before = play.mock.calls.length
    await act(async () => btn.click())
    expect(play.mock.calls.length).toBe(before + 1)
    expect(card()?.querySelector('h1')?.textContent).not.toBe('Tap to start')
    expect(container.querySelector('button')).toBeNull()
  })

  it('a tap on the scrim also starts the picture', async () => {
    play.mockRejectedValueOnce(new DOMException('blocked', 'NotAllowedError'))
    render()
    await act(async () => client().emit('track', {} as MediaStream))
    const before = play.mock.calls.length
    await act(async () => (scrim() as HTMLElement).click())
    expect(play.mock.calls.length).toBe(before + 1)
    expect(card()?.querySelector('h1')?.textContent).not.toBe('Tap to start')
  })

  it('keeps the Tap to Start and Reload Page buttons outside the live status region', async () => {
    play.mockRejectedValueOnce(new DOMException('blocked', 'NotAllowedError'))
    render()
    status({ kind: 'streaming' })
    await act(async () => client().emit('track', {} as MediaStream))
    const live = region()
    const tap = container.querySelector('button')!
    expect(tap.textContent).toBe('Tap to Start')
    expect(card()?.contains(tap)).toBe(true)
    expect(live.contains(tap)).toBe(false)
    expect(live.textContent).toContain('Tap to start')
    status({ kind: 'unreachable' })
    const reload = container.querySelector('button')!
    expect(reload.textContent).toBe('Reload Page')
    expect(region()).toBe(live) // the region persists, so the change is announced
    expect(live.contains(reload)).toBe(false)
    expect(live.textContent).toContain("Can't reach this Mac")
  })

  it('stays on the card when play() is rejected again', async () => {
    play.mockRejectedValue(new DOMException('blocked', 'NotAllowedError'))
    render()
    await act(async () => client().emit('track', {} as MediaStream))
    await act(async () => container.querySelector('button')!.click())
    expect(card()?.querySelector('h1')?.textContent).toBe('Tap to start')
  })
})

describe('layout and scope', () => {
  it('has one muted, playsinline, autoplay video with no transform, zoom, size attributes or colour on it', () => {
    render()
    const v = video()
    expect(v.className).toBe('desktop-view__video')
    expect(v.muted).toBe(true)
    expect(v.hasAttribute('playsinline')).toBe(true)
    expect(v.autoplay).toBe(true)
    expect(v.getAttribute('aria-label')).toBe('Mac desktop')
    for (const a of ['width', 'height', 'style']) expect(v.hasAttribute(a), a).toBe(false)
  })

  it('adds no keydown, pointerdown or contextmenu listener to window or document', () => {
    const add = [vi.spyOn(window, 'addEventListener'), vi.spyOn(document, 'addEventListener')]
    act(() => root.render(null))
    render()
    const types = add.flatMap((s) => s.mock.calls.map((c) => c[0]))
    for (const t of ['keydown', 'keyup', 'pointerdown', 'contextmenu', 'mousedown']) expect(types).not.toContain(t)
  })

  it('sets the title and the apple-mobile-web-app-title meta, and leaves ?cap= in the address bar', () => {
    window.history.replaceState(null, '', '/desktop/?cap=tok123')
    render()
    expect(document.title).toBe('Portty Desktop')
    expect(document.querySelector('meta[name="apple-mobile-web-app-title"]')?.getAttribute('content')).toBe('Portty Desktop')
    expect(window.location.search).toBe('?cap=tok123')
  })

  it('stops the client on unmount', () => {
    render()
    const c = client()
    act(() => root.render(null))
    expect(c.stop).toHaveBeenCalledTimes(1)
  })
})

describe('videoContentRect', () => {
  it('letterboxes a wide video in a wider element', () => {
    expect(videoContentRect(1000, 500, 1600, 1000)).toEqual({ x: 100, y: 0, width: 800, height: 500 })
  })
  it('letterboxes top and bottom in a tall element', () => {
    expect(videoContentRect(400, 800, 1600, 1000)).toEqual({ x: 0, y: 275, width: 400, height: 250 })
  })
  it('gives a zero rect for a 0x0 video or element', () => {
    expect(videoContentRect(1000, 500, 0, 0)).toEqual({ x: 0, y: 0, width: 0, height: 0 })
    expect(videoContentRect(0, 0, 1600, 1000)).toEqual({ x: 0, y: 0, width: 0, height: 0 })
  })
})

describe('onVideoRect', () => {
  function size(el: HTMLElement, w: number, hgt: number) {
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: w })
    Object.defineProperty(el, 'clientHeight', { configurable: true, value: hgt })
  }
  function videoSize(el: HTMLVideoElement, w: number, hgt: number) {
    Object.defineProperty(el, 'videoWidth', { configurable: true, value: w })
    Object.defineProperty(el, 'videoHeight', { configurable: true, value: hgt })
  }

  it('reports the recomputed rect after the video resize event and after a window resize', () => {
    const rects: VideoRect[] = []
    render({ onVideoRect: (r) => rects.push(r) })
    size(video(), 1000, 500)
    videoSize(video(), 1600, 1000)
    act(() => video().dispatchEvent(new Event('resize')))
    expect(rects[rects.length - 1]).toEqual({ x: 100, y: 0, width: 800, height: 500 })
    size(video(), 400, 800)
    act(() => window.dispatchEvent(new Event('resize')))
    expect(rects[rects.length - 1]).toEqual({ x: 0, y: 275, width: 400, height: 250 })
  })
})

describe('style.css', () => {
  const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf8')

  it('declares --color-viewer-bg once in the light block and once in the dark block', () => {
    expect(css.match(/--color-viewer-bg:\s*#000000;/g)).toHaveLength(2)
    const dark = css.slice(css.indexOf('prefers-color-scheme: dark'))
    expect(dark.indexOf('--color-viewer-bg')).toBeGreaterThan(-1)
    expect(css.slice(0, css.indexOf('prefers-color-scheme: dark')).match(/--color-viewer-bg/g)).toHaveLength(1)
  })

  it('fades the chip in 150 ms and drops the transition under prefers-reduced-motion', () => {
    expect(css).toMatch(/\.desktop-chip\s*\{[^}]*transition:[^;}]*150ms/)
    const rm = css.match(/@media \(prefers-reduced-motion: reduce\)\s*\{[\s\S]*?\.desktop-chip[^}]*transition:\s*none/)
    expect(rm).not.toBeNull()
  })

  it('puts no transform, zoom or image-rendering on the video and fits it by object-fit', () => {
    const rule = css.match(/\.desktop-view__video\s*\{([^}]*)\}/)![1]
    expect(rule).toMatch(/object-fit:\s*contain/)
    expect(rule).not.toMatch(/transform|zoom|image-rendering/)
  })
})
