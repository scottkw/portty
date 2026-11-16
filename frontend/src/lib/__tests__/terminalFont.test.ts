import { describe, it, expect } from 'vitest'
import { DEFAULT_TERMINAL_FONT_STACK, DEFAULT_TERMINAL_FONT_SIZE, terminalFontFamily, parseTerminalFont } from '../terminalFont'

// Quick 261116-ku9 (R1): the user's family always goes double-quoted ahead of
// the fixed default stack so the Nerd Font fallbacks stay present.
describe('terminalFontFamily', () => {
  it('returns the default stack for a blank family', () => {
    expect(terminalFontFamily('')).toBe(DEFAULT_TERMINAL_FONT_STACK)
    expect(terminalFontFamily('   ')).toBe(DEFAULT_TERMINAL_FONT_STACK)
  })

  it('quotes the family ahead of the stack', () => {
    expect(terminalFontFamily('Hack Nerd Font Mono')).toBe(`"Hack Nerd Font Mono", ${DEFAULT_TERMINAL_FONT_STACK}`)
  })

  it('falls back to the stack for forbidden characters or an over-long name', () => {
    for (const bad of ['a"b', "a'b", 'a;b', 'a{b', 'a}b', 'a\\b', 'a\u0007b', 'x'.repeat(65)]) {
      expect(terminalFontFamily(bad)).toBe(DEFAULT_TERMINAL_FONT_STACK)
    }
  })

  it('keeps the Nerd Font fallbacks and monospace at the end of the stack', () => {
    expect(DEFAULT_TERMINAL_FONT_STACK.endsWith('"MesloLGS NF", "Symbols Nerd Font Mono", "Hack Nerd Font Mono", monospace')).toBe(true)
    expect(DEFAULT_TERMINAL_FONT_SIZE).toBe(13)
  })
})

// Quick 261116-ku9 (R1 web part): the host font on /api/plugin-config is re-checked.
describe('parseTerminalFont', () => {
  it('accepts a valid family and size', () => {
    expect(parseTerminalFont({ family: 'Menlo', size: 15 })).toEqual({ family: 'Menlo', size: 15 })
  })

  it('returns null for anything else', () => {
    for (const bad of [null, undefined, 'Menlo', 15, { family: 5, size: 15 }, { family: 'a;b', size: 15 },
      { family: 'Menlo', size: 14.5 }, { family: 'Menlo', size: 5 }, { family: 'Menlo', size: 33 }, { family: 'Menlo' }]) {
      expect(parseTerminalFont(bad)).toBeNull()
    }
  })
})
