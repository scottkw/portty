import type { VideoRect } from '../components/DesktopView'

/**
 * Phase 20-06 — the browser half of control. Pointer, keyboard and text capture on
 * the viewer stage, encoded as the frozen v1 messages of internal/desktop/testdata/
 * input-v1.jsonl and sent on the `input` data channel. Pure TypeScript with no React
 * and no Wails imports, and every dependency injectable, so the Phase 21 Wails tab and
 * iPhone layer reuse it and vitest drives it with fakes.
 *
 * Listeners sit on the stage and on its hidden text input only. window and document
 * get exactly the release triggers (blur, pagehide, visibilitychange), never a key or
 * pointer listener: stray typing must not reach the Mac (T-20-31).
 */

export const HEARTBEAT_MS = 1000
export const MOVE_BUFFER_LIMIT = 65536

export type InputMsg =
  | { t: 'move'; x: number; y: number; e: number }
  | { t: 'rmove'; dx: number; dy: number; e: number }
  | { t: 'btn'; b: number; d: boolean; x: number; y: number; e: number; ts: number }
  | { t: 'wheel'; dx: number; dy: number; e: number }
  | { t: 'key'; code: string; d: boolean; r: boolean; m: number; caps: boolean; k: string; ts: number }
  | { t: 'text'; s: string }
  | { t: 'hb' }
  | { t: 'release' }

export const HB: InputMsg = { t: 'hb' }
export const RELEASE: InputMsg = { t: 'release' }

export const encodeMove = (x: number, y: number, e: number): InputMsg => ({ t: 'move', x, y, e })
export const encodeRmove = (dx: number, dy: number, e: number): InputMsg => ({ t: 'rmove', dx, dy, e })
export const encodeBtn = (b: number, d: boolean, x: number, y: number, e: number, ts: number): InputMsg =>
  ({ t: 'btn', b, d, x, y, e, ts })
export const encodeWheel = (dx: number, dy: number, e: number): InputMsg => ({ t: 'wheel', dx, dy, e })

export const encodeKey = (
  code: string, d: boolean, r: boolean, m: number, caps: boolean, k: string, ts: number,
): InputMsg => ({ t: 'key', code, d, r, m, caps, k, ts })
export const encodeText = (s: string): InputMsg => ({ t: 'text', s })

export type ReleaseReason =
  | 'blur' | 'hidden' | 'pagehide' | 'channel' | 'view-only' | 'chord'
  | 'lock' | 'surface' | 'disconnect' | 'menu'

export interface DesktopInputDeps {
  /** One message on the input channel; false when it was not sent. Moves are droppable. */
  send(msg: InputMsg, droppable?: boolean): boolean
  /** The picture's rectangle inside the stage (videoContentRect), in CSS pixels. */
  getRect(): VideoRect
  /** The surface epoch the client last received, or null when it does not know one yet. */
  getEpoch(): number | null
  now?: () => number
  raf?: (cb: () => void) => number
  cancelRaf?: (id: number) => void
  setInterval?: (fn: () => void, ms: number) => unknown
  clearInterval?: (id: unknown) => void
  win?: Window
  doc?: Document
  storage?: Pick<Storage, 'getItem' | 'setItem'> | null
}

export interface DesktopInputEvents {
  /** The local pointer dot, on the animation frame of the move; null hides it. */
  onDot?: (p: { x: number; y: number } | null) => void
  /** The hidden input gained or lost focus: the keyboard is captured or not. */
  onCapture?: (captured: boolean) => void
  /** Ctrl+Option+Cmd+Return: everything is released and the keyboard is given back. */
  onReleaseChord?: () => void
  /** Something was held when a release ran (the host decides which reasons deserve a notice). */
  onReleased?: (reason: ReleaseReason, held: number) => void
}

export interface DesktopInput {
  setControl(on: boolean): void
  /** True while the viewer must not send (locked, reconnecting, a switch in flight). */
  setPaused(on: boolean): void
  releaseAll(reason: ReleaseReason): void
  held(): number
  destroy(): void
}

const clamp01 = (v: number) => Math.min(1, Math.max(0, v))

