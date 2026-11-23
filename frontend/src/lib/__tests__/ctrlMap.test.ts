// Phase 14-10 (MOB-01): the key bar's byte table.
import { describe, it, expect } from 'vitest'
import { arrowSeq, ctrlByte, ESC, PIPE, TAB, type ArrowDir } from '../ctrlMap'

describe('ctrlByte', () => {
  it('maps every letter, both cases, to 0x01-0x1A', () => {
    for (let i = 0; i < 26; i++) {
      const want = String.fromCharCode(i + 1)
      expect(ctrlByte(String.fromCharCode(0x61 + i))).toBe(want)
      expect(ctrlByte(String.fromCharCode(0x41 + i))).toBe(want)
    }
  })
  it('maps @, space and the four bracket symbols to C0', () => {
    expect(ctrlByte('@')).toBe('\x00')
    expect(ctrlByte(' ')).toBe('\x00')
    expect(ctrlByte('[')).toBe('\x1b')
    expect(ctrlByte('\\')).toBe('\x1c')
    expect(ctrlByte(']')).toBe('\x1d')
    expect(ctrlByte('^')).toBe('\x1e')
    expect(ctrlByte('_')).toBe('\x1f')
  })
  it('is null for digits, non-ASCII, empty and multi-character strings', () => {
    for (const s of ['1', '0', 'é', '', 'ab', '\r']) expect(ctrlByte(s)).toBeNull()
  })
})

describe('arrowSeq', () => {
  const finals: Record<ArrowDir, string> = { up: 'A', down: 'B', right: 'C', left: 'D' }
  for (const dir of Object.keys(finals) as ArrowDir[]) {
    it(`${dir}: normal, application and ctrl`, () => {
      const f = finals[dir]
      expect(arrowSeq(dir, false, false)).toBe(`\x1b[${f}`)
      expect(arrowSeq(dir, true, false)).toBe(`\x1bO${f}`)
      expect(arrowSeq(dir, false, true)).toBe(`\x1b[1;5${f}`)
      expect(arrowSeq(dir, true, true)).toBe(`\x1b[1;5${f}`)
    })
  }
})

describe('constants', () => {
  it('are ESC, TAB and the pipe', () => {
    expect([ESC, TAB, PIPE]).toEqual(['\x1b', '\x09', '|'])
  })
})
