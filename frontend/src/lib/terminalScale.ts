/**
 * computeGuestScale — pure scale helper for the guest terminal viewer.
 *
 * VIEW-05: derives the CSS scale factor that fits the host grid (gridW × gridH
 * pixels) inside the guest container (containerW × containerH pixels).
 *
 * Rules:
 *  • s = min(containerW / gridW, containerH / gridH)
 *  • cap: s is never > 1.0 (downscale-only; never upscale a guest view)
 *  • guard: if gridW or gridH is ≤ 0, returns 1 (safe no-op; avoids divide-by-zero)
 */
export function computeGuestScale(
  containerW: number,
  containerH: number,
  gridW: number,
  gridH: number,
): number {
  if (gridW <= 0 || gridH <= 0) return 1
  const s = Math.min(containerW / gridW, containerH / gridH)
  return s > 1 ? 1 : s
}

/**
 * DEFAULT_GUEST_MIN_SCALE — readability floor for the guest terminal viewer
 * (BUG-01 / #128). At a 14px base font, ~0.7 scale renders ~10px effective
 * text — the floor below which the grid would keep shrinking to unreadable
 * sizes on a narrow phone viewport. Single source of truth shared by
 * TerminalPanel.recomputeScale and this file's tests.
 */
export const DEFAULT_GUEST_MIN_SCALE = 0.7

/**
 * computeGuestViewport — floor-aware guest scale helper (BUG-01 / #128).
 *
 * Extends computeGuestScale with a readability floor: below `minScale` the
 * guest viewer stops shrinking further and instead signals the caller to
 * enable horizontal scroll (overflowX: true), keeping text legible. The
 * never-upscale invariant (VIEW-04/05) is preserved — naturalCapped is
 * always ≤ 1 before the floor comparison.
 *
 * Rules:
 *  • natural = min(containerW / gridW, containerH / gridH)
 *  • naturalCapped = min(natural, 1)  — downscale-only, never upscale
 *  • naturalCapped >= minScale → { scale: naturalCapped, overflowX: false }
 *  • naturalCapped <  minScale → { scale: minScale, overflowX: true }
 *  • guard: gridW ≤ 0 or gridH ≤ 0 → { scale: 1, overflowX: false }
 */
export function computeGuestViewport(
  containerW: number,
  containerH: number,
  gridW: number,
  gridH: number,
  minScale: number = DEFAULT_GUEST_MIN_SCALE,
): { scale: number; overflowX: boolean } {
  if (gridW <= 0 || gridH <= 0) return { scale: 1, overflowX: false }
  const natural = Math.min(containerW / gridW, containerH / gridH)
  const naturalCapped = natural > 1 ? 1 : natural
  if (naturalCapped >= minScale) {
    return { scale: naturalCapped, overflowX: false }
  }
  return { scale: minScale, overflowX: true }
}

type Pointer = { clientX: number; clientY: number }

/**
 * unscalePointer — maps a pointer into an element's untransformed box
 * (selection-offset-copy). recomputeScale puts transform: scale(s) on
 * term.element, but xterm 6 turns a pointer into a cell as
 * (clientY - rect.top) / cellHeight, with a post-transform rect over an
 * unscaled cell, so hits land at row * s, above the pointer. Moving the
 * pointer (not the rect or the cell size) keeps xterm's padding and clamp
 * math right. s = rect.width / offsetWidth; an unmeasurable box passes through.
 */
export function unscalePointer(
  ev: Pointer,
  rect: { left: number; top: number; width: number },
  offsetWidth: number,
): Pointer {
  if (!(offsetWidth > 0) || !(rect.width > 0)) return { clientX: ev.clientX, clientY: ev.clientY }
  const s = rect.width / offsetWidth
  return {
    clientX: rect.left + (ev.clientX - rect.left) / s,
    clientY: rect.top + (ev.clientY - rect.top) / s,
  }
}

interface MouseServiceLike {
  getCoords(ev: Pointer, el: HTMLElement, ...rest: unknown[]): unknown
  getMouseReportCoords(ev: Pointer, el: HTMLElement): unknown
}

let warnedNoMouseService = false

/**
 * compensatePointerScale — routes xterm's one MouseService (shared by
 * selection, link hover/click and TUI mouse reports) through unscalePointer.
 * Call once per Terminal, after term.open.
 */
export function compensatePointerScale(term: { element?: HTMLElement | null }): void {
  const ms = (term as { _core?: { _mouseService?: MouseServiceLike } })._core?._mouseService
  if (!ms) {
    // No silent fallback: an xterm upgrade that renames this would bring the offset back.
    if (term.element && !warnedNoMouseService) {
      warnedNoMouseService = true
      console.warn('compensatePointerScale: xterm _core._mouseService not found; selection will drift on a scaled pane')
    }
    return
  }
  // A fresh { clientX, clientY } each call: spreading a MouseEvent yields {} (getters).
  const unscale = (ev: Pointer, el: HTMLElement) => unscalePointer(ev, el.getBoundingClientRect(), el.offsetWidth)
  const getCoords = ms.getCoords
  const getReport = ms.getMouseReportCoords
  ms.getCoords = (ev, el, ...rest) => getCoords.call(ms, unscale(ev, el), el, ...rest)
  ms.getMouseReportCoords = (ev, el) => getReport.call(ms, unscale(ev, el), el)
}
