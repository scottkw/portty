import { encodeRmove } from './desktopInput'

/**
 * Phase 21-10 (DVWR-03) — what the iPhone viewer sends on Phase 20's input channel. Pure TypeScript,
 * every dependency injected. The messages are Phase 20's frozen v1 encoders (desktopInput.ts); this
 * file only decides when they go out. The phone never holds a pointer of its own: the Mac's pointer
 * moves by relative moves and the Mac reports where it is.
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
}

export function createPhoneInput(deps: PhoneInputDeps): PhoneInput {
  return {
    rmove(ndx, ndy) {
      const e = deps.epoch()
      if (e === null) return
      deps.sendInput(encodeRmove(ndx, ndy, e), true)
    },
  }
}
