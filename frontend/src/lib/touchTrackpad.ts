/**
 * Phase 21-03 (DVWR-03) — the iPhone trackpad gesture recognizer.
 *
 * createTrackpad is a pure state machine: touches in, TrackpadIntent out, every timer and
 * timestamp taken from an injectable clock so tests run on a fake one. attachTrackpad wires
 * it to a container's touch events (non-passive, container only, never window or document).
 *
 * Intents carry finger pixels (move and drag deltas are already multiplied by accelGain);
 * desktopCoords.normalizedDelta turns them into picture fractions. A pinch emits only local
 * zoom intents, never anything meant for the Mac. Nothing here reads the user agent.
 */

export type TrackpadIntent =
  | { kind: 'move'; dx: number; dy: number }
  | { kind: 'tap'; at: number }
  | { kind: 'doubleTap'; at: [number, number] }
  | { kind: 'longPress'; at: number }
  | { kind: 'dragStart'; at: number }
  | { kind: 'dragMove'; dx: number; dy: number }
  | { kind: 'dragEnd'; at: number }
  | { kind: 'scroll'; dx: number; dy: number }
  | { kind: 'pinch'; scale: number; cx: number; cy: number }
  | { kind: 'pinchEnd' }
  | { kind: 'resetZoom' }

export interface TrackpadClock {
  now(): number
  setTimeout(fn: () => void, ms: number): unknown
  clearTimeout(id: unknown): void
}

export interface TouchPoint {
  identifier: number
  clientX: number
  clientY: number
}

export interface TouchLike {
  touches: TouchPoint[]
  changedTouches: TouchPoint[]
}

// UI-SPEC Gestures defaults; the device rows may tune them.
export const TRACKPAD = { tapMovePx: 8, tapMs: 250, doubleTapMs: 300, doubleTapPx: 24, longPressMs: 500, decidePx: 12 }

// UI-SPEC: gain 1.0 below 200 px/s, linear up to 2.5 at 1500 px/s, flat above.
const GAIN_LOW_SPEED = 200
const GAIN_HIGH_SPEED = 1500
const GAIN_MAX = 2.5

export function accelGain(pxPerSecond: number): number {
  if (pxPerSecond <= GAIN_LOW_SPEED) return 1
  if (pxPerSecond >= GAIN_HIGH_SPEED) return GAIN_MAX
  return 1 + ((GAIN_MAX - 1) * (pxPerSecond - GAIN_LOW_SPEED)) / (GAIN_HIGH_SPEED - GAIN_LOW_SPEED)
}

const realClock: TrackpadClock = {
  now: () => performance.now(),
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (id) => clearTimeout(id as ReturnType<typeof setTimeout>),
}

export function createTrackpad(
  onIntent: (i: TrackpadIntent) => void,
  clock: TrackpadClock = realClock,
): { start(e: TouchLike): void; move(e: TouchLike): void; end(e: TouchLike): void; cancel(): void } {
  // One-finger state: where the touch began, and the last point a delta was taken from.
  let down = false
  let sx = 0
  let sy = 0
  let lx = 0
  let ly = 0
  let lt = 0
  let moving = false // moved past tapMovePx: this touch is a pointer move, not a tap

  const reset = (): void => {
    down = false
    moving = false
  }

  // Delta since the last reference point, scaled by the speed measured over that span.
  const gainedDelta = (x: number, y: number): { dx: number; dy: number } => {
    const now = clock.now()
    const dx = x - lx
    const dy = y - ly
    const dt = Math.max(now - lt, 1) // a zero timestamp gap must not read as infinite speed
    const gain = accelGain((Math.hypot(dx, dy) / dt) * 1000)
    lx = x
    ly = y
    lt = now
    return { dx: dx * gain, dy: dy * gain }
  }

  return {
    start(e) {
      if (e.touches.length !== 1) return
      const t = e.touches[0]
      down = true
      moving = false
      sx = lx = t.clientX
      sy = ly = t.clientY
      lt = clock.now()
    },
    move(e) {
      if (!down || e.touches.length !== 1) return
      const t = e.touches[0]
      if (!moving) {
        if (Math.hypot(t.clientX - sx, t.clientY - sy) <= TRACKPAD.tapMovePx) return
        moving = true
      }
      const d = gainedDelta(t.clientX, t.clientY)
      if (d.dx !== 0 || d.dy !== 0) onIntent({ kind: 'move', ...d })
    },
    end() {
      reset()
    },
    cancel() {
      reset()
    },
  }
}

export function attachTrackpad(
  container: HTMLElement,
  onIntent: (i: TrackpadIntent) => void,
  clock?: TrackpadClock,
): () => void {
  const pad = createTrackpad(onIntent, clock)
  const like = (e: TouchEvent): TouchLike => ({
    touches: Array.from(e.touches),
    changedTouches: Array.from(e.changedTouches),
  })

  // preventDefault on the stage's touches so a long press shows no iOS callout or selection.
  const onStart = (e: TouchEvent): void => {
    e.preventDefault()
    pad.start(like(e))
  }
  const onMove = (e: TouchEvent): void => {
    e.preventDefault()
    pad.move(like(e))
  }
  const onEnd = (e: TouchEvent): void => pad.end(like(e))
  const onCancel = (): void => pad.cancel()
  const onGestureStart = (e: Event): void => e.preventDefault()

  // Non-passive: a passive listener's preventDefault is ignored and Safari would zoom the page.
  container.addEventListener('touchstart', onStart, { passive: false })
  container.addEventListener('touchmove', onMove, { passive: false })
  container.addEventListener('touchend', onEnd, { passive: false })
  container.addEventListener('touchcancel', onCancel, { passive: false })
  container.addEventListener('gesturestart', onGestureStart)

  return () => {
    pad.cancel()
    container.removeEventListener('touchstart', onStart)
    container.removeEventListener('touchmove', onMove)
    container.removeEventListener('touchend', onEnd)
    container.removeEventListener('touchcancel', onCancel)
    container.removeEventListener('gesturestart', onGestureStart)
  }
}
