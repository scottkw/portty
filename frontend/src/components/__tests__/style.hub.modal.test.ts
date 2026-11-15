import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 134 — Hub modal CSS contract tests.
// Pattern: mirrors style.hub.test.ts (reads raw CSS text, block-finder helper for structural assertions).
const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// Portty 02-09 (LOOK-01): the Share dialog moved onto the shared sheet shell,
// so the Hub overlay contract became the .dialog-scrim contract.
describe('Dialog scrim (MODAL-01: overlay contract)', () => {
  const scrim = cssRaw.match(/\.dialog-scrim\s*\{([^}]*)\}/)?.[1] ?? ''
  it('.dialog-scrim is fixed over the whole window', () => {
    expect(scrim).toContain('position: fixed')
    expect(scrim).toContain('inset: 0')
  })
  it('.dialog-scrim uses the scrim token and sits above the toolbar', () => {
    expect(scrim).toContain('var(--color-scrim)')
    expect(scrim).toContain('padding-top: 52px')
  })
  it('the old Hub overlay and its keyframes are gone', () => {
    expect(cssRaw).not.toContain('.hub-modal-overlay')
    expect(cssRaw).not.toContain('@keyframes hub-modal-overlay-in')
  })
})

describe('Dialog animation guard (MODAL-01: reduced-motion compliance)', () => {
  it('prefers-reduced-motion: reduce sets animation: none on .dialog and .dialog-scrim', () => {
    const m = cssRaw.match(/@media \(prefers-reduced-motion: reduce\) \{\s*\.dialog,\s*\.dialog-scrim\s*\{([^}]*)\}/)
    expect(m, 'reduced-motion .dialog rule not found').toBeTruthy()
    expect(m![1]).toContain('animation: none')
    expect(m![1]).toContain('transition: none')
  })
  it('opens with a 150ms ease-out fade plus translateY(-8px) and closes with a 100ms fade', () => {
    expect(cssRaw).toMatch(/@keyframes dialog-in\s*\{[^}]*translateY\(-8px\)/)
    expect(cssRaw).toMatch(/\.dialog\s*\{\s*animation:\s*dialog-in 150ms ease-out/)
    expect(cssRaw).toMatch(/\.dialog--exiting,\s*\.dialog-scrim--exiting\s*\{\s*animation:\s*dialog-out 100ms/)
  })
})
