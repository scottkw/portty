import { describe, it, expect } from 'vitest'

// UI-01: WCAG AA contrast regression tests for non-terminal GUI text.
// Portty 02-12: the old #565f89 / removed-selector pins were dropped; style.tokens.test.ts
// pins these values in style.css and bans literal colours elsewhere.

function sRGB(c: number): number {
  c = c / 255
  return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
}
function relativeLuminance(hex: string): number {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return 0.2126 * sRGB(r) + 0.7152 * sRGB(g) + 0.0722 * sRGB(b)
}
function contrastRatio(fg: string, bg: string): number {
  const L1 = relativeLuminance(fg)
  const L2 = relativeLuminance(bg)
  return (Math.max(L1, L2) + 0.05) / (Math.min(L1, L2) + 0.05)
}

// Portty 02-03: retargeted from the retired comp palette to the UI-SPEC
// --color-text-secondary pairs, in both modes.
describe('UI-01: --color-text-secondary passes WCAG AA on every chrome surface (both modes)', () => {
  const pairs: Array<[string, string, string]> = [
    ['light sidebar', '#636366', '#ECECEE'],
    ['light toolbar', '#636366', '#F6F6F7'],
    ['light window', '#636366', '#FFFFFF'],
    ['dark sidebar', '#A1A1A6', '#262628'],
    ['dark toolbar', '#A1A1A6', '#2B2B2D'],
    ['dark window', '#A1A1A6', '#1E1E1E'],
  ]
  for (const [name, fg, bg] of pairs) {
    it(`${fg} on ${bg} (${name}) >= 4.5`, () => {
      expect(contrastRatio(fg, bg)).toBeGreaterThanOrEqual(4.5)
    })
  }
})
