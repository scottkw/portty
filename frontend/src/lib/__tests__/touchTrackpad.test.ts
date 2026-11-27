import { describe, it, expect, vi } from 'vitest'
import {
  accelGain,
  attachTrackpad,
  createTrackpad,
  type TouchPoint,
  type TrackpadClock,
  type TrackpadIntent,
} from '../touchTrackpad'
import { normalizedDelta } from '../desktopCoords'

// jsdom's TouchEvent is incomplete (jsdom#1508): drive plain event objects, as pinchZoom.test.ts does.

type Listener = (e: unknown) => void

interface FakeClock extends TrackpadClock {
  /** Move time forward, firing every timer that falls due on the way. */
  advance(ms: number): void
  /** Move time forward without firing timers (an event that beats an overdue timer). */
  jump(ms: number): void
  pending(): number
}

function fakeClock(): FakeClock {
  let t = 0
  let seq = 0
  const timers = new Map<number, { due: number; fn: () => void }>()
  return {
    now: () => t,
    setTimeout(fn, ms) {
      const id = ++seq
      timers.set(id, { due: t + ms, fn })
      return id
    },
    clearTimeout(id) {
      timers.delete(id as number)
    },
    advance(ms) {
      const target = t + ms
      for (;;) {
        let next: [number, { due: number; fn: () => void }] | undefined
        for (const e of timers) if (e[1].due <= target && (!next || e[1].due < next[1].due)) next = e
        if (!next) break
        timers.delete(next[0])
        t = Math.max(t, next[1].due)
        next[1].fn()
      }
      t = target
    },
    jump(ms) {
      t += ms
    },
    pending: () => timers.size,
  }
}

const pt = (x: number, y: number, id = 0): TouchPoint => ({ identifier: id, clientX: x, clientY: y })
const ev = (...touches: TouchPoint[]) => ({ touches, changedTouches: touches })

function rig() {
  const clock = fakeClock()
  const intents: TrackpadIntent[] = []
  const pad = createTrackpad((i) => intents.push(i), clock)
  return { clock, intents, pad }
}

describe('attachTrackpad listeners', () => {
  function mockContainer() {
    const added: Array<{ type: string; fn: Listener; opts: unknown }> = []
    const removed: Array<{ type: string; fn: Listener }> = []
    const container = {
      addEventListener: vi.fn((type: string, fn: Listener, opts?: unknown) => added.push({ type, fn, opts })),
      removeEventListener: vi.fn((type: string, fn: Listener) => removed.push({ type, fn })),
    } as unknown as HTMLElement
    return { container, added, removed }
  }

  it('registers non-passive touch listeners plus a gesturestart guard on the container only', () => {
    const winAdd = vi.spyOn(window, 'addEventListener')
    const docAdd = vi.spyOn(document, 'addEventListener')
    const { container, added } = mockContainer()
    const dispose = attachTrackpad(container, () => {})
    expect(added.map((a) => a.type).sort()).toEqual(['gesturestart', 'touchcancel', 'touchend', 'touchmove', 'touchstart'])
    for (const a of added.filter((x) => x.type.startsWith('touch'))) expect(a.opts).toEqual({ passive: false })
    expect(winAdd).not.toHaveBeenCalled()
    expect(docAdd).not.toHaveBeenCalled()
    dispose()
    winAdd.mockRestore()
    docAdd.mockRestore()
  })

  it('the disposer removes every listener it added', () => {
    const { container, added, removed } = mockContainer()
    attachTrackpad(container, () => {})()
    expect(removed.map((r) => `${r.type}`).sort()).toEqual(added.map((a) => a.type).sort())
    for (const a of added) expect(removed.some((r) => r.type === a.type && r.fn === a.fn)).toBe(true)
  })

  it('a drag through the real listener path becomes accelerated move intents; touches are prevented', () => {
    const clock = fakeClock()
    const { container, added } = mockContainer()
    const intents: TrackpadIntent[] = []
    attachTrackpad(container, (i) => intents.push(i), clock)
    const h = (type: string): Listener => added.find((a) => a.type === type)!.fn
    const e = (...touches: TouchPoint[]) => ({ touches, changedTouches: touches, preventDefault: vi.fn() })
    const start = e(pt(0, 0))
    h('touchstart')(start)
    expect(start.preventDefault).toHaveBeenCalled()
    for (let i = 1; i <= 10; i++) {
      clock.advance(10)
      const m = e(pt(i * 4, 0))
      h('touchmove')(m)
      expect(m.preventDefault).toHaveBeenCalled()
    }
    h('touchend')({ touches: [], changedTouches: [pt(40, 0)], preventDefault: vi.fn() })
    const sum = intents.reduce((a, i) => a + (i.kind === 'move' ? i.dx : 0), 0)
    expect(sum).toBeCloseTo(40 * accelGain(400), 6)
  })
})

