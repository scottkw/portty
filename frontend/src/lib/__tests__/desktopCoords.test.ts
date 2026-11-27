import { describe, it, expect } from 'vitest'
import {
  FIT,
  MAX_ZOOM,
  MIN_ZOOM,
  PAN_MARGIN_PX,
  SCROLL_SIGN,
  clampZoom,
  normalizedDelta,
  panToKeepPointer,
  pointerToStage,
  scrollDelta,
  videoTransform,
  zoomAround,
  type ZoomState,
} from '../desktopCoords'

const W = 400
const H = 300
// Where a stage point at 1x lands on screen under z.
const screenOf = (z: ZoomState, x: number, y: number) => ({ x: z.scale * x + z.tx, y: z.scale * y + z.ty })
// The picture must always cover the stage: no edge may move inside it.
function covers(z: ZoomState): boolean {
  return z.tx <= 0 && z.ty <= 0 && z.scale * W + z.tx >= W - 1e-9 && z.scale * H + z.ty >= H - 1e-9
}

describe('normalizedDelta', () => {
  it('divides finger pixels by zoom times the 1x picture size', () => {
    expect(normalizedDelta(40, 0, 1, 800, 500)).toEqual({ ndx: 0.05, ndy: 0 })
    expect(normalizedDelta(40, 0, 2, 800, 500)).toEqual({ ndx: 0.025, ndy: 0 })
    expect(normalizedDelta(0, 50, 1, 800, 500)).toEqual({ ndx: 0, ndy: 0.1 })
  })

  it('an unknown picture size (zero rect) yields no movement, never Infinity', () => {
    expect(normalizedDelta(40, 40, 1, 0, 0)).toEqual({ ndx: 0, ndy: 0 })
  })
})

describe('zoom limits', () => {
  it('clampZoom holds 1x through 5x', () => {
    expect(MIN_ZOOM).toBe(1)
    expect(MAX_ZOOM).toBe(5)
    expect(clampZoom(0.5)).toBe(1)
    expect(clampZoom(9)).toBe(5)
    expect(clampZoom(2.5)).toBe(2.5)
    expect(FIT).toEqual({ scale: 1, tx: 0, ty: 0 })
  })
})

describe('zoomAround', () => {
  it('keeps the pinch centre fixed on screen', () => {
    const z = zoomAround(FIT, 2, 100, 50, W, H)
    expect(z.scale).toBe(2)
    expect(screenOf(z, 100, 50)).toEqual({ x: 100, y: 50 })
    expect(covers(z)).toBe(true)

    // and again from an already zoomed, panned state
    const z2 = zoomAround(z, 3, 250, 120, W, H)
    const p = { x: (250 - z.tx) / z.scale, y: (120 - z.ty) / z.scale } // the picture point under the centre
    expect(screenOf(z2, p.x, p.y).x).toBeCloseTo(250, 9)
    expect(screenOf(z2, p.x, p.y).y).toBeCloseTo(120, 9)
    expect(covers(z2)).toBe(true)
  })

  it('clamps to 5x, and to 1x with no offset', () => {
    const big = zoomAround(FIT, 9, W, H, W, H) // pinch in the far corner
    expect(big.scale).toBe(5)
    expect(covers(big)).toBe(true)
    expect(zoomAround(big, 0.5, 10, 10, W, H)).toEqual({ scale: 1, tx: 0, ty: 0 })
  })

  it('never lets the picture edge move inside the stage edge', () => {
    for (const [cx, cy] of [[0, 0], [W, H], [W, 0], [0, H], [200, 150]]) {
      for (const s of [1.5, 2, 5]) expect(covers(zoomAround(FIT, s, cx, cy, W, H))).toBe(true)
    }
  })
})

describe('panToKeepPointer', () => {
  it('shifts the view so a pointer 10 px from the right edge sits 32 px inside', () => {
    const z: ZoomState = { scale: 3, tx: -500, ty: -100 }
    const px = (W - 10 - z.tx) / z.scale // the pointer at 1x, 10 px from the right edge on screen
    const out = panToKeepPointer(z, px, 100, W, H)
    expect(PAN_MARGIN_PX).toBe(32)
    expect(screenOf(out, px, 100).x).toBeCloseTo(W - 32, 9)
    expect(out.ty).toBe(z.ty)
    expect(covers(out)).toBe(true)
  })

  it('shifts the other way at the left and top edges, within the clamp', () => {
    const z: ZoomState = { scale: 3, tx: -500, ty: -400 }
    const px = (5 - z.tx) / z.scale
    const py = (5 - z.ty) / z.scale
    const out = panToKeepPointer(z, px, py, W, H)
    expect(screenOf(out, px, py).x).toBeCloseTo(32, 9)
    expect(screenOf(out, px, py).y).toBeCloseTo(32, 9)
    expect(covers(out)).toBe(true)
  })

  it('leaves the view alone when the pointer is comfortably inside, and at 1x', () => {
    const z: ZoomState = { scale: 3, tx: -500, ty: -100 }
    expect(panToKeepPointer(z, 250, 100, W, H)).toBe(z)
    expect(panToKeepPointer(FIT, 5, 5, W, H)).toBe(FIT)
  })
})

describe('videoTransform', () => {
  it('is empty at 1x so the video carries no transform', () => {
    expect(videoTransform(FIT)).toBe('')
  })

  it('is a translate then scale string while zoomed', () => {
    expect(videoTransform({ scale: 2, tx: -100, ty: -50 })).toBe('translate(-100px, -50px) scale(2)')
  })
})

describe('scrollDelta', () => {
  it('content moves with the fingers (natural scrolling), divided by the zoom', () => {
    expect(SCROLL_SIGN).toBe(-1)
    expect(scrollDelta(0, -30, 1).dy).toBeCloseTo(30, 9)
    expect(scrollDelta(0, -30, 2).dy).toBeCloseTo(15, 9)
    expect(scrollDelta(20, 0, 2).dx).toBeCloseTo(-10, 9)
  })

  it('the sign lives only in SCROLL_SIGN', () => {
    expect(scrollDelta(10, -30, 1).dx).toBeCloseTo(SCROLL_SIGN * 10, 9)
    expect(scrollDelta(10, -30, 1).dy).toBeCloseTo(SCROLL_SIGN * -30, 9)
  })
})

describe('pointerToStage', () => {
  it('places a normalized point inside the letterbox rect', () => {
    expect(pointerToStage(0.5, 0.5, { x: 10, y: 20, width: 200, height: 100 })).toEqual({ x: 110, y: 70 })
  })
})
