/**
 * Phase 14-11 (MOB-01) — two-finger pinch changes the terminal font size.
 *
 * A pinch maps finger distance to a whole-pixel font size (size0 x d / d0, clamped to 6..32),
 * and never zooms the page: every two-touch touchmove is preventDefault'ed on a non-passive
 * listener, and iOS Safari's gesturestart is prevented as a belt. One-finger events are left
 * to attachTouchScroll (which releases multi-touch to the browser, so the two do not fight).
 *
 * setSize runs on every change during the pinch (the caller lays out and reports only);
 * onEnd runs once, 250 ms after the last finger lifts, so the caller makes ONE grid claim at
 * the final size (INPT-06: a claim reflows the shared grid, so a pinch must not send a stream
 * of them). A new pinch inside that window restarts it.
 */

export const MIN_FONT_PX = 6
export const MAX_FONT_PX = 32
// Long enough that a quick re-pinch is one gesture, short enough to feel immediate.
const END_DELAY_MS = 250

export interface PinchHandlers {
  getSize(): number
  setSize(px: number): void
  onEnd(): void
}

function distance(t: TouchList): number {
  return Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY)
}

export function attachPinchZoom(container: HTMLElement, h: PinchHandlers): () => void {
  let d0 = 0 // 0 = no pinch in progress (a second finger has not moved yet)
  let size0 = 0
  let last = 0
  let pinched = false // a pinch happened since the last onEnd
  let timer: ReturnType<typeof setTimeout> | null = null

  const cancelTimer = (): void => {
    if (timer !== null) clearTimeout(timer)
    timer = null
  }

  const onTouchStart = (e: TouchEvent): void => {
    if (e.touches.length >= 2) cancelTimer()
  }

  const onTouchMove = (e: TouchEvent): void => {
    if (e.touches.length !== 2) return
    e.preventDefault()
    pinched = true
    cancelTimer()
    const d = distance(e.touches)
    if (d0 === 0) {
      // Two fingers on the same point give 0; wait for them to spread rather than divide by it.
      if (d === 0) return
      d0 = d
      size0 = h.getSize()
      last = size0
      return
    }
    const next = Math.min(MAX_FONT_PX, Math.max(MIN_FONT_PX, Math.round((size0 * d) / d0)))
    if (next !== last) {
      last = next
      h.setSize(next)
    }
  }

  const onTouchEnd = (e: TouchEvent): void => {
    if (e.touches.length < 2) d0 = 0 // the next two-finger move starts a fresh pinch from the current size
    if (e.touches.length === 0 && pinched) {
      cancelTimer()
      timer = setTimeout(() => {
        timer = null
        pinched = false
        h.onEnd()
      }, END_DELAY_MS)
    }
  }

  const onGestureStart = (e: Event): void => e.preventDefault()

  container.addEventListener('touchstart', onTouchStart, { passive: true })
  // Non-passive: a passive listener's preventDefault is ignored and the page would zoom.
  container.addEventListener('touchmove', onTouchMove, { passive: false })
  container.addEventListener('touchend', onTouchEnd, { passive: true })
  container.addEventListener('touchcancel', onTouchEnd, { passive: true })
  document.addEventListener('gesturestart', onGestureStart)

  return () => {
    cancelTimer()
    container.removeEventListener('touchstart', onTouchStart)
    container.removeEventListener('touchmove', onTouchMove)
    container.removeEventListener('touchend', onTouchEnd)
    container.removeEventListener('touchcancel', onTouchEnd)
    document.removeEventListener('gesturestart', onGestureStart)
  }
}
