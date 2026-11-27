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

/** A pointer position in the picture (0..1 each way) to stage pixels at 1x, given the letterbox rect. */
export function pointerToStage(nx: number, ny: number, rect: VideoRect): { x: number; y: number } {
  return { x: rect.x + nx * rect.width, y: rect.y + ny * rect.height }
}
