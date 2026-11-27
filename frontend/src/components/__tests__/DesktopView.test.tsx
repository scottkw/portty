// Phase 19-06 Task 3 — the /desktop/ viewer's states and look (19-UI-SPEC Surface 5).
// The client is mocked with emitters; the view is rendered into jsdom.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { ClientStatus, DesktopEvents } from '../../lib/desktopClient'
import { DesktopView, videoContentRect, type DesktopViewProps, type VideoRect } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  start: ReturnType<typeof vi.fn>
  stop: ReturnType<typeof vi.fn>
  wake: ReturnType<typeof vi.fn>
  send: ReturnType<typeof vi.fn>
  sendInput: ReturnType<typeof vi.fn>
  currentEpoch: ReturnType<typeof vi.fn>
  requestSurfaces: ReturnType<typeof vi.fn>
  selectSurface: ReturnType<typeof vi.fn>
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
      send: vi.fn(),
      sendInput: vi.fn(() => true),
      currentEpoch: vi.fn(() => 1),
      requestSurfaces: vi.fn(() =>
        Promise.resolve({
          displays: [
            { id: 1, name: 'Main Display', w: 1470, h: 956, main: true },
            { id: 2, name: 'Display 2', w: 1920, h: 1080, main: false },
          ],
          windows: [{ id: 4242, app: 'Terminal', title: 'zsh', name: 'Terminal — zsh' }],
        }),
      ),
      selectSurface: vi.fn(),
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

