import { describe, it, expect, vi } from 'vitest'
import { compensatePointerScale, computeGuestScale, computeGuestViewport, DEFAULT_GUEST_MIN_SCALE, unscalePointer } from './terminalScale'

describe('computeGuestScale — VIEW-05 scale math', () => {
  // Container fits the grid (would-be scale > 1.0): cap to 1.0
  it('returns 1.0 when container is larger than grid (downscale-only cap)', () => {
    // container 800×600, grid 400×300 → s = min(2.0, 2.0) = 2.0 → capped to 1.0
    expect(computeGuestScale(800, 600, 400, 300)).toBe(1)
  })

  // Container is exactly half the grid size in both dimensions
  it('returns 0.5 when container is half the grid in both axes', () => {
    // container 400×300, grid 800×600 → s = min(0.5, 0.5) = 0.5
    expect(computeGuestScale(400, 300, 800, 600)).toBe(0.5)
  })

  // Width-bound: container height is more generous than width
  it('returns the width-bound scale when width is the tighter constraint', () => {
    // container 400×600, grid 800×300 → s = min(0.5, 2.0) = 0.5
    expect(computeGuestScale(400, 600, 800, 300)).toBe(0.5)
  })

  // Height-bound: container width is more generous than height
  it('returns the height-bound scale when height is the tighter constraint', () => {
    // container 600×400, grid 300×800 → s = min(2.0, 0.5) = 0.5
    expect(computeGuestScale(600, 400, 300, 800)).toBe(0.5)
  })

  // Zero gridW guard (avoid divide-by-zero)
  it('returns 1 when gridW is 0 (safe no-op)', () => {
    expect(computeGuestScale(800, 600, 0, 300)).toBe(1)
  })

  // Zero gridH guard
  it('returns 1 when gridH is 0 (safe no-op)', () => {
    expect(computeGuestScale(800, 600, 400, 0)).toBe(1)
  })

  // Both grid dims zero
  it('returns 1 when both grid dims are 0', () => {
    expect(computeGuestScale(800, 600, 0, 0)).toBe(1)
  })

  // Negative gridW guard (defensive: negative is treated as ≤ 0)
  it('returns 1 when gridW is negative', () => {
    expect(computeGuestScale(800, 600, -1, 300)).toBe(1)
  })

  // Exact fit (scale = 1.0 exactly)
  it('returns exactly 1.0 when container equals grid size', () => {
    expect(computeGuestScale(800, 600, 800, 600)).toBe(1)
  })

  // Non-integer scale
  it('returns a non-integer scale factor for non-even ratios', () => {
    // container 300×200, grid 400×300 → min(0.75, 0.666…) ≈ 0.666…
    const s = computeGuestScale(300, 200, 400, 300)
    expect(s).toBeCloseTo(2 / 3, 5)
    expect(s).toBeLessThanOrEqual(1)
  })
})

describe('computeGuestViewport — BUG-01 floor-aware guest scale', () => {
  // Above-floor: natural (capped) scale is at or above the floor — behavior
  // matches computeGuestScale exactly, no horizontal scroll.
  it('returns the natural capped scale with overflowX=false when above the floor', () => {
    // container 400×300, grid 800×600 → natural = 0.5; floor 0.3 → 0.5 >= 0.3
    expect(computeGuestViewport(400, 300, 800, 600, 0.3)).toEqual({ scale: 0.5, overflowX: false })
  })

  // Below-floor: natural scale would be unreadably small — clamp to the floor
  // and signal the caller to enable horizontal scroll instead of shrinking further.
  it('clamps to minScale and sets overflowX=true when natural scale is below the floor', () => {
    // container 200×150, grid 800×600 → natural = 0.25; floor 0.5 → 0.25 < 0.5
    expect(computeGuestViewport(200, 150, 800, 600, 0.5)).toEqual({ scale: 0.5, overflowX: true })
  })

  // Upscale cap: container ≥ grid — natural > 1 is capped to 1.0, never upscale
  // (VIEW-04/05 invariant), and since 1.0 is always ≥ any reasonable floor,
  // overflowX stays false.
  it('caps at 1.0 (never upscale) with overflowX=false when container exceeds grid', () => {
    // container 800×600, grid 400×300 → natural = 2.0 → capped to 1.0; floor 0.5 → 1.0 >= 0.5
    expect(computeGuestViewport(800, 600, 400, 300, 0.5)).toEqual({ scale: 1, overflowX: false })
  })

  // Exact fit at the floor boundary: naturalCapped === minScale is NOT below
  // the floor — stays on the non-overflow branch.
  it('treats an exact floor match as above-floor (overflowX=false)', () => {
    // container 300×300, grid 600×600 → natural = 0.5; floor 0.5 → 0.5 >= 0.5
    expect(computeGuestViewport(300, 300, 600, 600, 0.5)).toEqual({ scale: 0.5, overflowX: false })
  })

  // Zero/negative grid-dim guard mirrors computeGuestScale's divide-by-zero safety.
  it('returns {scale:1, overflowX:false} when gridW is 0', () => {
    expect(computeGuestViewport(800, 600, 0, 300, 0.5)).toEqual({ scale: 1, overflowX: false })
  })

  it('returns {scale:1, overflowX:false} when gridH is 0', () => {
    expect(computeGuestViewport(800, 600, 400, 0, 0.5)).toEqual({ scale: 1, overflowX: false })
  })

  it('returns {scale:1, overflowX:false} when gridW is negative', () => {
    expect(computeGuestViewport(800, 600, -1, 300, 0.5)).toEqual({ scale: 1, overflowX: false })
  })

  // Default floor constant: callers/tests share one source of truth.
  it('DEFAULT_GUEST_MIN_SCALE is a readability floor between 0 and 1', () => {
    expect(DEFAULT_GUEST_MIN_SCALE).toBeGreaterThan(0)
    expect(DEFAULT_GUEST_MIN_SCALE).toBeLessThan(1)
  })

  it('uses DEFAULT_GUEST_MIN_SCALE when minScale is omitted', () => {
    // container tiny enough that natural scale is always below DEFAULT_GUEST_MIN_SCALE
    const result = computeGuestViewport(10, 10, 800, 600)
    expect(result).toEqual({ scale: DEFAULT_GUEST_MIN_SCALE, overflowX: true })
  })
})

