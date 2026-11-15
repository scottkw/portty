import { describe, it, expect } from 'vitest'
import { LIGHT_PALETTE, DARK_PALETTE } from '../terminalPalettes'
import * as again from '../terminalPalettes'

// THEM-01: the user is colour-blind, so every palette value is pinned here
// (copied from 02-UI-SPEC.md "Terminal palettes") instead of checked by eye.
const LIGHT: Record<string, string> = {
  background: '#FFFFFF', foreground: '#1D1D1F', cursor: '#1D1D1F', cursorAccent: '#FFFFFF',
  selectionBackground: 'rgba(0,99,204,0.25)',
  black: '#1D1D1F', brightBlack: '#6E6E73', red: '#CF222E', brightRed: '#A40E26',
  green: '#1A7F37', brightGreen: '#116329', yellow: '#9A6700', brightYellow: '#7D4E00',
  blue: '#0969DA', brightBlue: '#0550AE', magenta: '#8250DF', brightMagenta: '#6639BA',
  cyan: '#1B7C83', brightCyan: '#0F5E64', white: '#636366', brightWhite: '#8E8E93',
}
const DARK: Record<string, string> = {
  background: '#1C1C1E', foreground: '#D8D8DC', cursor: '#D8D8DC', cursorAccent: '#1C1C1E',
  selectionBackground: 'rgba(77,163,255,0.35)',
  black: '#48484A', brightBlack: '#8E8E93', red: '#FF7B72', brightRed: '#FFA198',
  green: '#7EE787', brightGreen: '#A6F4B0', yellow: '#E3B341', brightYellow: '#F2CC60',
  blue: '#79C0FF', brightBlue: '#A5D6FF', magenta: '#D2A8FF', brightMagenta: '#E2C5FF',
  cyan: '#76E3EA', brightCyan: '#B3F0FF', white: '#D8D8DC', brightWhite: '#FFFFFF',
}

describe('terminalPalettes (THEM-01)', () => {
  it('both specs list 21 keys', () => {
    expect(Object.keys(LIGHT)).toHaveLength(21)
    expect(Object.keys(DARK)).toHaveLength(21)
  })

  it('LIGHT_PALETTE matches UI-SPEC exactly', () => {
    expect(LIGHT_PALETTE).toEqual(LIGHT)
  })

  it('DARK_PALETTE matches UI-SPEC exactly', () => {
    expect(DARK_PALETTE).toEqual(DARK)
  })

  it('palettes are module constants (stable identity)', () => {
    expect(again.LIGHT_PALETTE).toBe(LIGHT_PALETTE)
    expect(again.DARK_PALETTE).toBe(DARK_PALETTE)
  })
})