function render(props: DesktopViewProps = {}) {
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

describe('control on the stage (Phase 20-06 Task 1)', () => {
  function size(el: HTMLElement, w: number, hgt: number) {
    Object.defineProperty(el, 'clientWidth', { configurable: true, value: w })
    Object.defineProperty(el, 'clientHeight', { configurable: true, value: hgt })
  }
  function pointerdown(el: Element, x: number, y: number) {
    act(() => {
      el.dispatchEvent(new PointerEvent('pointerdown', { clientX: x, clientY: y, bubbles: true, button: 0, pointerId: 1 }))
    })
  }
  const stage = () => container.querySelector('.desktop-stage') as HTMLElement
  function ownAccount() {
    size(video(), 1000, 500)
    Object.defineProperty(video(), 'videoWidth', { configurable: true, value: 1600 })
    Object.defineProperty(video(), 'videoHeight', { configurable: true, value: 1000 })
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
  }

  it('wraps the one video in the stage, with the hidden input beside it', () => {
    render()
    expect(video().parentElement).toBe(stage())
    expect(stage().querySelector('textarea.desktop-stage__input')).not.toBeNull()
  })

  it('forwards a click on the picture to the client as a btn on the input channel, and not one in the bar', () => {
    render()
    ownAccount()
    pointerdown(stage(), 50, 250)
    expect(client().sendInput).not.toHaveBeenCalled()
    pointerdown(stage(), 500, 250)
    expect(client().sendInput).toHaveBeenCalledWith(expect.objectContaining({ t: 'btn', b: 0, d: true, x: 0.5, y: 0.5, e: 1 }), false)
  })

  it('creates no controller when the hello does not grant control', () => {
    render()
    ownAccount()
    act(() => root.render(null))
    render()
    size(video(), 1000, 500)
    act(() => client().emit('rights', { control: false, clipboard: false, surface: false }))
    pointerdown(stage(), 500, 250)
    expect(client().sendInput).not.toHaveBeenCalled()
  })
})

describe('control layer (Phase 20-06 Task 3)', () => {
  const stage = () => container.querySelector('.desktop-stage') as HTMLElement
  const input = () => container.querySelector('textarea') as HTMLTextAreaElement
  const ring = () => container.querySelector('.desktop-ring')
  const dot = () => container.querySelector('.desktop-dot') as HTMLElement | null
  const notice = () => container.querySelector('.desktop-chip--notice')
  const LONG_HINT = 'Typing goes to the Mac. To give the keyboard back, press Ctrl+Option+Cmd+Return.'

  function ownAccount() {
    for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
      Object.defineProperty(video(), a, { configurable: true, value: b })
    }
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
  }
  const click = () =>
    act(() => {
      stage().dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 }))
    })
  const move = (x: number, y: number) =>
    act(() => {
      stage().dispatchEvent(new PointerEvent('pointermove', { clientX: x, clientY: y, bubbles: true, pointerId: 1 }))
      vi.advanceTimersByTime(20)
    })
  const key = (code: string, k: string, init: KeyboardEventInit = {}) =>
    act(() => {
      input().dispatchEvent(new KeyboardEvent('keydown', { code, key: k, bubbles: true, cancelable: true, ...init }))
    })
  const sentTypes = () => client().sendInput.mock.calls.map((c) => (c[0] as { t: string }).t)

  beforeEach(() => window.localStorage.clear())

  it('an own-account viewer starts in Control with the keyboard off: stage tabindex 0 and no ring', () => {
    render()
    ownAccount()
    expect(stage().getAttribute('tabindex')).toBe('0')
    expect(stage().getAttribute('role')).toBe('application')
    expect(stage().getAttribute('aria-describedby')).toBe('desktop-stage-hint')
    expect(document.getElementById('desktop-stage-hint')?.textContent).toContain('press Control, Option, Command and Return together')
    expect(ring()).toBeNull()
    expect(notice()).toBeNull()
  })

  it('a click on the picture shows the ring and the long hint once; later captures say only Typing goes to the Mac', () => {
    render()
    ownAccount()
    click()
    expect(ring()).not.toBeNull()
    expect(notice()?.textContent).toBe(LONG_HINT)
    expect(window.localStorage.getItem('portty:desktop-release-hint-seen')).not.toBeNull()
    key('Enter', 'Enter', { ctrlKey: true, altKey: true, metaKey: true })
    expect(ring()).toBeNull()
    expect(notice()?.textContent).toBe('Typing stays on this device')
    click()
    expect(ring()).not.toBeNull()
    expect(notice()?.textContent).toBe('Typing goes to the Mac')
  })

  it('the release chord calls the host once and is never sent to the Mac', () => {
    const onReleaseChord = vi.fn()
    render({ onReleaseChord })
    ownAccount()
    click()
    key('Enter', 'Enter', { ctrlKey: true, altKey: true, metaKey: true })
    expect(onReleaseChord).toHaveBeenCalledTimes(1)
    expect(sentTypes()).not.toContain('key')
  })

  it('a window blur removes the ring and shows no notice', () => {
    render()
    ownAccount()
    click()
    advance(5200)
    expect(notice()).toBeNull()
    act(() => window.dispatchEvent(new Event('blur')))
    expect(ring()).toBeNull()
    expect(notice()).toBeNull()
  })

  it('the notice dwells 3 s (5 s with the hint), then fades 150 ms and goes; a newer one replaces it', () => {
    render()
    ownAccount()
    click()
    advance(4999)
    expect(notice()).not.toBeNull()
    advance(1 + 150)
    expect(notice()).toBeNull()
    key('Enter', 'Enter', { ctrlKey: true, altKey: true, metaKey: true })
    click()
    expect(container.querySelectorAll('.desktop-chip--notice')).toHaveLength(1)
    advance(2999)
    expect(notice()).not.toBeNull()
    advance(1 + 150)
    expect(notice()).toBeNull()
  })

  it('View Only releases, drops the stage to tabindex -1 and hides the dot and the ring', () => {
    render()
    ownAccount()
    click()
    move(500, 250)
    client().sendInput.mockClear()
    render({ controlMode: 'view' })
    expect(client().sendInput).toHaveBeenCalledWith({ t: 'release' }, false)
    expect(stage().getAttribute('tabindex')).toBe('-1')
    expect(ring()).toBeNull()
    expect(dot()?.hidden).toBe(true)
    expect(notice()?.textContent).toBe('Typing stays on this device')
    click()
    expect(sentTypes()).toEqual(['release'])
  })

  it('opening a menu releases everything and gives the keyboard back with a word', () => {
    render()
    ownAccount()
    click()
    client().sendInput.mockClear()
    render({ menuOpen: true })
    expect(sentTypes()).toContain('release')
    expect(ring()).toBeNull()
    expect(notice()?.textContent).toBe('Typing stays on this device')
  })

  describe('Held keys were released', () => {
    async function holding() {
      render()
      ownAccount()
      click()
      advance(5200)
      key('KeyA', 'a')
    }

    it.each([
      ['the Mac locks', () => state({ state: 'locked', viewers: 1, locked: true })],
      ['the surface epoch changes', () => {
        act(() => client().emit('surface', { e: 1, kind: 'display', id: 1, name: 'Main Display', w: 1470, h: 956, reason: '' }))
        act(() => client().emit('surface', { e: 2, kind: 'display', id: 1, name: 'Main Display', w: 1470, h: 956, reason: '' }))
      }],
      ['the connection drops', () => status({ kind: 'reconnecting' })],
      ['the input channel closes', () => act(() => client().emit('input', { open: false }))],
    ] as const)('shows the notice for 3 s when something was held and %s', async (_why, trigger) => {
      await holding()
      key('KeyA', 'a') // a repeat of the same key: still one held
      trigger()
      expect(notice()?.textContent).toBe('Held keys were released')
      advance(3000 + 150)
      expect(notice()).toBeNull()
    })

    it('shows nothing when nothing was held', () => {
      render()
      ownAccount()
      advance(5200)
      state({ state: 'locked', viewers: 1, locked: true })
      status({ kind: 'reconnecting' })
      act(() => client().emit('input', { open: false }))
      expect(notice()).toBeNull()
    })

    it("a user's own blur with a key held shows no held-keys notice", async () => {
      await holding()
      act(() => window.dispatchEvent(new Event('blur')))
      expect(notice()).toBeNull()
    })
  })

  describe('the local dot', () => {
    it('follows the pointer on the animation frame and hides on pointer leave', () => {
      render()
      ownAccount()
      expect(dot()?.hidden).toBe(true)
      move(500, 250)
      expect(dot()?.hidden).toBe(false)
      expect(dot()?.style.transform).toBe('translate(496px, 246px)')
      expect(dot()?.getAttribute('aria-hidden')).toBe('true')
      act(() => stage().dispatchEvent(new PointerEvent('pointerleave')))
      expect(dot()?.hidden).toBe(true)
    })

    it('stays hidden while locked, and while the daemon says Control needs Accessibility', async () => {
      render()
      ownAccount()
      await firstFrame()
      state({ state: 'locked', viewers: 1, locked: true })
      move(500, 250)
      expect(dot()?.hidden).toBe(true)
      state({ state: 'streaming', viewers: 1, locked: false })
      act(() => client().emit('control', { input: 'needs_accessibility', window: '' }))
      move(510, 250)
      expect(dot()?.hidden).toBe(true)
      expect(container.textContent).toContain('Control needs Accessibility. On the Mac, allow Portty in Privacy & Security.')
      act(() => client().emit('control', { input: 'ok', window: '' }))
      move(520, 250)
      expect(dot()?.hidden).toBe(false)
    })

    it('is not drawn when the stage is not live: View Only', () => {
      render()
      ownAccount()
      render({ controlMode: 'view' })
      move(500, 250)
      expect(dot()?.hidden).toBe(true)
    })
  })

  it('shows the window chips the daemon reports and clears them with the condition', async () => {
    render()
    ownAccount()
    await firstFrame()
    act(() => client().emit('control', { input: 'ok', window: 'raise_failed' }))
    expect(chip()?.textContent).toBe("Couldn't bring the window to the front. Click the picture to try again.")
    act(() => client().emit('control', { input: 'ok', window: 'minimized' }))
    expect(chip()?.textContent).toBe('The shared window is minimized. Click the picture to bring it back.')
    act(() => client().emit('control', { input: 'ok', window: '' }))
    expect(container.querySelector('.desktop-chip:not(.desktop-chip--notice)')).toBeNull()
  })

  it('a phone gets no controller, no stage focus, no dot and no ring: the Phase 19 page', () => {
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (q: string) => ({ matches: true, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
    )
    render()
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
    expect(stage().hasAttribute('tabindex')).toBe(false)
    expect(stage().hasAttribute('role')).toBe(false)
    expect(dot()).toBeNull()
    expect(ring()).toBeNull()
    click()
    expect(client().sendInput).not.toHaveBeenCalled()
  })

  it('tells the host when control becomes available and when the keyboard is captured', () => {
    const calls: unknown[] = []
    render({ onControlChange: (c) => calls.push(c) })
    ownAccount()
    click()
    expect(calls).toEqual([
      { available: false, captured: false },
      { available: true, captured: false },
      { available: true, captured: true },
    ])
  })
})