describe('unscalePointer — selection-offset-copy pointer mapping under scale()', () => {
  // Scaled box: layout width 1000, drawn at 0.8 from (100, 50).
  const rect = { left: 100, top: 50, width: 800 }

  it('maps a pointer into the unscaled box (rows below the top move down by 1/s)', () => {
    expect(unscalePointer({ clientX: 500, clientY: 450 }, rect, 1000)).toEqual({ clientX: 600, clientY: 550 })
  })

  it('leaves the origin fixed (top-left stays put)', () => {
    expect(unscalePointer({ clientX: 100, clientY: 50 }, rect, 1000)).toEqual({ clientX: 100, clientY: 50 })
  })

  it('is the identity at scale 1', () => {
    expect(unscalePointer({ clientX: 333, clientY: 777 }, { left: 100, top: 50, width: 1000 }, 1000)).toEqual({ clientX: 333, clientY: 777 })
  })

  it('maps a pointer one cell inside the scaled box to one unscaled cell (row boundary neighbors)', () => {
    // cell 20px tall unscaled, 16px drawn at 0.8: drawn rows 47.9 / 48.1 straddle the 48th boundary.
    const cellH = 20
    const drawn = 16
    const rowOf = (y: number) => Math.ceil((unscalePointer({ clientX: 100, clientY: 50 + y }, rect, 1000).clientY - 50) / cellH)
    expect(rowOf(drawn * 47 + 0.1)).toBe(48)
    expect(rowOf(drawn * 48 - 0.1)).toBe(48)
    expect(rowOf(drawn * 48 + 0.1)).toBe(49)
  })

  it('passes through an unmeasurable box (hidden pane: offsetWidth 0 or rect width 0)', () => {
    expect(unscalePointer({ clientX: 5, clientY: 6 }, rect, 0)).toEqual({ clientX: 5, clientY: 6 })
    expect(unscalePointer({ clientX: 5, clientY: 6 }, { left: 0, top: 0, width: 0 }, 1000)).toEqual({ clientX: 5, clientY: 6 })
  })

  it('reads MouseEvent getters (a spread of a MouseEvent would be {})', () => {
    const ev = new MouseEvent('mousedown', { clientX: 500, clientY: 450 })
    expect(unscalePointer(ev, rect, 1000)).toEqual({ clientX: 600, clientY: 550 })
  })
})

describe('compensatePointerScale — wraps xterm\'s MouseService', () => {
  function scaledElement(): HTMLElement {
    const el = document.createElement('div')
    Object.defineProperty(el, 'offsetWidth', { value: 1000 })
    el.getBoundingClientRect = () => ({ left: 100, top: 50, width: 800, height: 400, right: 900, bottom: 450, x: 100, y: 50, toJSON: () => ({}) })
    return el
  }

  it('hands both methods the unscaled pointer, keeps the other args and this', () => {
    const seen: unknown[] = []
    const ms = {
      tag: 'ms',
      getCoords(this: { tag: string }, ev: { clientX: number; clientY: number }, _el: HTMLElement, cols: number, rows: number, isSel?: boolean) {
        seen.push([this.tag, ev.clientX, ev.clientY, cols, rows, isSel])
        return [1, 1]
      },
      getMouseReportCoords(this: { tag: string }, ev: { clientX: number; clientY: number }, _el: HTMLElement) {
        seen.push([this.tag, ev.clientX, ev.clientY])
        return { col: 0, row: 0, x: 0, y: 0 }
      },
    }
    const el = scaledElement()
    compensatePointerScale({ element: el, _core: { _mouseService: ms } } as unknown as { element: HTMLElement })
    expect(ms.getCoords(new MouseEvent('mousemove', { clientX: 500, clientY: 450 }), el, 80, 24, true)).toEqual([1, 1])
    ms.getMouseReportCoords(new MouseEvent('mousedown', { clientX: 500, clientY: 450 }), el)
    expect(seen).toEqual([['ms', 600, 550, 80, 24, true], ['ms', 600, 550]])
  })

  it('warns (once) instead of failing silently when the MouseService is missing', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const el = document.createElement('div')
    compensatePointerScale({ element: el, _core: {} } as unknown as { element: HTMLElement })
    compensatePointerScale({ element: el } as unknown as { element: HTMLElement })
    expect(warn).toHaveBeenCalledTimes(1)
    warn.mockRestore()
  })
})