export function createDesktopInput(
  stage: HTMLElement,
  input: HTMLTextAreaElement,
  deps: DesktopInputDeps,
  events: DesktopInputEvents = {},
): DesktopInput {
  const win = deps.win ?? window
  const now = deps.now ?? (() => performance.now())
  const raf = deps.raf ?? ((cb) => win.requestAnimationFrame(cb))
  const cancelRaf = deps.cancelRaf ?? ((id) => win.cancelAnimationFrame(id))
  const { send, getRect, getEpoch } = deps

  let control = false
  let paused = false
  const buttons = new Set<number>()
  let pending: { x: number; y: number; cx: number; cy: number; visible: boolean } | null = null
  let rafId: number | null = null

  const canSend = () => control && !paused && getEpoch() !== null

  /** Normalised 0..1 inside the picture; `inside` says whether the point is on the picture itself. */
  function locate(clientX: number, clientY: number) {
    const sr = stage.getBoundingClientRect()
    const px = clientX - sr.left
    const py = clientY - sr.top
    const r = getRect()
    if (r.width <= 0 || r.height <= 0) return null
    const nx = (px - r.x) / r.width
    const ny = (py - r.y) / r.height
    const inside = nx >= 0 && nx <= 1 && ny >= 0 && ny <= 1
    return { x: clamp01(nx), y: clamp01(ny), inside, px, py }
  }

  function flush() {
    rafId = null
    const p = pending
    pending = null
    if (!p) return
    const e = getEpoch()
    if (p.visible && canSend() && e !== null) send(encodeMove(p.x, p.y, e), true)
    events.onDot?.(p.visible && control && !paused ? { x: p.cx, y: p.cy } : null)
  }

  function onPointerDown(ev: PointerEvent) {
    if (!control || ev.button < 0 || ev.button > 2) return
    const e = getEpoch()
    if (paused || e === null) return
    const at = locate(ev.clientX, ev.clientY)
    if (!at || !at.inside) return
    buttons.add(ev.button)
    pending = null
    // A click on the picture is the user action that gives the keyboard to the Mac, and it is forwarded too.
    input.focus({ preventScroll: true })
    send(encodeBtn(ev.button, true, at.x, at.y, e, Math.round(now())))
    try {
      stage.setPointerCapture?.(ev.pointerId)
    } catch {
      // A synthetic or already-released pointer: the press was still sent and the up will follow.
    }
  }

  function onPointerMove(ev: PointerEvent) {
    if (!control) return
    const at = locate(ev.clientX, ev.clientY)
    if (!at) return
    const held = buttons.size > 0
    pending = { x: at.x, y: at.y, cx: at.px, cy: at.py, visible: at.inside || held }
    rafId ??= raf(flush)
  }

  function onPointerUp(ev: PointerEvent) {
    if (!buttons.delete(ev.button)) return
    pending = null
    const e = getEpoch()
    const at = locate(ev.clientX, ev.clientY)
    // A release always goes out; without a position or epoch the daemon's own release stands in.
    if (at && e !== null) send(encodeBtn(ev.button, false, at.x, at.y, e, Math.round(now())))
    else send(RELEASE)
  }

  function onPointerLeave() {
    pending = null
    events.onDot?.(null)
  }

  const onContextMenu = (ev: Event) => {
    if (control) ev.preventDefault()
  }

  stage.addEventListener('pointerdown', onPointerDown)
  stage.addEventListener('pointermove', onPointerMove)
  stage.addEventListener('pointerup', onPointerUp)
  stage.addEventListener('pointercancel', onPointerUp)
  stage.addEventListener('pointerleave', onPointerLeave)
  stage.addEventListener('contextmenu', onContextMenu)

  function releaseAll(reason: ReleaseReason) {
    const n = buttons.size
    buttons.clear()
    pending = null
    if (rafId !== null) {
      cancelRaf(rafId)
      rafId = null
    }
    send(RELEASE)
    if (n > 0) events.onReleased?.(reason, n)
  }

  return {
    setControl(on) {
      control = on
      if (!on) {
        releaseAll('view-only')
        events.onDot?.(null)
      }
    },
    setPaused(on) {
      paused = on
      if (on) events.onDot?.(null)
    },
    releaseAll,
    held: () => buttons.size,
    destroy() {
      stage.removeEventListener('pointerdown', onPointerDown)
      stage.removeEventListener('pointermove', onPointerMove)
      stage.removeEventListener('pointerup', onPointerUp)
      stage.removeEventListener('pointercancel', onPointerUp)
      stage.removeEventListener('pointerleave', onPointerLeave)
      stage.removeEventListener('contextmenu', onContextMenu)
      if (rafId !== null) cancelRaf(rafId)
      rafId = null
    },
  }
}
