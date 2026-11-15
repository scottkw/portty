import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// UI-01: WCAG AA contrast regression tests for non-terminal GUI text.
const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

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

describe('UI-01: tab bar contrast — no failing #565f89 text color', () => {
  it('.tab rule does not use #565f89 for color', () => {
    expect(css).not.toMatch(/\.tab\s*\{[^}]*color:\s*#565f89/)
  })

  it('.tab__close rule does not use #565f89', () => {
    expect(css).not.toMatch(/\.tab__close\s*\{[^}]*color:\s*#565f89/)
  })

})

describe('UI-01: settings panel contrast — no failing #565f89 text color', () => {
  it('.settings-panel__body h3 does not use #565f89', () => {
    expect(css).not.toMatch(/\.settings-panel__body\s+h3\s*\{[^}]*color:\s*#565f89/)
  })

  it('.settings-panel__description does not use #565f89', () => {
    expect(css).not.toMatch(/\.settings-panel__description\s*\{[^}]*color:\s*#565f89/)
  })

  it('the removed Settings → Paths table and empty-state rules are gone (Portty 01-11)', () => {
    expect(css).not.toMatch(/\.settings-panel__(empty|table)\b/)
  })

  it('.settings-panel__url does not use #565f89', () => {
    expect(css).not.toMatch(/\.settings-panel__url\s*\{[^}]*color:\s*#565f89/)
  })
})

describe('UI-01: welcome tab contrast — no failing #565f89 text color', () => {
  it('.welcome-tab__version does not use #565f89', () => {
    expect(css).not.toMatch(/\.welcome-tab__version\s*\{[^}]*color:\s*#565f89/)
  })

  it('.welcome-tab__heading does not use #565f89', () => {
    expect(css).not.toMatch(/\.welcome-tab__heading\s*\{[^}]*color:\s*#565f89/)
  })
})

describe('UI-01: modal contrast — no failing #565f89 text color', () => {
  it('.new-session-modal__section-label does not use #565f89', () => {
    expect(css).not.toMatch(/\.new-session-modal__section-label\s*\{[^}]*color:\s*#565f89/)
  })
})

describe('UI-01: intentionally-dim elements preserved', () => {
  // Portty 02-08: the status bar and its dim "WEB SERVER NOT RUNNING" state are removed.
  it('the status bar rules are gone', () => {
    expect(css).not.toContain('.tab-status-bar')
  })
})
