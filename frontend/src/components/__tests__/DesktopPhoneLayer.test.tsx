// Phase 21-10 (DVWR-03) — the iPhone touch layer, driven through DesktopView's phone branch.
// The client is mocked with emitters; touches are plain events with a touches array (jsdom's TouchEvent is incomplete).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopEvents } from '../../lib/desktopClient'
import { normalizedDelta } from '../../lib/desktopCoords'
import { accelGain } from '../../lib/touchTrackpad'
import { DesktopView, videoContentRect } from '../DesktopView'

type Listener = (v: never) => void
interface MockClient {
  sendInput: ReturnType<typeof vi.fn>
  currentEpoch: ReturnType<typeof vi.fn>
  emit: <K extends keyof DesktopEvents>(ev: K, v: DesktopEvents[K]) => void
}

const h = vi.hoisted(() => ({ clients: [] as unknown[] }))

vi.mock('../../lib/desktopClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../lib/desktopClient')>()),
  createDesktopClient: vi.fn(() => {
    const ls = new Map<string, Set<Listener>>()
    const client = {
      start: vi.fn(),
      stop: vi.fn(),
      wake: vi.fn(),
      send: vi.fn(),
      sendInput: vi.fn(() => true),
      currentEpoch: vi.fn(() => 1),
      requestSurfaces: vi.fn(() => Promise.resolve({ displays: [], windows: [] })),
      selectSurface: vi.fn(),
      copyFromMac: vi.fn(() => Promise.resolve({ outcome: 'failed' })),
      pasteToMac: vi.fn(() => Promise.resolve({ outcome: 'ok' })),
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

const client = () => h.clients[h.clients.length - 1] as MockClient
const stage = () => container.querySelector('.desktop-stage') as HTMLElement
const video = () => container.querySelector('video') as HTMLVideoElement

const media = (matches: boolean) =>
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (q: string) => ({ matches, media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList,
  )

function sizeVideo() {
  for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500], ['videoWidth', 1600], ['videoHeight', 1000]] as const) {
    Object.defineProperty(video(), a, { configurable: true, value: b })
  }
  for (const [a, b] of [['clientWidth', 1000], ['clientHeight', 500]] as const) {
    Object.defineProperty(stage(), a, { configurable: true, value: b })
  }
}

function mount(rights: DesktopEvents['rights'] = { control: true, clipboard: true, surface: true }) {
  act(() => root.render(<DesktopView />))
  sizeVideo()
  act(() => client().emit('rights', rights))
}

let touchId = 0
const pt = (x: number, y: number) => ({ identifier: ++touchId, clientX: x, clientY: y })
function touch(type: 'touchstart' | 'touchmove' | 'touchend', touches: ReturnType<typeof pt>[], changed = touches) {
  const ev = new Event(type, { bubbles: true, cancelable: true })
  Object.assign(ev, { touches, changedTouches: changed })
  act(() => {
    stage().dispatchEvent(ev)
  })
  return ev
}
const sent = () => client().sendInput.mock.calls.map((c) => c[0] as { t: string; dx?: number; dy?: number; e?: number })
const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.useFakeTimers()
  h.clients.length = 0
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
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

describe('tracer: a one-finger drag on a phone sends a relative move', () => {
  it('a 40 px drag records rmove messages whose dx sums to normalizedDelta for the picture rect', () => {
    media(true)
    mount()
    const t = pt(0, 0)
    touch('touchstart', [t])
    for (let i = 1; i <= 10; i++) {
      advance(10)
      touch('touchmove', [{ ...t, clientX: i * 4, clientY: 0 }])
    }
    touch('touchend', [], [{ ...t, clientX: 40, clientY: 0 }])
    const moves = sent().filter((m) => m.t === 'rmove')
    expect(moves.length).toBeGreaterThan(0)
    expect(moves.every((m) => m.e === 1)).toBe(true)
    expect(client().sendInput.mock.calls.every((c) => c[1] === true)).toBe(true) // droppable
    const r = videoContentRect(1000, 500, 1600, 1000)
    const want = normalizedDelta(40 * accelGain(400), 0, 1, r.width, r.height).ndx
    expect(moves.reduce((a, m) => a + m.dx!, 0)).toBeCloseTo(want, 9)
    expect(moves.reduce((a, m) => a + m.dy!, 0)).toBe(0)
  })

  it('sends nothing while the epoch is unknown', () => {
    media(true)
    mount()
    client().currentEpoch.mockReturnValue(null)
    const t = pt(0, 0)
    touch('touchstart', [t])
    advance(10)
    touch('touchmove', [{ ...t, clientX: 30, clientY: 0 }])
    expect(sent()).toEqual([])
  })

  it('a mouse device gets no layer and sends no message, and the stage keeps no touch handlers', () => {
    media(false)
    mount()
    expect(container.querySelector('.desktop-view--phone')).toBeNull()
    const ev = touch('touchstart', [pt(0, 0)])
    expect(ev.defaultPrevented).toBe(false)
    touch('touchmove', [pt(30, 0)])
    expect(sent()).toEqual([])
  })

  it('a viewer without the control right sends nothing', () => {
    media(true)
    mount({ control: false, clipboard: false, surface: false })
    const t = pt(0, 0)
    touch('touchstart', [t])
    advance(10)
    touch('touchmove', [{ ...t, clientX: 30, clientY: 0 }])
    expect(sent()).toEqual([])
  })
})

// ─── Task 2: clicks, drag, right-click, scroll, zoom, pan-follow, cursor request, heartbeat, release ───

type Wire = { t: string; b?: number; d?: boolean; x?: number; y?: number; e?: number; ts?: number; dx?: number; dy?: number }
const wire = () => client().sendInput.mock.calls.map((c) => c[0] as Wire)
const btns = () => wire().filter((m) => m.t === 'btn')
const cursor = (x = 0.5, y = 0.25, e = 1) => act(() => client().emit('cursor', { x, y, e }))
const ripple = () => container.querySelector('.desktop-ripple') as HTMLElement

function tapAt(x = 100, y = 100, holdMs = 50) {
  const t = pt(x, y)
  touch('touchstart', [t])
  advance(holdMs)
  touch('touchend', [], [t])
}

function startDrag() {
  tapAt()
  advance(100)
  const t = pt(100, 100)
  touch('touchstart', [t])
  advance(10)
  touch('touchmove', [{ ...t, clientX: 130, clientY: 100 }])
  return t
}

describe('one-finger gestures reach the Mac as Phase 20 buttons', () => {
  beforeEach(() => {
    media(true)
    mount()
    cursor()
  })

  it('a tap is one left click at the reported cursor, stamped with the tap time', () => {
    tapAt(100, 100, 50)
    expect(btns()).toEqual([])
    advance(300)
    expect(btns()).toEqual([
      { t: 'btn', b: 0, d: true, x: 0.5, y: 0.25, e: 1, ts: 50 },
      { t: 'btn', b: 0, d: false, x: 0.5, y: 0.25, e: 1, ts: 50 },
    ])
  })

  it('a double-tap is two click pairs carrying the two real tap times', () => {
    tapAt(100, 100, 50) // lifts at 50
    advance(100)
    tapAt(102, 100, 50) // lifts at 200
    expect(btns().map((m) => [m.b, m.d, m.ts])).toEqual([[0, true, 50], [0, false, 50], [0, true, 200], [0, false, 200]])
  })

  it('a long press is a right click with a ripple that lasts 150 ms', () => {
    touch('touchstart', [pt(100, 100)])
    expect(ripple().hidden).toBe(true)
    advance(500)
    expect(btns().map((m) => [m.b, m.d])).toEqual([[2, true], [2, false]])
    expect(ripple().hidden).toBe(false)
    advance(149)
    expect(ripple().hidden).toBe(false)
    advance(1)
    expect(ripple().hidden).toBe(true)
  })

  it('tap then drag holds the left button for the drag and lets go on lift, with no click before it', () => {
    const t = startDrag()
    expect(btns().map((m) => [m.b, m.d])).toEqual([[0, true]])
    expect(wire().filter((m) => m.t === 'rmove').length).toBeGreaterThan(0)
    advance(10)
    touch('touchend', [], [{ ...t, clientX: 130, clientY: 100 }])
    expect(btns().map((m) => [m.b, m.d])).toEqual([[0, true], [0, false]])
  })

  it('the heartbeat runs every second while the button is held and stops after the lift', () => {
    const t = startDrag()
    client().sendInput.mockClear()
    advance(2000)
    expect(wire().filter((m) => m.t === 'hb')).toHaveLength(2)
    touch('touchend', [], [t])
    client().sendInput.mockClear()
    advance(3000)
    expect(wire()).toEqual([])
  })

  it.each([
    ['window blur', () => window.dispatchEvent(new Event('blur'))],
    ['pagehide', () => window.dispatchEvent(new Event('pagehide'))],
    [
      'visibilitychange to hidden',
      () => {
        vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
        document.dispatchEvent(new Event('visibilitychange'))
      },
    ],
    ['the input channel closing', () => client().emit('input', { open: false })],
  ])('%s releases what is held and ends the heartbeat', (_n, trigger) => {
    startDrag()
    client().sendInput.mockClear()
    act(() => {
      trigger()
    })
    expect(wire().filter((m) => m.t === 'release')).toHaveLength(1)
    client().sendInput.mockClear()
    advance(3000)
    expect(wire()).toEqual([])
  })

  it('a locked Mac gets no click and a reconnecting viewer none either', () => {
    act(() => client().emit('state', { state: 'locked', viewers: 1, locked: true }))
    tapAt()
    advance(400)
    expect(btns()).toEqual([])
  })
})

describe('the cursor request', () => {
  beforeEach(() => {
    media(true)
    mount()
  })

  it('when the input channel opens one zero rmove asks the Mac for its cursor', () => {
    act(() => client().emit('input', { open: true }))
    expect(wire()).toEqual([{ t: 'rmove', dx: 0, dy: 0, e: 1 }])
  })

  it('a tap before any report waits for it and clicks when it arrives within 500 ms', () => {
    tapAt()
    advance(300) // the tap intent
    client().sendInput.mockClear()
    advance(200)
    cursor(0.4, 0.6)
    expect(btns().map((m) => [m.d, m.x, m.y])).toEqual([[true, 0.4, 0.6], [false, 0.4, 0.6]])
  })

  it('a tap with no report within 500 ms is dropped: no click, nothing guessed', () => {
    tapAt()
    advance(300)
    advance(500)
    cursor()
    expect(btns()).toEqual([])
  })

  it('a report for another surface epoch is not used', () => {
    cursor(0.5, 0.25, 7)
    tapAt()
    advance(300)
    expect(btns()).toEqual([])
  })
})

describe('two fingers: scroll, pinch zoom, pan-follow, reset', () => {
  beforeEach(() => {
    media(true)
    mount()
    cursor()
  })

  const two = (ax: number, ay: number, bx: number, by: number) => [pt(ax, ay), pt(bx, by)]
  function pinchTo(spread: number, cx = 500, cy = 250) {
    const [a, b] = two(cx - 50, cy, cx + 50, cy)
    touch('touchstart', [a])
    touch('touchstart', [a, b])
    advance(10)
    touch('touchmove', [{ ...a, clientX: cx - spread / 2 }, { ...b, clientX: cx + spread / 2 }])
    return [a, b]
  }

  it('a two-finger drag up sends pixel wheel messages that move the content with the fingers, and no pointer motion', () => {
    const [a, b] = two(300, 300, 340, 300)
    touch('touchstart', [a])
    touch('touchstart', [a, b])
    for (let i = 1; i <= 3; i++) {
      advance(16)
      touch('touchmove', [{ ...a, clientY: 300 - i * 10 }, { ...b, clientY: 300 - i * 10 }])
    }
    const wheels = wire().filter((m) => m.t === 'wheel')
    expect(wheels.length).toBeGreaterThan(0)
    expect(wheels.reduce((n, m) => n + m.dy!, 0)).toBeCloseTo(30, 6)
    expect(wire().filter((m) => m.t === 'rmove')).toEqual([])
  })

  it('a pinch zooms the video locally around the centre and sends nothing', () => {
    client().sendInput.mockClear()
    const [a, b] = pinchTo(300)
    expect(video().style.transform).toBe('translate(-1000px, -500px) scale(3)')
    expect(video().style.transformOrigin).toBe('0 0')
    expect(video().style.willChange).toBe('transform')
    touch('touchend', [b], [a])
    expect(video().style.willChange).toBe('')
    expect(wire()).toEqual([])
  })

  it('zoom is clamped at 5x and the 1x video carries no transform at all', () => {
    pinchTo(900)
    expect(video().style.transform).toContain('scale(5)')
    expect(video().style.transform).not.toContain('scale(6)')
    expect(video().style.transform).not.toContain('scale(9)')
  })

  it('two two-finger taps return the video to 1x', () => {
    const [pa, pb] = pinchTo(200)
    touch('touchend', [pb], [pa])
    touch('touchend', [], [pb])
    expect(video().style.transform).not.toBe('')
    const tapTwo = () => {
      const [a, b] = two(480, 250, 520, 250)
      touch('touchstart', [a])
      touch('touchstart', [a, b])
      advance(40)
      touch('touchend', [b], [a])
      touch('touchend', [], [b])
    }
    advance(1000)
    tapTwo()
    advance(100)
    tapTwo()
    expect(video().style.transform).toBe('')
  })

  it('while zoomed, a cursor report near the edge moves the view to keep the pointer 32 px inside', () => {
    pinchTo(200) // 2x around the middle: translate(-500px, -250px)
    expect(video().style.transform).toBe('translate(-500px, -250px) scale(2)')
    cursor(1, 1)
    expect(video().style.transform).toBe('translate(-832px, -500px) scale(2)')
  })

  it('a rotation resets the zoom to 1x', () => {
    pinchTo(200)
    expect(video().style.transform).not.toBe('')
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 844 })
    act(() => {
      window.dispatchEvent(new Event('resize'))
    })
    expect(video().style.transform).toBe('')
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
    Object.defineProperty(window, 'innerHeight', { configurable: true, value: 768 })
  })
})