describe('one-finger drag moves', () => {
  it('a 40 px drag over 100 ms (400 px/s) sums to 40 * accelGain(400), nothing before 8 px', () => {
    const { clock, intents, pad } = rig()
    pad.start(ev(pt(0, 0)))
    clock.advance(10)
    pad.move(ev(pt(4, 0)))
    clock.advance(10)
    pad.move(ev(pt(8, 0))) // exactly 8 px is still a tap-sized wobble
    expect(intents).toEqual([])
    for (let i = 3; i <= 10; i++) {
      clock.advance(10)
      pad.move(ev(pt(i * 4, 0)))
    }
    const moves = intents.filter((i): i is Extract<TrackpadIntent, { kind: 'move' }> => i.kind === 'move')
    expect(moves.length).toBeGreaterThan(0)
    expect(moves.reduce((a, m) => a + m.dx, 0)).toBeCloseTo(40 * accelGain(400), 6)
    expect(moves.every((m) => m.dy === 0)).toBe(true)
  })

  it('accelGain is 1 below 200 px/s, linear to 2.5 at 1500 px/s, flat above', () => {
    expect(accelGain(0)).toBe(1)
    expect(accelGain(200)).toBe(1)
    expect(accelGain(850)).toBeCloseTo(1.75, 6)
    expect(accelGain(1500)).toBe(2.5)
    expect(accelGain(9000)).toBe(2.5)
  })

  it('maps through normalizedDelta to the zoom-scaled fraction of the picture', () => {
    expect(normalizedDelta(40, 0, 1, 800, 500).ndx).toBeCloseTo(0.05, 9)
    expect(normalizedDelta(40, 0, 2, 800, 500).ndx).toBeCloseTo(0.025, 9)
  })
})

// One finger down at (x,y), optionally wobbling, held for holdMs, then lifted.
function touchFor(r: ReturnType<typeof rig>, holdMs: number, x = 100, y = 100, wobblePx = 0): void {
  r.pad.start(ev(pt(x, y)))
  if (wobblePx) r.pad.move(ev(pt(x + wobblePx, y)))
  r.clock.advance(holdMs)
  r.pad.end({ touches: [], changedTouches: [pt(x + wobblePx, y)] })
}
const kinds = (i: TrackpadIntent[]): string[] => i.map((x) => x.kind)

