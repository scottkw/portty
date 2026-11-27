import {
  HB,
  HEARTBEAT_MS,
  MAX_TEXT_BYTES,
  MAX_WHEEL,
  RELEASE,
  encodeBtn,
  encodeKey,
  encodeRmove,
  encodeText,
  encodeWheel,
  splitText,
} from './desktopInput'

/**
 * Phase 21-10 (DVWR-03) — what the iPhone viewer sends on Phase 20's input channel. Pure TypeScript,
 * every dependency injected. The messages are Phase 20's frozen v1 encoders (desktopInput.ts); this
 * file only decides when they go out. The phone never holds a pointer of its own: the Mac's pointer
 * moves by relative moves and the Mac reports where it is.
 *
 * Only a mouse button can stay held (a key is always sent as a complete tap or chord), so the
 * heartbeat that feeds Phase 20's dead-man runs only while a button is down. Window blur, pagehide
 * and a hidden page release everything; the layer adds the channel close and the keyboard going down.
 */

export interface PhoneInputDeps {
  /** One message on the input channel; false when it was not sent. Moves are droppable. */
  sendInput(msg: object, droppable?: boolean): boolean
  /** The surface epoch the client last received, or null when it does not know one yet. */
  epoch(): number | null
  now(): number
  setInterval(fn: () => void, ms: number): unknown
  clearInterval(id: unknown): void
  win: Window
  doc: Document
}

export interface PhoneInput {
  /** A relative move in fractions of the picture; nothing goes out until the epoch is known. */
  rmove(ndx: number, ndy: number): void
  /** A complete click (down, up) at a normalized picture position; ts is the viewer time of the tap. */
  click(x: number, y: number, button: number, ts: number): void
  /** Two click pairs, each carrying its own real tap time. */
  doubleClick(x: number, y: number, ts: [number, number]): void
  /** A button goes down and stays held (heartbeat on) until release or releaseAll. */
  press(x: number, y: number, button: number, ts: number): void
  release(x: number, y: number, button: number, ts: number): void
  /** Pixel wheel deltas for the Mac, clamped to the wire limit. */
  wheel(dx: number, dy: number): void
  /** One key down and up with the modifier mask (1 shift, 2 ctrl, 4 alt, 8 meta). */
  keyTap(code: string, mask: number): void
  /** Text of any length as pieces of at most 1 KiB. */
  text(s: string): void
  /** Codes down in order, then up in reverse, the mask following: modifiers first, the key last. */
  chord(codes: string[]): void
  /** Release message; clears what is held. */
  releaseAll(): void
  held(): number
  destroy(): void
}

// Left-hand codes only: the phone never sends a right-hand modifier.
const MODIFIER: Record<string, { bit: number; key: string }> = {
  ShiftLeft: { bit: 1, key: 'Shift' },
  ControlLeft: { bit: 2, key: 'Control' },
  AltLeft: { bit: 4, key: 'Alt' },
  MetaLeft: { bit: 8, key: 'Meta' },
}

const clampWheel = (v: number) => Math.max(-MAX_WHEEL, Math.min(MAX_WHEEL, v))

export function createPhoneInput(deps: PhoneInputDeps): PhoneInput {
  const { sendInput, epoch, now, win, doc } = deps
  const buttons = new Set<number>()
  let hb: unknown = null

  function syncHeartbeat() {
    if (buttons.size > 0 && hb === null) hb = deps.setInterval(() => sendInput(HB), HEARTBEAT_MS)
    else if (buttons.size === 0 && hb !== null) {
      deps.clearInterval(hb)
      hb = null
    }
  }

  function button(x: number, y: number, b: number, down: boolean, ts: number) {
    const e = epoch()
    if (e === null) return false
    sendInput(encodeBtn(b, down, x, y, e, Math.round(ts)))
    return true
  }

  function click(x: number, y: number, b: number, ts: number) {
    if (button(x, y, b, true, ts)) button(x, y, b, false, ts)
  }

  function releaseAll() {
    buttons.clear()
    syncHeartbeat()
    sendInput(RELEASE)
  }

  const onWinBlur = () => releaseAll()
  const onPageHide = () => releaseAll()
  const onVisibility = () => {
    if (doc.visibilityState === 'hidden') releaseAll()
  }
  win.addEventListener('blur', onWinBlur)
  win.addEventListener('pagehide', onPageHide)
  doc.addEventListener('visibilitychange', onVisibility)

  return {
    rmove(ndx, ndy) {
      const e = epoch()
      if (e === null) return
      sendInput(encodeRmove(ndx, ndy, e), true)
    },
    click,
    doubleClick(x, y, ts) {
      click(x, y, 0, ts[0])
      click(x, y, 0, ts[1])
    },
    press(x, y, b, ts) {
      if (!button(x, y, b, true, ts)) return
      buttons.add(b)
      syncHeartbeat()
    },
    release(x, y, b, ts) {
      if (!buttons.delete(b)) return
      syncHeartbeat()
      // A release always goes out; without an epoch the daemon's own release stands in.
      if (!button(x, y, b, false, ts)) sendInput(RELEASE)
    },
    wheel(dx, dy) {
      const e = epoch()
      if (e === null) return
      sendInput(encodeWheel(clampWheel(dx), clampWheel(dy), e))
    },
    keyTap(code, mask) {
      const ts = Math.round(now())
      sendInput(encodeKey(code, true, false, mask, false, code, ts))
      sendInput(encodeKey(code, false, false, mask, false, code, ts))
    },
    text(s) {
      for (const piece of splitText(s, MAX_TEXT_BYTES)) sendInput(encodeText(piece))
    },
    chord(codes) {
      let m = 0
      const ts = Math.round(now())
      const down: string[] = []
      for (const code of codes) {
        m |= MODIFIER[code]?.bit ?? 0
        sendInput(encodeKey(code, true, false, m, false, MODIFIER[code]?.key ?? code, ts))
        down.push(code)
      }
      for (const code of down.reverse()) {
        m &= ~(MODIFIER[code]?.bit ?? 0)
        sendInput(encodeKey(code, false, false, m, false, MODIFIER[code]?.key ?? code, ts))
      }
    },
    releaseAll,
    held: () => buttons.size,
    destroy() {
      win.removeEventListener('blur', onWinBlur)
      win.removeEventListener('pagehide', onPageHide)
      doc.removeEventListener('visibilitychange', onVisibility)
      if (hb !== null) deps.clearInterval(hb)
      hb = null
    },
  }
}
