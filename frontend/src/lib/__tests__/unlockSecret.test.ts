// Phase 23-07 — encodeUnlock and zeroBytes (23-UI-SPEC Surface 2, password handling). Test strings are made-up sentinels.

import { describe, it, expect } from 'vitest'
import { UNLOCK_MSG_TYPE, encodeUnlock, zeroBytes } from '../unlockSecret'

describe('encodeUnlock', () => {
  it('starts with the 0x01 type byte and carries the text as UTF-8', () => {
    const b = encodeUnlock('abc')
    expect(UNLOCK_MSG_TYPE).toBe(1)
    expect(Array.from(b)).toEqual([1, 0x61, 0x62, 0x63])
  })

  it('round-trips multi-byte UTF-8: accents, CJK and an emoji', () => {
    const text = 'pässwörd-密码-🔑'
    const b = encodeUnlock(text)
    expect(b[0]).toBe(1)
    expect(new TextDecoder().decode(b.subarray(1))).toBe(text)
    expect(b.length).toBe(1 + new TextEncoder().encode(text).length)
  })

  it('an empty string is just the type byte', () => {
    expect(Array.from(encodeUnlock(''))).toEqual([1])
  })
})

describe('zeroBytes', () => {
  it('leaves every byte 0, the type byte included', () => {
    const b = encodeUnlock('sentinel-pw-0451')
    zeroBytes(b)
    expect(b.length).toBe(17)
    expect(b.every((x) => x === 0)).toBe(true)
  })
})
