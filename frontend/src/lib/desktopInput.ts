import type { VideoRect } from '../components/DesktopView'
import { guardPasteInsert } from './pasteGuard'

/**
 * Phase 20-06 — the browser half of control. Pointer, keyboard and text capture on
 * the viewer stage, encoded as the frozen v1 messages of internal/desktop/testdata/
 * input-v1.jsonl and sent on the `input` data channel. Pure TypeScript with no React
 * and no Wails imports, and every dependency injectable, so the Phase 21 Wails tab and
 * iPhone layer reuse it and vitest drives it with fakes.
 *
 * Pointer, wheel and context-menu listeners sit on the stage; key, composition and
 * text listeners sit on the stage's hidden text input, so keys are captured only while
 * it has focus (a user action). window and document get exactly the release triggers
 * (blur, pagehide, visibilitychange), never a key or pointer listener: stray typing
 * must not reach the Mac (T-20-31).
 */

export const SWAP_KEY = 'portty:desktop-swap-ctrl-cmd'
export const HINT_KEY = 'portty:desktop-release-hint-seen'
export const HEARTBEAT_MS = 1000
export const MAX_TEXT_BYTES = 1024
export const MOVE_BUFFER_LIMIT = 65536
export const LINE_PX = 16
export const MAX_WHEEL = 2400

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
  /** localStorage for the Swap setting; null or a storage that throws reads as off. */
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
  swap(): boolean
  setSwap(on: boolean): void
  /** Taps a chord for Send Keys: modifiers down in order, the key, then everything up in reverse. */
  sendChord(codes: string[]): void
  held(): number
  destroy(): void
}

const KEEP_FOCUS: ReadonlySet<ReleaseReason> = new Set<ReleaseReason>(['lock', 'surface', 'disconnect'])

const clamp01 = (v: number) => Math.min(1, Math.max(0, v))
const clampWheel = (v: number) => Math.max(-MAX_WHEEL, Math.min(MAX_WHEEL, v))

const SWAPPED: Record<string, string> = {
  ControlLeft: 'MetaLeft', MetaLeft: 'ControlLeft', ControlRight: 'MetaRight', MetaRight: 'ControlRight',
}
/** Ctrl and Cmd exchanged (the Swap setting): the Control and Meta pairs, nothing else. */
export const swapCode = (code: string): string => SWAPPED[code] ?? code

const MODIFIER_BIT: Record<string, number> = {
  ShiftLeft: 1, ShiftRight: 1, ControlLeft: 2, ControlRight: 2, AltLeft: 4, AltRight: 4, MetaLeft: 8, MetaRight: 8,
}
const MODIFIER_CODES: ReadonlySet<string> = new Set([...Object.keys(MODIFIER_BIT), 'CapsLock'])
const MODIFIER_KEY: Record<string, string> = {
  ShiftLeft: 'Shift', ShiftRight: 'Shift', ControlLeft: 'Control', ControlRight: 'Control',
  AltLeft: 'Alt', AltRight: 'Alt', MetaLeft: 'Meta', MetaRight: 'Meta',
}

/** Ctrl+Option+Cmd+Return on the physical keys, read before the Swap. */
export function isReleaseChord(e: Pick<KeyboardEvent, 'ctrlKey' | 'altKey' | 'metaKey' | 'code'>): boolean {
  return e.ctrlKey && e.altKey && e.metaKey && e.code === 'Enter'
}

/** UTF-8 bytes of one code point. */
export const utf8Len = (cp: number) => (cp < 0x80 ? 1 : cp < 0x800 ? 2 : cp < 0x10000 ? 3 : 4)
/** `ev.buttons` bit of each `ev.button` index (0 left, 1 middle, 2 right). */
const BUTTON_BIT = [1, 4, 2]
const JOINER = /^[‍︎️\p{M}]$/u

/**
 * Splits composed text into pieces of at most maxBytes UTF-8 bytes, at code-point
 * boundaries, keeping a ZWJ, a variation selector or a combining mark with the code point
 * before it (and the code point a ZWJ joins). A cluster longer than a piece is cut between
 * code points, never inside a surrogate pair.
 */