describe('control bar (Phase 20-07 Task 1)', () => {
  const bar = () => container.querySelector('[role="toolbar"]') as HTMLElement | null
  const stage = () => container.querySelector('.desktop-stage') as HTMLElement
  const input = () => container.querySelector('textarea') as HTMLTextAreaElement
  const sendKeysBtn = () => container.querySelector('button[aria-label="Send Keys"]') as HTMLButtonElement
  const menuItem = (t: string) =>
    Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find((i) => i.textContent?.startsWith(t))!

  function ownAccount() {
    for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
      Object.defineProperty(video(), a, { configurable: true, value: b })
    }
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
  }

  beforeEach(() => window.localStorage.clear())

  it('renders the toolbar only for a viewer that may control, and only once there is a picture', async () => {
    render()
    expect(bar()).toBeNull()
    ownAccount()
    expect(bar()).toBeNull() // connecting card
    await firstFrame()
    expect(bar()?.getAttribute('aria-label')).toBe('Desktop controls')
  })

  it('renders no toolbar for a viewer without the control right', async () => {
    render()
    act(() => client().emit('rights', { control: false, clipboard: false, surface: false }))
    await firstFrame()
    expect(bar()).toBeNull()
  })

  it('renders no toolbar on a phone', async () => {
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (q: string) => ({ matches: true, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
    )
    render()
    ownAccount()
    await firstFrame()
    expect(bar()).toBeNull()
  })

  it('Send Keys then Cmd+Tab puts one complete tap on the input channel', async () => {
    render()
    ownAccount()
    await firstFrame()
    client().sendInput.mockClear()
    act(() => sendKeysBtn().click())
    expect(sendKeysBtn().getAttribute('aria-expanded')).toBe('true')
    act(() => menuItem('Cmd+Tab').click())
    const keys = client().sendInput.mock.calls.map((c) => c[0] as { t: string; code?: string; d?: boolean; m?: number })
    expect(keys.filter((m) => m.t === 'key').map((m) => [m.code, m.d, m.m])).toEqual([
      ['MetaLeft', true, 8], ['Tab', true, 8], ['Tab', false, 8], ['MetaLeft', false, 0],
    ])
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(sendKeysBtn().getAttribute('aria-expanded')).toBe('false')
  })

  it('returns focus to the stage after the item when the stage had the keyboard, and to the button when it did not', async () => {
    render()
    ownAccount()
    await firstFrame()
    // The stage has the keyboard: a press on the picture captures it.
    act(() => {
      stage().dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 }))
    })
    expect(document.activeElement).toBe(input())
    act(() => {
      sendKeysBtn().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
      sendKeysBtn().click()
    })
    expect(document.activeElement).not.toBe(input()) // the menu took the keyboard
    act(() => menuItem('Cmd+Space').click())
    expect(document.activeElement).toBe(input())

    // The stage did not have it: the button gets focus back.
    act(() => input().blur())
    act(() => {
      sendKeysBtn().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
      sendKeysBtn().click()
    })
    act(() => menuItem('Cmd+Tab').click())
    expect(document.activeElement).toBe(sendKeysBtn())
  })
})

