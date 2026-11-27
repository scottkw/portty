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
 *
 * A single tap is emitted only after the 300 ms double-tap window passes with no second touch.
 * That delay is deliberate: without it, "tap then drag" would reach the Mac as a click followed
 * by a press inside the double-click interval, which macOS reads as a double-click drag (word
 * selection). Cost: 300 ms on every single tap, the same trade other trackpad viewers make.
 * iOS fires no contextmenu on a long press, so the long press (right click) is a timer.
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
  // idle: nothing down. touching: one finger, still tap-sized. second: a finger that landed within
  // the double-tap window of a tap (tap, double-tap or drag, not decided yet). moving: a pointer
  // move. dragging: tap-then-drag. longPressed: the long press fired. tapPending: finger up, the
  // tap is waiting out the double-tap window. two: a two-finger gesture (scroll, pinch or tap).
  // ignoring: a cancelled or finished gesture whose fingers are still down; nothing until all lift.
  type Mode = 'idle' | 'touching' | 'second' | 'moving' | 'dragging' | 'longPressed' | 'tapPending' | 'two' | 'ignoring'
  let mode: Mode = 'idle'

  // Two-finger state. The gesture is chosen once, on the first move past decidePx: pinch if the
  // finger distance changed more than the pair's centroid travelled, else scroll.
  let d0 = 0 // finger distance at the start (0 until the fingers separate)
  let c0x = 0 // centroid at the start
  let c0y = 0
  let lcx = 0 // centroid at the last scroll delta
  let lcy = 0
  let twoT0 = 0
  let decided: 'pinch' | 'scroll' | null = null
  let maxDist = 0 // largest distance change and centroid travel seen, for the two-finger tap test
  let maxTravel = 0
  let twoTapChained = false // this pair landed within the double-tap window of a two-finger tap
  let lastTwoTapAt: number | null = null
  let sx = 0 // where this touch began
  let sy = 0
  let t0 = 0 // when it began
  let lx = 0 // the last point a delta was taken from, and when
  let ly = 0
  let lt = 0
  let prev: { at: number; x: number; y: number } | null = null // the lifted tap waiting to be emitted
  let tapTimer: unknown = null
  let pressTimer: unknown = null

  const clearTapTimer = (): void => {
    if (tapTimer !== null) clock.clearTimeout(tapTimer)
    tapTimer = null
  }
  const clearPressTimer = (): void => {
    if (pressTimer !== null) clock.clearTimeout(pressTimer)
    pressTimer = null
  }
  const flushTap = (): void => {
    clearTapTimer()
    if (!prev) return
    const at = prev.at
    prev = null
    onIntent({ kind: 'tap', at })
  }
  const armTap = (): void => {
    prev = { at: clock.now(), x: sx, y: sy }
    mode = 'tapPending'
    tapTimer = clock.setTimeout(() => {
      tapTimer = null
      flushTap()
      if (mode === 'tapPending') mode = 'idle'
    }, TRACKPAD.doubleTapMs)
  }
  // However a gesture is torn down, a held button must be released on the Mac and a pinch closed
  // so the caller can drop will-change.
  const endGestureIfAny = (): void => {
    if (mode === 'dragging') onIntent({ kind: 'dragEnd', at: clock.now() })
    else if (mode === 'two' && decided === 'pinch') onIntent({ kind: 'pinchEnd' })
  }

  const dist = (a: TouchPoint, b: TouchPoint): number => Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY)

  const beginTwo = (a: TouchPoint, b: TouchPoint): void => {
    clearPressTimer()
    endGestureIfAny()
    flushTap() // a tap still waiting out its window is a real tap
    const now = clock.now()
    twoTapChained = lastTwoTapAt !== null && now - lastTwoTapAt <= TRACKPAD.doubleTapMs
    lastTwoTapAt = null
    d0 = dist(a, b)
    c0x = lcx = (a.clientX + b.clientX) / 2
    c0y = lcy = (a.clientY + b.clientY) / 2
    twoT0 = now
    decided = null
    maxDist = maxTravel = 0
    mode = 'two'
  }

  const moveTwo = (a: TouchPoint, b: TouchPoint): void => {
    const d = dist(a, b)
    if (d0 === 0) {
      // Two fingers on one point have no distance to compare against; wait for them to separate.
      if (d > 0) d0 = d
      return
    }
    const cx = (a.clientX + b.clientX) / 2
    const cy = (a.clientY + b.clientY) / 2
    const dDist = Math.abs(d - d0)
    const travel = Math.hypot(cx - c0x, cy - c0y)
    maxDist = Math.max(maxDist, dDist)
    maxTravel = Math.max(maxTravel, travel)
    if (decided === null) {
      if (dDist <= TRACKPAD.decidePx && travel <= TRACKPAD.decidePx) return
      decided = dDist > travel ? 'pinch' : 'scroll'
    }
    if (decided === 'pinch') {
      onIntent({ kind: 'pinch', scale: d / d0, cx, cy })
      return
    }
    const dx = cx - lcx
    const dy = cy - lcy
    lcx = cx
    lcy = cy
    if (dx !== 0 || dy !== 0) onIntent({ kind: 'scroll', dx, dy })
  }

  // The first finger to lift ends a two-finger gesture; the other is ignored until it lifts too.
  const endTwo = (remaining: number): void => {
    const now = clock.now()
    if (decided === 'pinch') {
      onIntent({ kind: 'pinchEnd' })
    } else if (decided === null && maxDist <= TRACKPAD.tapMovePx && maxTravel <= TRACKPAD.tapMovePx && now - twoT0 <= TRACKPAD.tapMs) {
      if (twoTapChained) onIntent({ kind: 'resetZoom' })
      else lastTwoTapAt = now
    }
    mode = remaining === 0 ? 'idle' : 'ignoring'
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
      const n = e.touches.length
      if (n === 2) {
        beginTwo(e.touches[0], e.touches[1])
        return
      }
      if (n >= 3) {
        // A third finger cancels whatever was going on, with no intent of its own.
        clearPressTimer()
        endGestureIfAny()
        flushTap()
        mode = 'ignoring'
        return
      }
      if (n !== 1) return
      const t = e.touches[0]
      clearPressTimer()
      endGestureIfAny() // a start with no preceding end: the lift was lost
      const now = clock.now()
      if (mode === 'tapPending' && prev && now - prev.at <= TRACKPAD.doubleTapMs) {
        clearTapTimer()
        mode = 'second'
      } else {
        flushTap() // overdue window (no-op when nothing is pending)
        mode = 'touching'
      }
      sx = lx = t.clientX
      sy = ly = t.clientY
      t0 = lt = now
      pressTimer = clock.setTimeout(() => {
        pressTimer = null
        if (mode === 'second') {
          // Tap, then touch and hold: the button goes down now, the drag follows the finger (UI-SPEC: button
          // down at the second touch). M-195: a finger resting on a title bar before it moves right-clicked.
          prev = null
          mode = 'dragging'
          onIntent({ kind: 'dragStart', at: clock.now() })
          return
        }
        if (mode !== 'touching') return
        mode = 'longPressed'
        onIntent({ kind: 'longPress', at: clock.now() })
      }, TRACKPAD.longPressMs)
    },
    move(e) {
      if (mode === 'two' && e.touches.length === 2) {
        moveTwo(e.touches[0], e.touches[1])
        return
      }
      if (e.touches.length !== 1) return
      const t = e.touches[0]
      if (mode === 'touching' || mode === 'second') {
        if (Math.hypot(t.clientX - sx, t.clientY - sy) <= TRACKPAD.tapMovePx) return
        clearPressTimer()
        if (mode === 'second') {
          prev = null // tap then drag: the first tap is consumed, no click before the drag
          mode = 'dragging'
          onIntent({ kind: 'dragStart', at: clock.now() })
        } else {
          mode = 'moving'
        }
      } else if (mode !== 'moving' && mode !== 'dragging') {
        return
      }
      const d = gainedDelta(t.clientX, t.clientY)
      if (d.dx !== 0 || d.dy !== 0) onIntent({ kind: mode === 'dragging' ? 'dragMove' : 'move', ...d })
    },
    end(e) {
      clearPressTimer()
      if (mode === 'two') {
        endTwo(e.touches.length)
        return
      }
      if (mode === 'ignoring') {
        if (e.touches.length === 0) mode = 'idle'
        return
      }
      const now = clock.now()
      const tapped = now - t0 <= TRACKPAD.tapMs
      switch (mode) {
        case 'touching':
          if (tapped) armTap()
          else mode = 'idle'
          break
        case 'second':
          if (tapped && prev && Math.hypot(sx - prev.x, sy - prev.y) <= TRACKPAD.doubleTapPx) {
            onIntent({ kind: 'doubleTap', at: [prev.at, now] })
            prev = null
            mode = 'idle'
          } else if (tapped) {
            flushTap() // a tap too far from the first: two taps
            armTap()
          } else {
            flushTap()
            mode = 'idle'
          }
          break
        case 'dragging':
          endGestureIfAny()
          mode = 'idle'
          break
        case 'moving':
        case 'longPressed':
          mode = 'idle'
          break
      }
    },
    cancel() {
      clearPressTimer()
      clearTapTimer()
      prev = null
      lastTwoTapAt = null
      endGestureIfAny() // pending states emit nothing; only a held button or open pinch needs closing
      mode = 'idle'
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