export function splitText(s: string, maxBytes: number): string[] {
  const clusters: string[] = []
  let joinNext = false
  for (const ch of s) {
    if (clusters.length > 0 && (joinNext || JOINER.test(ch))) clusters[clusters.length - 1] += ch
    else clusters.push(ch)
    joinNext = ch === '‍'
  }
  const bytesOf = (c: string) => [...c].reduce((n, ch) => n + utf8Len(ch.codePointAt(0)!), 0)
  const pieces: string[] = []
  let cur = ''
  let curBytes = 0
  const push = (c: string, n: number) => {
    if (curBytes + n > maxBytes && cur) {
      pieces.push(cur)
      cur = ''
      curBytes = 0
    }
    cur += c
    curBytes += n
  }
  for (const c of clusters) {
    const n = bytesOf(c)
    if (n <= maxBytes) {
      push(c, n)
      continue
    }
    for (const ch of c) push(ch, utf8Len(ch.codePointAt(0)!))
  }
  if (cur) pieces.push(cur)
  return pieces
}

export function createDesktopInput(
  stage: HTMLElement,
  input: HTMLTextAreaElement,
  deps: DesktopInputDeps,
  events: DesktopInputEvents = {},
): DesktopInput {
  const win = deps.win ?? window
  const doc = deps.doc ?? document
  const now = deps.now ?? (() => performance.now())
  const raf = deps.raf ?? ((cb) => win.requestAnimationFrame(cb))
  const cancelRaf = deps.cancelRaf ?? ((id) => win.cancelAnimationFrame(id))
  const setIv = deps.setInterval ?? ((fn, ms) => win.setInterval(fn, ms))
  const clearIv = deps.clearInterval ?? ((id) => win.clearInterval(id as number))
  const { send, getRect, getEpoch } = deps

  let swapOn = false
  try {
    swapOn = deps.storage?.getItem(SWAP_KEY) === '1'
  } catch {
    // Safari private mode can throw: the setting reads as off.
  }

  let control = false
  let paused = false
  let composing = false
  let quietBlur = false
  const buttons = new Set<number>()
  // Physical code -> the code actually sent (after the Swap), so a keyup follows its own keydown.
  const forwarded = new Map<string, string>()
  let pending: { x: number; y: number; cx: number; cy: number; visible: boolean } | null = null
  let rafId: number | null = null
  let hbTimer: unknown = null

  const canSend = () => control && !paused && getEpoch() !== null
  const held = () => buttons.size + forwarded.size

  function syncHeartbeat() {
    if (held() > 0 && hbTimer === null) hbTimer = setIv(() => send(HB), HEARTBEAT_MS)
    else if (held() === 0 && hbTimer !== null) {
      clearIv(hbTimer)
      hbTimer = null
    }
  }

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
    syncHeartbeat()
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

  /**
   * Pointer Events deliver a second button's press and release as pointermove and fire
   * pointerup only for the last button, so every held button that `ev.buttons` no longer
   * shows (or that is `up`) is released here. Returns whether anything was released.
   */
  function releaseGone(ev: PointerEvent, up: number): boolean {
    const gone = [...buttons].filter((b) => b === up || !(ev.buttons & BUTTON_BIT[b]))
    if (gone.length === 0) return false
    for (const b of gone) buttons.delete(b)
    syncHeartbeat()
    pending = null
    const e = getEpoch()
    const at = locate(ev.clientX, ev.clientY)
    // A release always goes out; without a position or epoch the daemon's own release stands in.
    if (at && e !== null) for (const b of gone) send(encodeBtn(b, false, at.x, at.y, e, Math.round(now())))
    else send(RELEASE)
    return true
  }

  function onPointerMove(ev: PointerEvent) {
    if (!control) return
    releaseGone(ev, -1) // before locate: a release is never skipped when the rect is unknown
    const at = locate(ev.clientX, ev.clientY)
    if (!at) return
    // The system candidate window opens beside the pointer, so the hidden input follows it.
    input.style.transform = `translate(${Math.round(at.px)}px, ${Math.round(at.py)}px)`
    const heldBtn = buttons.size > 0
    pending = { x: at.x, y: at.y, cx: at.px, cy: at.py, visible: at.inside || heldBtn }
    rafId ??= raf(flush)
  }

  function onPointerUp(ev: PointerEvent) {
    releaseGone(ev, ev.button)
  }

  function onPointerLeave() {
    pending = null
    events.onDot?.(null)
  }

  const onContextMenu = (ev: Event) => {
    if (control) ev.preventDefault()
  }

  function onWheel(ev: WheelEvent) {
    if (!control) return
    // Nothing scrolls or zooms the viewer page while it controls the Mac; a pinch (ctrlKey) goes nowhere.
    ev.preventDefault()
    if (ev.ctrlKey) return
    const e = getEpoch()
    if (paused || e === null) return
    const at = locate(ev.clientX, ev.clientY)
    if (!at || !at.inside) return
    const unit = ev.deltaMode === 1 ? LINE_PX : ev.deltaMode === 2 ? stage.clientHeight : 1
    send(encodeWheel(clampWheel(ev.deltaX * unit), clampWheel(ev.deltaY * unit), e))
  }

  /** Bits: 1 shift, 2 ctrl, 4 alt, 8 meta, with Ctrl and Cmd exchanged when swapped. */
  function maskOf(ev: Pick<KeyboardEvent, 'shiftKey' | 'ctrlKey' | 'altKey' | 'metaKey'>, swapped: boolean): number {
    const ctrl = swapped ? ev.metaKey : ev.ctrlKey
    const meta = swapped ? ev.ctrlKey : ev.metaKey
    return (ev.shiftKey ? 1 : 0) | (ctrl ? 2 : 0) | (ev.altKey ? 4 : 0) | (meta ? 8 : 0)
  }

  /** macOS drops the keyup of a key released while Cmd is down, so Cmd going down or up lets go of held non-modifiers. */
  function releaseUnderCmd(ev: KeyboardEvent) {
    if (ev.code !== 'MetaLeft' && ev.code !== 'MetaRight') return
    for (const [code, sent] of forwarded) {
      if (MODIFIER_CODES.has(code)) continue
      forwarded.delete(code)
      send(encodeKey(sent, false, false, maskOf(ev, sent !== code), ev.getModifierState('CapsLock'), '', Math.round(now())))
    }
    syncHeartbeat()
  }

  function onKeyDown(ev: KeyboardEvent) {
    if (!control) return
    if (isReleaseChord(ev)) {
      ev.preventDefault()
      releaseAll('chord')
      events.onReleaseChord?.()
      return
    }
    // The input method owns it: Safari fires compositionend before the committing keydown, so only 229 is reliable.
    if (ev.isComposing || ev.keyCode === 229 || ev.key === 'Dead') return
    ev.preventDefault()
    releaseUnderCmd(ev)
    if (!canSend()) return
    const swapped = swapOn
    const sent = swapped ? swapCode(ev.code) : ev.code
    const m = maskOf(ev, swapped)
    const caps = ev.getModifierState('CapsLock')
    const ts = Math.round(now())
    send(encodeKey(sent, true, ev.repeat, m, caps, ev.key, ts))
    if (ev.metaKey && !MODIFIER_CODES.has(ev.code)) {
      // macOS delivers no keyup for a key pressed while Cmd is down, so a Cmd chord is a tap.
      send(encodeKey(sent, false, false, m, caps, ev.key, ts))
      return
    }
    forwarded.set(ev.code, sent)
    syncHeartbeat()
  }

  function onKeyUp(ev: KeyboardEvent) {
    if (!control) return
    releaseUnderCmd(ev)
    const sent = forwarded.get(ev.code)
    if (sent === undefined) return
    forwarded.delete(ev.code)
    syncHeartbeat()
    ev.preventDefault()
    // The mask follows the code the down went out with, so toggling Swap while held strands nothing.
    send(encodeKey(sent, false, false, maskOf(ev, sent !== ev.code), ev.getModifierState('CapsLock'), ev.key, Math.round(now())))
  }

  function sendText(s: string) {
    if (!canSend()) return
    for (const piece of splitText(s, MAX_TEXT_BYTES)) send(encodeText(piece))
  }

  const onCompositionStart = () => {
    composing = true
  }
  const onCompositionEnd = (ev: CompositionEvent) => {
    composing = false
    if (ev.data) sendText(ev.data)
    input.value = ''
  }
  // Emoji picker and dictation insert text without a key: send it, and keep it out of the input.
  const onBeforeInput = (ev: InputEvent) => {
    if (!control || ev.inputType !== 'insertText' || ev.isComposing || !ev.data) return
    ev.preventDefault()
    sendText(ev.data)
  }
  const onInput = () => {
    if (!composing) input.value = ''
  }
  const onFocus = () => {
    if (control) events.onCapture?.(true)
  }
  const onBlur = () => {
    if (control && !quietBlur) releaseAll('blur')
    events.onCapture?.(false)
  }
  const onStageFocus = () => {
    if (control) input.focus({ preventScroll: true })
  }

  /** Take focus off the hidden input so the keyboard is the viewer's own again. */
  function dropKeyboard() {
    quietBlur = true
    input.blur()
    quietBlur = false
  }

  function releaseAll(reason: ReleaseReason) {
    const n = held()
    buttons.clear()
    forwarded.clear()
    pending = null
    if (rafId !== null) {
      cancelRaf(rafId)
      rafId = null
    }
    syncHeartbeat()
    send(RELEASE)
    // A lock, a surface switch or a reconnect pauses control but the user still owns the keyboard;
    // every other release hands it back, so a returning window comes back as keyboard off.
    if (!KEEP_FOCUS.has(reason)) dropKeyboard()
    if (n > 0) events.onReleased?.(reason, n)
  }

  const onWinBlur = () => control && releaseAll('blur')
  const onPageHide = () => control && releaseAll('pagehide')
  const onVisibility = () => {
    if (control && doc.visibilityState === 'hidden') releaseAll('hidden')
  }

  stage.addEventListener('pointerdown', onPointerDown)
  stage.addEventListener('pointermove', onPointerMove)
  stage.addEventListener('pointerup', onPointerUp)
  stage.addEventListener('pointercancel', onPointerUp)
  stage.addEventListener('pointerleave', onPointerLeave)
  stage.addEventListener('contextmenu', onContextMenu)
  stage.addEventListener('wheel', onWheel, { passive: false })
  stage.addEventListener('focus', onStageFocus)
  input.addEventListener('keydown', onKeyDown)
  input.addEventListener('keyup', onKeyUp)
  input.addEventListener('compositionstart', onCompositionStart)
  input.addEventListener('compositionend', onCompositionEnd)
  input.addEventListener('beforeinput', onBeforeInput)
  input.addEventListener('input', onInput)
  input.addEventListener('focus', onFocus)
  input.addEventListener('blur', onBlur)
  const unguardPaste = guardPasteInsert(input)
  win.addEventListener('blur', onWinBlur)
  win.addEventListener('pagehide', onPageHide)
  doc.addEventListener('visibilitychange', onVisibility)

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
    swap: () => swapOn,
    setSwap(on) {
      swapOn = on
      try {
        deps.storage?.setItem(SWAP_KEY, on ? '1' : '0')
      } catch {
        // Not persisted (private mode); the choice still holds for this page.
      }
    },
    sendChord(codes) {
      if (!canSend()) return
      let m = 0
      const ts = Math.round(now())
      const down: string[] = []
      for (const code of codes) {
        m |= MODIFIER_BIT[code] ?? 0
        send(encodeKey(code, true, false, m, false, MODIFIER_KEY[code] ?? code, ts))
        down.push(code)
      }
      for (const code of down.reverse()) {
        m &= ~(MODIFIER_BIT[code] ?? 0)
        send(encodeKey(code, false, false, m, false, MODIFIER_KEY[code] ?? code, ts))
      }
    },
    held,
    destroy() {
      stage.removeEventListener('pointerdown', onPointerDown)
      stage.removeEventListener('pointermove', onPointerMove)
      stage.removeEventListener('pointerup', onPointerUp)
      stage.removeEventListener('pointercancel', onPointerUp)
      stage.removeEventListener('pointerleave', onPointerLeave)
      stage.removeEventListener('contextmenu', onContextMenu)
      stage.removeEventListener('wheel', onWheel)
      stage.removeEventListener('focus', onStageFocus)
      input.removeEventListener('keydown', onKeyDown)
      input.removeEventListener('keyup', onKeyUp)
      input.removeEventListener('compositionstart', onCompositionStart)
      input.removeEventListener('compositionend', onCompositionEnd)
      input.removeEventListener('beforeinput', onBeforeInput)
      input.removeEventListener('input', onInput)
      input.removeEventListener('focus', onFocus)
      input.removeEventListener('blur', onBlur)
      unguardPaste()
      win.removeEventListener('blur', onWinBlur)
      win.removeEventListener('pagehide', onPageHide)
      doc.removeEventListener('visibilitychange', onVisibility)
      if (rafId !== null) cancelRaf(rafId)
      rafId = null
      if (hbTimer !== null) clearIv(hbTimer)
      hbTimer = null
    },
  }
}