describe('surface picker and notices (Phase 20-07 Task 2)', () => {
  const picker = () => container.querySelector('.desktop-bar__picker') as HTMLButtonElement
  const notice = () => container.querySelector('.desktop-chip--notice')
  const radios = () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitemradio"]'))
  const MAIN = { e: 1, kind: 'display', id: 1, name: 'Main Display', w: 1470, h: 956, reason: '' } as const
  const surface = (over: Partial<DesktopEvents['surface']> = {}) =>
    act(() => client().emit('surface', { ...MAIN, ...over } as DesktopEvents['surface']))

  function ownAccount() {
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
  }
  async function picked(label: string) {
    await act(async () => { picker().click() })
    act(() => radios().find((r) => r.textContent?.startsWith(label))!.click())
  }

  beforeEach(() => window.localStorage.clear())

  it('choosing a window sends select and the label reads Switching… (greyed)', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    expect(picker().textContent).toBe('Main Display')
    await picked('Terminal')
    expect(client().selectSurface).toHaveBeenCalledWith('window', 4242)
    expect(picker().textContent).toBe('Switching…')
    expect(picker().getAttribute('aria-disabled')).toBe('true')
  })

  it('the surface message for that pick sets the label and shows Showing <name> with no failure', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    await picked('Terminal')
    surface({ e: 2, kind: 'window', id: 4242, name: 'Terminal — zsh', w: 1280, h: 800 })
    expect(picker().textContent).toBe('Terminal — zsh')
    expect(notice()?.textContent).toBe('Showing Terminal — zsh')
    expect(notice()?.getAttribute('role')).toBeNull()
    advance(5000)
    expect(notice()).toBeNull()
    expect(container.textContent).not.toContain('Reconnecting')
  })

  it('with no surface message within 5 s the label reverts and a failure notice reads Couldn\'t switch (role alert)', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    await picked('Terminal')
    advance(4999)
    expect(picker().textContent).toBe('Switching…')
    advance(1)
    expect(picker().textContent).toBe('Main Display')
    expect(notice()?.textContent).toBe("Couldn't switch to Terminal — zsh. Still showing Main Display.")
    expect(notice()?.getAttribute('role')).toBe('alert')
    advance(5000 + 150)
    expect(notice()).toBeNull()
  })

  it('select_failed reverts at once with the same notice', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    await picked('Display 2')
    act(() => client().emit('selectFailed', { code: 'switch_failed' }))
    expect(picker().textContent).toBe('Main Display')
    expect(notice()?.textContent).toBe("Couldn't switch to Display 2. Still showing Main Display.")
    advance(6000) // the old timer must not fire a second notice
    expect(notice()).toBeNull()
  })

  it('a stray select_failed with no pick in flight says nothing', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    act(() => client().emit('selectFailed', { code: 'busy' }))
    expect(notice()).toBeNull()
  })

  it.each([
    ['window_closed', 'The shared window closed; showing the main display'],
    ['display_gone', 'The shared display disconnected; showing the main display'],
  ] as const)('a surface message with reason %s shows its fallback notice for 5 s', async (reason, text) => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    surface({ e: 2, kind: 'window', id: 4242, name: 'Terminal — zsh' })
    surface({ e: 3, reason })
    expect(notice()?.textContent).toBe(text)
    advance(4999)
    expect(notice()).not.toBeNull()
    advance(1 + 150)
    expect(notice()).toBeNull()
    expect(picker().textContent).toBe('Main Display')
  })

  it('another viewer\'s switch shows Showing <name>; the main display reads Showing the main display; the first message says nothing', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    expect(notice()).toBeNull()
    surface({ e: 2, id: 2, name: 'Display 2', w: 1920, h: 1080 })
    expect(notice()?.textContent).toBe('Showing Display 2')
    surface({ e: 3 })
    expect(notice()?.textContent).toBe('Showing the main display')
  })

  it('a new epoch for the same surface (a failed switch told to everyone) says nothing', async () => {
    render()
    ownAccount()
    await firstFrame()
    surface()
    surface({ e: 2 })
    expect(notice()).toBeNull()
  })

  it('a phone sees the surface notices too, with no bar', async () => {
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (q: string) => ({ matches: true, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
    )
    render()
    act(() => client().emit('rights', { control: true, clipboard: true, surface: true }))
    await firstFrame()
    surface()
    surface({ e: 2, kind: 'window', id: 4242, name: 'Terminal — zsh' })
    expect(container.querySelector('[role="toolbar"]')).toBeNull()
    expect(notice()?.textContent).toBe('Showing Terminal — zsh')
    surface({ e: 3, reason: 'window_closed' })
    expect(notice()?.textContent).toBe('The shared window closed; showing the main display')
  })
})

