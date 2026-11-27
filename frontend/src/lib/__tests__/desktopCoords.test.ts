import { describe, it, expect } from 'vitest'
import { FIT, MAX_ZOOM, MIN_ZOOM, clampZoom, normalizedDelta, pointerToStage } from '../desktopCoords'

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

describe('pointerToStage', () => {
  it('places a normalized point inside the letterbox rect', () => {
    expect(pointerToStage(0.5, 0.5, { x: 10, y: 20, width: 200, height: 100 })).toEqual({ x: 110, y: 70 })
  })
})
