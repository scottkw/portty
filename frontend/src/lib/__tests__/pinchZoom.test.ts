import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { attachPinchZoom } from '../pinchZoom'

// jsdom's TouchEvent is incomplete (jsdom#1508): capture the handlers off a mock container and
// call them with plain event objects, as touchScrollHandler.test.ts does.

type Listener = (e: unknown) => void
type Pt = { clientX: number; clientY: number }

const cleanups: Array<() => void> = []

function setup(size = 13) {
  const handlers: Record<string, Listener> = {}
  const opts: Record<string, unknown> = {}
  const container = {
    addEventListener: vi.fn((type: string, fn: Listener, o?: unknown) => { handlers[type] = fn; opts[type] = o }),
    removeEventListener: vi.fn(),
  } as unknown as HTMLElement
  const h = { getSize: vi.fn(() => size), setSize: vi.fn(), onEnd: vi.fn() }
  const cleanup = attachPinchZoom(container, h)
  cleanups.push(cleanup) // no listener may leak between tests
  return { handlers, opts, container, h, cleanup }
}

const two = (d: number): Pt[] => [{ clientX: 0, clientY: 0 }, { clientX: d, clientY: 0 }]
function ev(touches: Pt[]) {
  return { touches, preventDefault: vi.fn() }
}

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { vi.useRealTimers(); cleanups.splice(0).forEach((c) => c()) })

describe('attachPinchZoom', () => {
  it('spreading 100 to 200 px at size 13 sets 26, clamps at 32, and pinching in clamps at 6', () => {
    const { handlers, h } = setup(13)
    handlers.touchmove(ev(two(100))) // records d0
    handlers.touchmove(ev(two(200)))
    expect(h.setSize).toHaveBeenLastCalledWith(26)
    handlers.touchmove(ev(two(900)))
    expect(h.setSize).toHaveBeenLastCalledWith(32)
    handlers.touchmove(ev(two(1)))
    expect(h.setSize).toHaveBeenLastCalledWith(6)
  })

  it('calls setSize only when the whole-pixel size changes', () => {
    const { handlers, h } = setup(13)
    handlers.touchmove(ev(two(100)))
    handlers.touchmove(ev(two(101))) // 13.13 rounds to 13
    expect(h.setSize).not.toHaveBeenCalled()
    handlers.touchmove(ev(two(120))) // 15.6 rounds to 16
    handlers.touchmove(ev(two(121))) // 15.7 rounds to 16
    expect(h.setSize).toHaveBeenCalledTimes(1)
  })

  it('every two-touch move prevents default; one-finger moves do nothing', () => {
    const { handlers, h } = setup()
    const a = ev(two(100)); handlers.touchmove(a)
    const b = ev(two(150)); handlers.touchmove(b)
    expect(a.preventDefault).toHaveBeenCalled()
    expect(b.preventDefault).toHaveBeenCalled()
    const one = ev([{ clientX: 5, clientY: 5 }])
    handlers.touchmove(one)
    handlers.touchstart(one)
    handlers.touchend(one)
    expect(one.preventDefault).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1000)
    expect(h.onEnd).not.toHaveBeenCalled() // a one-finger touch is not a pinch
  })

  it('touchmove is registered non-passive', () => {
    const { opts } = setup()
    expect(opts.touchmove).toEqual({ passive: false })
  })

  it('onEnd fires once, 250 ms after the last finger lifts', () => {
    const { handlers, h } = setup()
    handlers.touchmove(ev(two(100)))
    handlers.touchmove(ev(two(200)))
    handlers.touchend(ev([{ clientX: 0, clientY: 0 }])) // one finger left
    vi.advanceTimersByTime(1000)
    expect(h.onEnd).not.toHaveBeenCalled()
    handlers.touchend(ev([]))
    vi.advanceTimersByTime(249)
    expect(h.onEnd).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(h.onEnd).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(1000)
    expect(h.onEnd).toHaveBeenCalledTimes(1)
  })

  it('a second pinch inside the window delays onEnd', () => {
    const { handlers, h } = setup()
    handlers.touchmove(ev(two(100)))
    handlers.touchend(ev([]))
    vi.advanceTimersByTime(200)
    handlers.touchstart(ev(two(100)))
    vi.advanceTimersByTime(300)
    expect(h.onEnd).not.toHaveBeenCalled()
    handlers.touchend(ev([]))
    vi.advanceTimersByTime(250)
    expect(h.onEnd).toHaveBeenCalledTimes(1)
  })

  it('a new pinch measures from the size at that time', () => {
    const { handlers, h } = setup(13)
    handlers.touchmove(ev(two(100)))
    handlers.touchmove(ev(two(200)))
    handlers.touchend(ev([]))
    h.getSize.mockReturnValue(26)
    handlers.touchmove(ev(two(100)))
    handlers.touchmove(ev(two(50)))
    expect(h.setSize).toHaveBeenLastCalledWith(13)
  })

  it('gesturestart is prevented on the terminal container only, never on the document', () => {
    const { handlers, container } = setup()
    const g = { preventDefault: vi.fn() }
    handlers.gesturestart(g)
    expect(g.preventDefault).toHaveBeenCalled()
    // Outside the terminal (the in-app PDF layer, the rest of the page) native pinch must survive.
    const outside = new Event('gesturestart', { cancelable: true })
    document.dispatchEvent(outside)
    expect(outside.defaultPrevented).toBe(false)
    expect((container.addEventListener as ReturnType<typeof vi.fn>).mock.calls.map((c) => c[0])).toContain('gesturestart')
  })

  it('the cleanup removes every listener, and a pending end dies with it', () => {
    const { handlers, container, h, cleanup } = setup()
    handlers.touchmove(ev(two(100)))
    handlers.touchend(ev([]))
    cleanup()
    vi.advanceTimersByTime(1000)
    expect(h.onEnd).not.toHaveBeenCalled()
    const removed = (container.removeEventListener as ReturnType<typeof vi.fn>).mock.calls.map((c) => c[0])
    expect(removed.sort()).toEqual(['gesturestart', 'touchcancel', 'touchend', 'touchmove', 'touchstart'])
  })
})