describe('bar items wired to the view (Phase 20-07 Task 3)', () => {
  const btn = (name: string) => container.querySelector<HTMLButtonElement>(`button[aria-label="${name}"]`)!
  const radio = (name: string) => Array.from(container.querySelectorAll<HTMLElement>('[role="radio"]')).find((r) => r.textContent === name)!
  const notice = () => container.querySelector('.desktop-chip--notice')
  const stage = () => container.querySelector('.desktop-stage') as HTMLElement
  const input = () => container.querySelector('textarea') as HTMLTextAreaElement
  const ring = () => container.querySelector('.desktop-ring')
  const sentTypes = () => client().sendInput.mock.calls.map((c) => (c[0] as { t: string }).t)

  function ownAccount(rights = { control: true, clipboard: true, surface: true }) {
    for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
      Object.defineProperty(video(), a, { configurable: true, value: b })
    }
    act(() => client().emit('rights', rights))
  }
  const press = () =>
    act(() => {
      stage().dispatchEvent(new PointerEvent('pointerdown', { clientX: 500, clientY: 250, bubbles: true, button: 0, pointerId: 1 }))
    })

  beforeEach(() => window.localStorage.clear())
  afterEach(() => {
    delete (document as unknown as { fullscreenEnabled?: unknown }).fullscreenEnabled
    delete (document as unknown as { fullscreenElement?: unknown }).fullscreenElement
    delete (navigator as unknown as { keyboard?: unknown }).keyboard
  })

  it('chips sit below the bar row on a viewer that may control, and stay at the top elsewhere', () => {
    render()
    expect(container.querySelector('.desktop-view')?.classList.contains('desktop-view--bar')).toBe(false)
    ownAccount()
    expect(container.querySelector('.desktop-view')?.classList.contains('desktop-view--bar')).toBe(true)
  })

  it('the picker is absent when the hello grants no surface choice', async () => {
    render()
    ownAccount({ control: true, clipboard: false, surface: false })
    await firstFrame()
    expect(container.querySelector('.desktop-bar__picker')).toBeNull()
  })

  it('View Only releases and drops the ring, the stage goes to tabindex -1, Control returns it', async () => {
    render()
    ownAccount()
    await firstFrame()
    press()
    expect(ring()).not.toBeNull()
    client().sendInput.mockClear()
    act(() => radio('View Only').click())
    expect(sentTypes()).toContain('release')
    expect(ring()).toBeNull()
    expect(stage().getAttribute('tabindex')).toBe('-1')
    expect(radio('View Only').getAttribute('aria-checked')).toBe('true')
    expect(btn('Send Keys').getAttribute('title')).toBe('Turn on Control first')
    act(() => radio('Control').click())
    expect(stage().getAttribute('tabindex')).toBe('0')
  })

  it('the release chord moves focus to the Control segment and shows the bar', async () => {
    render()
    ownAccount()
    await firstFrame()
    advance(5200)
    expect(container.querySelector('.desktop-bar')?.classList.contains('desktop-bar--hidden')).toBe(true)
    press()
    act(() => {
      input().dispatchEvent(new KeyboardEvent('keydown', { code: 'Enter', key: 'Enter', ctrlKey: true, altKey: true, metaKey: true, bubbles: true, cancelable: true }))
    })
    expect(document.activeElement).toBe(radio('Control'))
    expect(container.querySelector('.desktop-bar')?.classList.contains('desktop-bar--hidden')).toBe(false)
  })

  it('Swap toggles the controller setting, remembers it, and shows its notice', async () => {
    render()
    ownAccount()
    await firstFrame()
    act(() => btn('Swap Ctrl and Cmd').click())
    expect(btn('Swap Ctrl and Cmd').getAttribute('aria-pressed')).toBe('true')
    expect(window.localStorage.getItem('portty:desktop-swap-ctrl-cmd')).toBe('1')
    expect(notice()?.textContent).toBe('Ctrl and Cmd are swapped')
    act(() => btn('Swap Ctrl and Cmd').click())
    expect(btn('Swap Ctrl and Cmd').getAttribute('aria-pressed')).toBe('false')
    expect(notice()?.textContent).toBe('Ctrl and Cmd are back to normal')
  })

  it('Swap starts on when this browser remembered it', async () => {
    window.localStorage.setItem('portty:desktop-swap-ctrl-cmd', '1')
    render()
    ownAccount()
    await firstFrame()
    expect(btn('Swap Ctrl and Cmd').getAttribute('aria-pressed')).toBe('true')
  })

  it('the Locked state greys what sends and keeps Swap live', async () => {
    render()
    ownAccount()
    state({ state: 'locked', viewers: 1, locked: true })
    await firstFrame()
    expect(btn('Send Keys').getAttribute('aria-disabled')).toBe('true')
    expect(btn('Send Keys').getAttribute('title')).toBe('Locked')
    expect(btn('Swap Ctrl and Cmd').getAttribute('aria-disabled')).toBeNull()
  })

  it('needs Accessibility greys Send Keys and the Control segment', async () => {
    render()
    ownAccount()
    await firstFrame()
    act(() => client().emit('control', { input: 'needs_accessibility', window: '' }))
    expect(btn('Send Keys').getAttribute('title')).toBe('Control needs Accessibility')
    expect(radio('Control').getAttribute('aria-disabled')).toBe('true')
    expect(radio('View Only').getAttribute('aria-disabled')).toBeNull()
  })

  describe('Full Screen', () => {
    function fullscreenSupport(enabled: boolean) {
      Object.defineProperty(document, 'fullscreenEnabled', { configurable: true, value: enabled })
    }
    const request = () => {
      const fn = vi.fn(() => Promise.resolve())
      Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', { configurable: true, writable: true, value: fn })
      return fn
    }

    it('is absent when the browser reports fullscreen unavailable', async () => {
      fullscreenSupport(false)
      render()
      ownAccount()
      await firstFrame()
      expect(btn('Full Screen')).toBeNull()
    })

    it('enters fullscreen on the viewer root, focuses the stage and, with Keyboard Lock granted, says so', async () => {
      fullscreenSupport(true)
      const req = request()
      const lock = vi.fn(() => Promise.resolve())
      Object.defineProperty(navigator, 'keyboard', { configurable: true, value: { lock, unlock: vi.fn() } })
      render()
      ownAccount()
      await firstFrame()
      await act(async () => { btn('Full Screen').click() })
      expect(req).toHaveBeenCalledTimes(1)
      expect(req.mock.contexts[0]).toBe(container.querySelector('.desktop-view'))
      expect(lock).toHaveBeenCalledTimes(1)
      expect(document.activeElement).toBe(input())
      expect(notice()?.textContent).toBe('Cmd and Esc shortcuts now go to the Mac. Hold Esc to leave full screen.')
      advance(4999)
      expect(notice()).not.toBeNull()
    })

    it.each([
      ['Keyboard Lock is refused', () => Object.defineProperty(navigator, 'keyboard', { configurable: true, value: { lock: () => Promise.reject(new Error('no')) } })],
      ['Keyboard Lock does not exist', () => {}],
    ])('when %s the notice says the browser keeps Cmd shortcuts, and it is not an error', async (_w, setup) => {
      fullscreenSupport(true)
      request()
      setup()
      render()
      ownAccount()
      await firstFrame()
      await act(async () => { btn('Full Screen').click() })
      expect(notice()?.textContent).toBe(
        'Full screen is on. This browser keeps Cmd shortcuts; use Send Keys for Cmd+Tab and Cmd+Space.',
      )
      expect(notice()?.getAttribute('role')).toBeNull()
    })

    it('leaving fullscreen releases held input and the keyboard lock', async () => {
      fullscreenSupport(true)
      request()
      const unlock = vi.fn()
      Object.defineProperty(navigator, 'keyboard', { configurable: true, value: { lock: () => Promise.resolve(), unlock } })
      render()
      ownAccount()
      await firstFrame()
      press()
      client().sendInput.mockClear()
      Object.defineProperty(document, 'fullscreenElement', { configurable: true, get: () => container })
      act(() => { document.dispatchEvent(new Event('fullscreenchange')) })
      expect(btn('Exit Full Screen')).not.toBeNull()
      expect(sentTypes()).not.toContain('release')
      Object.defineProperty(document, 'fullscreenElement', { configurable: true, get: () => null })
      act(() => { document.dispatchEvent(new Event('fullscreenchange')) })
      expect(sentTypes()).toContain('release')
      expect(unlock).toHaveBeenCalledTimes(1)
      expect(btn('Full Screen')).not.toBeNull()
    })
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