describe('tap, double-tap, long-press and tap-then-drag', () => {
  it('a 120 ms touch that moved 5 px is one tap, emitted only after the 300 ms window', () => {
    const r = rig()
    touchFor(r, 120, 100, 100, 5)
    r.clock.advance(299)
    expect(r.intents).toEqual([])
    r.clock.advance(1)
    expect(r.intents).toEqual([{ kind: 'tap', at: 120 }])
    r.clock.advance(1000)
    expect(r.intents).toHaveLength(1)
  })

  it('two taps 200 ms and 10 px apart are exactly one doubleTap with both tap times and no tap', () => {
    const r = rig()
    touchFor(r, 100, 100, 100) // lifts at 100
    r.clock.advance(200)
    touchFor(r, 100, 110, 100) // lifts at 400
    r.clock.advance(1000)
    expect(r.intents).toEqual([{ kind: 'doubleTap', at: [100, 400] }])
  })

  it('a second tap 350 ms after the first is two separate taps', () => {
    const r = rig()
    touchFor(r, 100)
    r.clock.advance(350)
    touchFor(r, 100)
    r.clock.advance(1000)
    expect(kinds(r.intents)).toEqual(['tap', 'tap'])
  })

  it('a second touch 150 ms after a tap that moves 30 px is dragStart, dragMove, dragEnd and no click', () => {
    const r = rig()
    touchFor(r, 100)
    r.clock.advance(150)
    r.pad.start(ev(pt(100, 100)))
    for (let i = 1; i <= 6; i++) {
      r.clock.advance(20)
      r.pad.move(ev(pt(100 + i * 5, 100)))
    }
    r.pad.end({ touches: [], changedTouches: [pt(130, 100)] })
    r.clock.advance(1000)
    const k = kinds(r.intents)
    expect(k[0]).toBe('dragStart')
    expect(k.slice(1, -1).length).toBeGreaterThan(0)
    expect(k.slice(1, -1).every((x) => x === 'dragMove')).toBe(true)
    expect(k[k.length - 1]).toBe('dragEnd')
    expect(k).not.toContain('tap')
    expect(k).not.toContain('doubleTap')
    const sum = r.intents.reduce((a, i) => a + (i.kind === 'dragMove' ? i.dx : 0), 0)
    expect(sum).toBeGreaterThanOrEqual(30) // gain is never below 1
  })

  it('M-195: a second touch after a tap that rests 600 ms before it moves is a drag from 500 ms: no tap, no right click', () => {
    const r = rig()
    touchFor(r, 100)
    r.clock.advance(150)
    r.pad.start(ev(pt(100, 100)))
    r.pad.move(ev(pt(102, 100))) // resting jitter
    r.clock.advance(499)
    expect(r.intents).toEqual([])
    r.clock.advance(1)
    expect(r.intents).toEqual([{ kind: 'dragStart', at: 750 }])
    r.clock.advance(100)
    for (let i = 1; i <= 6; i++) {
      r.clock.advance(20)
      r.pad.move(ev(pt(100 + i * 5, 100)))
    }
    r.pad.end({ touches: [], changedTouches: [pt(130, 100)] })
    r.clock.advance(1000)
    const k = kinds(r.intents)
    expect(k[0]).toBe('dragStart')
    expect(k.slice(1, -1).length).toBeGreaterThan(0)
    expect(k.slice(1, -1).every((x) => x === 'dragMove')).toBe(true)
    expect(k[k.length - 1]).toBe('dragEnd')
    expect(k).not.toContain('tap')
    expect(k).not.toContain('longPress')
  })

  it('M-195: a second touch after a tap held still and lifted is a press and a release, nothing else', () => {
    const r = rig()
    touchFor(r, 100)
    r.clock.advance(150)
    r.pad.start(ev(pt(100, 100)))
    r.clock.advance(600)
    r.pad.end({ touches: [], changedTouches: [pt(100, 100)] })
    r.clock.advance(1000)
    expect(kinds(r.intents)).toEqual(['dragStart', 'dragEnd'])
  })

  it('held 520 ms with 3 px of movement is one longPress at 500 ms and the lift adds nothing', () => {
    const r = rig()
    r.pad.start(ev(pt(100, 100)))
    r.pad.move(ev(pt(103, 100)))
    r.clock.advance(499)
    expect(r.intents).toEqual([])
    r.clock.advance(1)
    expect(r.intents).toEqual([{ kind: 'longPress', at: 500 }])
    r.clock.advance(20)
    r.pad.end({ touches: [], changedTouches: [pt(103, 100)] })
    r.clock.advance(1000)
    expect(r.intents).toHaveLength(1)
  })

  it('a 12 px move before 500 ms cancels the long press and becomes a move', () => {
    const r = rig()
    r.pad.start(ev(pt(100, 100)))
    r.clock.advance(100)
    r.pad.move(ev(pt(112, 100)))
    r.clock.advance(1000)
    expect(kinds(r.intents)).toEqual(['move'])
  })

  it('boundaries: 8 px and 250 ms still tap; 9 px or 251 ms do not', () => {
    let r = rig()
    touchFor(r, 250, 100, 100, 8)
    r.clock.advance(300)
    expect(kinds(r.intents)).toEqual(['tap'])

    r = rig()
    touchFor(r, 251)
    r.clock.advance(1000)
    expect(r.intents).toEqual([])

    r = rig()
    touchFor(r, 100, 100, 100, 9)
    r.clock.advance(1000)
    expect(kinds(r.intents)).toEqual(['move'])
  })

  it('boundaries: a second touch at 300 ms and 24 px still double-taps', () => {
    const r = rig()
    touchFor(r, 100) // lifts at 100, tap timer due at 400
    r.clock.advance(299)
    r.clock.jump(1) // the touch beats the overdue timer: exactly 300 ms after the lift
    touchFor(r, 100, 124, 100) // 24 px away
    r.clock.advance(1000)
    expect(kinds(r.intents)).toEqual(['doubleTap'])
  })

  it('a second tap farther than 24 px is two taps, not a double-tap', () => {
    const r = rig()
    touchFor(r, 100)
    r.clock.advance(100)
    touchFor(r, 100, 125, 100)
    r.clock.advance(1000)
    expect(kinds(r.intents)).toEqual(['tap', 'tap'])
  })

  it('cancel() in every pending state clears the timers and emits nothing', () => {
    let r = rig()
    r.pad.start(ev(pt(100, 100))) // pending long press
    r.pad.cancel()
    expect(r.clock.pending()).toBe(0)

    r = rig()
    touchFor(r, 100) // pending single tap
    r.pad.cancel()
    expect(r.clock.pending()).toBe(0)

    r = rig()
    touchFor(r, 100)
    r.pad.start(ev(pt(100, 100))) // second touch after a tap
    r.pad.cancel()
    expect(r.clock.pending()).toBe(0)
    r.clock.advance(5000)
    expect(r.intents).toEqual([])
  })
})

