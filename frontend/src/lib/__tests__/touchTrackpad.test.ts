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