// ─── Task 3: bars, soft keyboard, armed modifiers, viewport ───

describe('the soft keyboard and the bars', () => {
  class StubVV extends EventTarget {
    height = 768
    offsetTop = 0
  }
  let vv: StubVV
  let frames: Array<() => void>
  const flush = () => act(() => { frames.splice(0).forEach((cb) => cb()) })
  const raiseKeyboard = () => {
    vv.height = 400
    act(() => { vv.dispatchEvent(new Event('resize')) })
    flush()
  }
  const lowerKeyboard = () => {
    vv.height = 768
    act(() => { vv.dispatchEvent(new Event('resize')) })
    flush()
  }
  const field = () => container.querySelector('.desktop-phone__input') as HTMLInputElement
  const toolbar = () => container.querySelector('[role="toolbar"]') as HTMLElement | null
  const key = (name: string) => container.querySelector(`button[aria-label="${name}"]`) as HTMLButtonElement
  const byText = (t: string) => Array.from(container.querySelectorAll('button')).find((b) => b.textContent === t)!
  const press = (el: HTMLElement) => act(() => { el.dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, cancelable: true })) })
  const typed = (data: string) => {
    const ev = new InputEvent('beforeinput', { inputType: 'insertText', data, cancelable: true, bubbles: true })
    act(() => { field().dispatchEvent(ev) })
    return ev
  }
  const keys = () => wire().filter((m) => m.t === 'key') as unknown as { code: string; d: boolean; m: number }[]
  const texts = () => wire().filter((m) => m.t === 'text').map((m) => (m as unknown as { s: string }).s)

  beforeEach(() => {
    vv = new StubVV()
    frames = []
    Object.defineProperty(window, 'visualViewport', { value: vv, configurable: true })
    vi.stubGlobal('requestAnimationFrame', (cb: () => void) => { frames.push(cb); return frames.length })
    vi.stubGlobal('cancelAnimationFrame', () => {})
    media(true)
    mount()
    flush()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    Reflect.deleteProperty(window, 'visualViewport')
  })

  it('the hidden field is a real input with iOS-safe attributes, and the root follows the visual viewport', () => {
    expect(field().getAttribute('aria-label')).toBe('Type on the Mac')
    expect(field().getAttribute('autocorrect')).toBe('off')
    expect(field().getAttribute('autocapitalize')).toBe('off')
    expect(field().getAttribute('spellcheck')).toBe('false')
    expect(field().closest('.desktop-stage')).not.toBeNull()
    expect((container.querySelector('.desktop-view') as HTMLElement).style.getPropertyValue('--vv-height')).toBe('768px')
  })

  it('Show Keyboard focuses the field inside its click and the slot flips to the key bar when the keyboard is up', () => {
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
    act(() => byText('Show Keyboard').click())
    expect(document.activeElement).toBe(field())
    raiseKeyboard()
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop keys')
    lowerKeyboard()
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
  })

  it('typed text goes as a Phase 20 text message and is kept out of the field', () => {
    const ev = typed('hi')
    expect(ev.defaultPrevented).toBe(true)
    expect(texts()).toEqual(['hi'])
    expect(keys()).toEqual([])
  })

  it('edge: a 3000-byte composed string goes out as three text messages that concatenate back', () => {
    const s = 'x'.repeat(3000)
    act(() => {
      field().dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
      field().dispatchEvent(new CompositionEvent('compositionend', { data: s, bubbles: true }))
    })
    expect(texts()).toHaveLength(3)
    expect(texts().join('')).toBe(s)
  })

  it('edge: Cmd armed, typing c sends key KeyC with the Meta bit and no text, then Cmd disarms', () => {
    raiseKeyboard()
    press(byText('Cmd'))
    expect(byText('Cmd').getAttribute('aria-pressed')).toBe('true')
    typed('c')
    const c = keys().find((k) => k.code === 'KeyC' && k.d)!
    expect(c.m & 8).toBe(8)
    expect(texts()).toEqual([])
    expect(byText('Cmd').getAttribute('aria-pressed')).toBe('false')
    typed('d') // the next character is plain text again
    expect(texts()).toEqual(['d'])
  })

  it('Ctrl and Opt armed together both apply to the next key from the bar, then disarm', () => {
    raiseKeyboard()
    press(byText('Ctrl'))
    press(byText('Opt'))
    expect(byText('Ctrl').getAttribute('aria-pressed')).toBe('true')
    expect(byText('Opt').getAttribute('aria-pressed')).toBe('true')
    press(key('Left arrow'))
    const left = keys().find((k) => k.code === 'ArrowLeft' && k.d)!
    expect(left.m).toBe(2 | 4)
    expect(byText('Ctrl').getAttribute('aria-pressed')).toBe('false')
    expect(byText('Opt').getAttribute('aria-pressed')).toBe('false')
  })

  it('tapping an armed modifier disarms it, and the keyboard going down disarms every one', () => {
    raiseKeyboard()
    press(byText('Cmd'))
    press(byText('Cmd'))
    expect(byText('Cmd').getAttribute('aria-pressed')).toBe('false')
    press(byText('Cmd'))
    press(byText('Ctrl'))
    client().sendInput.mockClear()
    lowerKeyboard()
    raiseKeyboard()
    expect(byText('Cmd').getAttribute('aria-pressed')).toBe('false')
    expect(byText('Ctrl').getAttribute('aria-pressed')).toBe('false')
    expect(wire().filter((m) => m.t === 'release').length).toBeGreaterThan(0)
  })

  it('a key bar key sends one down and one up and nothing else', () => {
    raiseKeyboard()
    client().sendInput.mockClear()
    press(byText('Esc'))
    expect(keys().map((k) => [k.code, k.d, k.m])).toEqual([['Escape', true, 0], ['Escape', false, 0]])
  })

  it('Backspace is observable through the sentinel: beforeinput and keydown each send one Backspace and the sentinel stays', () => {
    const sentinel = field().value
    expect(sentinel.length).toBe(1)
    act(() => {
      field().dispatchEvent(new InputEvent('beforeinput', { inputType: 'deleteContentBackward', cancelable: true, bubbles: true }))
    })
    expect(keys().filter((k) => k.code === 'Backspace' && k.d)).toHaveLength(1)
    expect(field().value).toBe(sentinel)
    act(() => {
      field().dispatchEvent(new KeyboardEvent('keydown', { key: 'Backspace', cancelable: true, bubbles: true }))
    })
    expect(keys().filter((k) => k.code === 'Backspace' && k.d)).toHaveLength(2)
  })

  it('edge: a deleted sentinel with no usable beforeinput still sends Backspace once and restores the sentinel', () => {
    const sentinel = field().value
    act(() => {
      field().value = ''
      field().dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect(keys().filter((k) => k.code === 'Backspace' && k.d)).toHaveLength(1)
    expect(field().value).toBe(sentinel)
  })

  it('an uncancellable beforeinput does not send Backspace a second time from its input event', () => {
    act(() => {
      field().dispatchEvent(new InputEvent('beforeinput', { inputType: 'deleteContentBackward', cancelable: false, bubbles: true }))
      field().value = ''
      field().dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect(keys().filter((k) => k.code === 'Backspace' && k.d)).toHaveLength(1)
  })

  it('Return sends Enter', () => {
    const ev = new KeyboardEvent('keydown', { key: 'Enter', cancelable: true, bubbles: true })
    act(() => { field().dispatchEvent(ev) })
    expect(ev.defaultPrevented).toBe(true)
    expect(keys().map((k) => [k.code, k.d])).toEqual([['Enter', true], ['Enter', false]])
  })

  it('the keyboard going down sends release', () => {
    raiseKeyboard()
    client().sendInput.mockClear()
    lowerKeyboard()
    expect(wire().filter((m) => m.t === 'release')).toHaveLength(1)
  })

  it('a locked Mac gets no typed text and no key', () => {
    act(() => client().emit('state', { state: 'locked', viewers: 1, locked: true }))
    typed('a')
    expect(wire()).toEqual([])
  })

  it('with no control right there is no field, no Show Keyboard and no Send Keys, but pinch still zooms', () => {
    act(() => client().emit('rights', { control: false, clipboard: false, surface: true }))
    act(() => client().emit('surface', { e: 1, kind: 'display', id: 1, name: 'Main Display', w: 1470, h: 956, reason: '' }))
    expect(field()).toBeNull()
    expect(Array.from(container.querySelectorAll('button')).map((b) => b.getAttribute('aria-label') ?? b.textContent)).toEqual(['Main Display'])
    const a = pt(450, 250)
    const b = pt(550, 250)
    touch('touchstart', [a])
    touch('touchstart', [a, b])
    advance(10)
    touch('touchmove', [{ ...a, clientX: 350 }, { ...b, clientX: 650 }])
    expect(video().style.transform).toContain('scale(3)')
    expect(Array.from(container.querySelectorAll('button')).some((x) => x.getAttribute('aria-label') === 'Fit to screen, zoom 3x')).toBe(true)
    expect(wire()).toEqual([])
  })

  it('Tap to start shows its card with the bottom bar visible under it, and no Wake Lock is requested', async () => {
    const request = vi.fn()
    Object.defineProperty(navigator, 'wakeLock', { value: { request }, configurable: true })
    vi.mocked(HTMLMediaElement.prototype.play).mockRejectedValue(new Error('NotAllowedError'))
    act(() => client().emit('status', { kind: 'streaming' }))
    await act(async () => { client().emit('track', {} as MediaStream) })
    expect(container.querySelector('.desktop-card__heading')?.textContent).toBe('Tap to start')
    expect(toolbar()?.getAttribute('aria-label')).toBe('Desktop controls')
    expect(request).not.toHaveBeenCalled()
    Reflect.deleteProperty(navigator, 'wakeLock')
  })

  it('the one video element survives a reconnect', () => {
    const v = video()
    act(() => client().emit('status', { kind: 'reconnecting' }))
    act(() => client().emit('status', { kind: 'streaming' }))
    expect(video()).toBe(v)
  })
})
