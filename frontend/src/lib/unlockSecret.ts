// Helpers for the unlock request (23-UI-SPEC Surface 2, "Password handling in the viewer").
// Pure functions, no state, no logging. A JavaScript string cannot be erased, so the only
// copy this file can scrub is the byte array it builds; the text is the caller's to drop at once.

/** First byte of the binary unlock message on the input channel (23-04 wire contract). */
export const UNLOCK_MSG_TYPE = 0x01

const encoder = new TextEncoder()

/** The type byte, then the UTF-8 of `text`, in one array. The intermediate encoder output is zeroed before returning. */
export function encodeUnlock(text: string): Uint8Array {
  const body = encoder.encode(text)
  const out = new Uint8Array(body.length + 1)
  out[0] = UNLOCK_MSG_TYPE
  out.set(body, 1)
  body.fill(0)
  return out
}

/** Overwrites every byte with zero. Call it right after the send. */
export function zeroBytes(a: Uint8Array): void {
  a.fill(0)
}
