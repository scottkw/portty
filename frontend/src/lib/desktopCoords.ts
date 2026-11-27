/**
 * Phase 21-03 (DVWR-03) — pixel-to-picture math for the iPhone trackpad mode.
 *
 * Pure functions: no React, no timers, no network. The gesture engine (touchTrackpad.ts)
 * produces finger-pixel intents; this file turns them into normalized pointer deltas and
 * into the local zoom transform. Zoom is a CSS transform on the <video> and never reaches
 * the Mac.
 */

// Type-only on purpose: the letterbox math stays the one videoContentRect in DesktopView, and this
// module keeps no runtime dependency on React.
import type { VideoRect } from '../components/DesktopView'

export interface ZoomState {
  scale: number
  tx: number
  ty: number
}

export const MIN_ZOOM = 1
export const MAX_ZOOM = 5

/** The unzoomed view. Treat as immutable: every function here returns a new state. */
export const FIT: ZoomState = { scale: MIN_ZOOM, tx: 0, ty: 0 }

export function clampZoom(s: number): number {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, s))
}

/**
 * Finger pixels to a fraction of the picture at 1x. Dividing by the zoom is what makes
 * precision rise as the user zooms in. pictureWidth1x/Height1x come from videoContentRect;
 * that rect is all zeros until the video size is known, and a zero size must not produce
 * an Infinity that goes on the wire, so it yields no movement.
 */
export function normalizedDelta(
  dxPx: number,
  dyPx: number,
  zoom: number,
  pictureWidth1x: number,
  pictureHeight1x: number,
): { ndx: number; ndy: number } {
  if (zoom <= 0 || pictureWidth1x <= 0 || pictureHeight1x <= 0) return { ndx: 0, ndy: 0 }
  return { ndx: dxPx / (zoom * pictureWidth1x), ndy: dyPx / (zoom * pictureHeight1x) }
}

export const PAN_MARGIN_PX = 32

// Natural scrolling: the content moves with the fingers, so fingers moving up (negative dy)
// is a positive wheel delta. This is the only place the sign appears; the device row (research
// A10) decides whether to flip it.
export const SCROLL_SIGN = -1

/** Finger pixels to wheel pixels for the Mac, divided by the zoom (the picture is magnified). */
export function scrollDelta(dxPx: number, dyPx: number, zoom: number): { dx: number; dy: number } {
  return { dx: (SCROLL_SIGN * dxPx) / zoom, dy: (SCROLL_SIGN * dyPx) / zoom }
}

// The zoomed element must keep covering the stage, so each offset lies between stage-minus-scaled
// size and zero. At 1x that range is a single point, so the offset is exactly zero.
function clampOffset(t: number, scale: number, stage: number): number {
  return Math.min(0, Math.max(stage - scale * stage, t))
}

/**
 * Zoom to an absolute scale (clamped 1x to 5x) keeping the stage point (cx, cy) fixed on screen.
 * (cx, cy) is in stage pixels, relative to the stage's top-left corner.
 */
export function zoomAround(z: ZoomState, scale: number, cx: number, cy: number, stageW: number, stageH: number): ZoomState {
  const s = clampZoom(scale)
  const k = s / z.scale
  return {
    scale: s,
    tx: clampOffset(cx - k * (cx - z.tx), s, stageW),
    ty: clampOffset(cy - k * (cy - z.ty), s, stageH),
  }
}

function keepInside(t: number, scale: number, p: number, stage: number): number {
  const screen = scale * p + t
  if (screen > stage - PAN_MARGIN_PX) return clampOffset(stage - PAN_MARGIN_PX - scale * p, scale, stage)
  if (screen < PAN_MARGIN_PX) return clampOffset(PAN_MARGIN_PX - scale * p, scale, stage)
  return t
}

/**
 * While zoomed, move the view so the pointer (px, py: stage pixels at 1x) stays at least
 * PAN_MARGIN_PX inside the stage edge. A no-op at 1x, and when the pointer is already inside.
 */
export function panToKeepPointer(z: ZoomState, px: number, py: number, stageW: number, stageH: number): ZoomState {
  if (z.scale <= MIN_ZOOM) return z
  const tx = keepInside(z.tx, z.scale, px, stageW)
  const ty = keepInside(z.ty, z.scale, py, stageH)
  return tx === z.tx && ty === z.ty ? z : { scale: z.scale, tx, ty }
}

/** The CSS transform for the <video> (transform-origin 0 0). Empty at 1x: Phase 19's sharpness rule. */
export function videoTransform(z: ZoomState): string {
  return z.scale <= MIN_ZOOM ? '' : `translate(${z.tx}px, ${z.ty}px) scale(${z.scale})`
}

/** A pointer position in the picture (0..1 each way) to stage pixels at 1x, given the letterbox rect. */
export function pointerToStage(nx: number, ny: number, rect: VideoRect): { x: number; y: number } {
  return { x: rect.x + nx * rect.width, y: rect.y + ny * rect.height }
}