// Two fingers d px apart, centred on (cx, cy). Ids 0 and 1.
const pair = (d: number, cx = 200, cy = 200): TouchPoint[] => [pt(cx - d / 2, cy, 0), pt(cx + d / 2, cy, 1)]
// Put two fingers down the way iOS reports it: one touchstart per finger.
function twoDown(r: ReturnType<typeof rig>, d = 60, cx = 200, cy = 200): void {
  const [a, b] = pair(d, cx, cy)
  r.pad.start(ev(a))
  r.pad.start(ev(a, b))
}
const twoUp = (r: ReturnType<typeof rig>): void => {
  r.pad.end({ touches: [pt(200, 200, 0)], changedTouches: [pt(230, 200, 1)] })
  r.pad.end({ touches: [], changedTouches: [pt(200, 200, 0)] })
}
const sumOf = (i: TrackpadIntent[], kind: 'scroll', f: 'dx' | 'dy'): number =>
  i.reduce((a, x) => a + (x.kind === kind ? x[f] : 0), 0)

describe('two fingers: scroll versus pinch', () => {
  it('moving together 30 px while the distance changes 4 px is scroll only, summing to the centroid travel', () => {
    const r = rig()
    twoDown(r)
    for (let i = 1; i <= 3; i++) {
      r.clock.advance(16)
      r.pad.move(ev(...pair(60 + i * (4 / 3), 200 + i * 10, 200)))
    }
    twoUp(r)
    expect(r.intents.length).toBeGreaterThan(0)
    expect(r.intents.every((i) => i.kind === 'scroll')).toBe(true)
    expect(sumOf(r.intents, 'scroll', 'dx')).toBeCloseTo(30, 6)
    expect(sumOf(r.intents, 'scroll', 'dy')).toBeCloseTo(0, 6)
  })

  it('spreading 40 px while the centroid moves 3 px is pinch intents then pinchEnd, never a scroll', () => {
    const r = rig()
    twoDown(r, 60)
    for (let i = 1; i <= 4; i++) {
      r.clock.advance(16)
      r.pad.move(ev(...pair(60 + i * 10, 200 + i * 0.75, 200)))
    }
    twoUp(r)
    const k = kinds(r.intents)
    expect(k.length).toBeGreaterThan(1)
    expect(k.slice(0, -1).every((x) => x === 'pinch')).toBe(true)
    expect(k[k.length - 1]).toBe('pinchEnd')
    const last = r.intents.filter((i): i is Extract<TrackpadIntent, { kind: 'pinch' }> => i.kind === 'pinch').pop()!
    expect(last.scale).toBeCloseTo(100 / 60, 6) // relative to the start of the gesture
    expect(last.cx).toBeCloseTo(203, 6)
    expect(last.cy).toBeCloseTo(200, 6)
  })

  it('the choice is made once: a pinch that later travels far never scrolls', () => {
    const r = rig()
    twoDown(r, 60)
    r.pad.move(ev(...pair(80, 200, 200))) // distance +20: pinch
    r.pad.move(ev(...pair(80, 300, 260))) // then the pair travels 100+ px
    expect(kinds(r.intents)).toEqual(['pinch', 'pinch'])
  })

  it('exactly 12 px of distance change decides nothing; 13 px is a pinch', () => {
    let r = rig()
    twoDown(r, 60)
    r.pad.move(ev(...pair(72)))
    expect(r.intents).toEqual([])
    r = rig()
    twoDown(r, 60)
    r.pad.move(ev(...pair(73)))
    expect(kinds(r.intents)).toEqual(['pinch'])
  })

  it('a pinch gives no scroll and no pointer intent, and a finger left down after it does nothing', () => {
    const r = rig()
    twoDown(r, 60)
    r.pad.move(ev(...pair(120)))
    r.pad.end({ touches: [pt(170, 200, 0)], changedTouches: [pt(230, 200, 1)] })
    r.pad.move(ev(pt(300, 300, 0)))
    r.pad.end({ touches: [], changedTouches: [pt(300, 300, 0)] })
    r.clock.advance(1000)
    const k = kinds(r.intents)
    expect(k).toEqual(['pinch', 'pinchEnd'])
  })

  it('two two-finger taps within 300 ms are one resetZoom; one tap emits nothing', () => {
    let r = rig()
    twoDown(r)
    r.clock.advance(80)
    twoUp(r)
    r.clock.advance(2000)
    expect(r.intents).toEqual([])

    r = rig()
    twoDown(r)
    r.clock.advance(80)
    twoUp(r)
    r.clock.advance(150)
    twoDown(r)
    r.clock.advance(80)
    twoUp(r)
    r.clock.advance(2000)
    expect(r.intents).toEqual([{ kind: 'resetZoom' }])
  })

  it('two-finger taps more than 300 ms apart do not reset', () => {
    const r = rig()
    twoDown(r)
    r.clock.advance(80)
    twoUp(r)
    r.clock.advance(400)
    twoDown(r)
    r.clock.advance(80)
    twoUp(r)
    expect(r.intents).toEqual([])
  })

  it('a third finger cancels the gesture with no intent, and the next gesture still works', () => {
    const r = rig()
    twoDown(r)
    const [a, b] = pair(60)
    r.pad.start(ev(a, b, pt(220, 260, 2)))
    r.pad.move(ev(...pair(120)))
    r.pad.move(ev(a, b, pt(300, 300, 2)))
    r.pad.end({ touches: [a, b], changedTouches: [pt(220, 260, 2)] })
    r.pad.end({ touches: [a], changedTouches: [b] })
    r.pad.end({ touches: [], changedTouches: [a] })
    expect(r.intents).toEqual([])
    touchFor(r, 100)
    r.clock.advance(400)
    expect(kinds(r.intents)).toEqual(['tap'])
  })

  it('a third finger or cancel() during a pinch ends it, so the caller can drop will-change', () => {
    let r = rig()
    twoDown(r, 60)
    r.pad.move(ev(...pair(120)))
    r.pad.start(ev(...pair(120), pt(10, 10, 2)))
    expect(kinds(r.intents)).toEqual(['pinch', 'pinchEnd'])

    r = rig()
    twoDown(r, 60)
    r.pad.move(ev(...pair(120)))
    r.pad.cancel()
    expect(kinds(r.intents)).toEqual(['pinch', 'pinchEnd'])
  })

  it('a second finger landing during a drag releases the button first', () => {
    const r = rig()
    touchFor(r, 100)
    r.clock.advance(100)
    r.pad.start(ev(pt(100, 100, 0)))
    r.pad.move(ev(pt(130, 100, 0)))
    r.pad.start(ev(pt(130, 100, 0), pt(190, 100, 1)))
    expect(kinds(r.intents)).toEqual(['dragStart', 'dragMove', 'dragEnd'])
  })
})
