import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 134 — Hub modal CSS contract tests.
// Pattern: mirrors style.hub.test.ts (reads raw CSS text, block-finder helper for structural assertions).
// These tests are intentionally RED until modal CSS is implemented in a later plan.
const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

describe('Hub modal overlay (MODAL-01: overlay z-index contract)', () => {
  it('.hub-modal-overlay block contains position: fixed', () => {
    const idx = cssRaw.indexOf('.hub-modal-overlay')
    expect(idx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', idx)
    const block = cssRaw.slice(idx, blockEnd)
    expect(block).toContain('position: fixed')
  })

  it('.hub-modal-overlay block contains inset: 0', () => {
    const idx = cssRaw.indexOf('.hub-modal-overlay')
    expect(idx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', idx)
    const block = cssRaw.slice(idx, blockEnd)
    expect(block).toContain('inset: 0')
  })

  it('.hub-modal-overlay block contains z-index: 200', () => {
    const idx = cssRaw.indexOf('.hub-modal-overlay')
    expect(idx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', idx)
    const block = cssRaw.slice(idx, blockEnd)
    expect(block).toContain('z-index: 200')
  })
})

describe('Hub modal keyframes (animation declarations)', () => {

  it('@keyframes hub-modal-overlay-in is declared', () => {
    expect(cssRaw).toContain('@keyframes hub-modal-overlay-in')
  })

  it('@keyframes hub-modal-overlay-out is declared', () => {
    expect(cssRaw).toContain('@keyframes hub-modal-overlay-out')
  })
})

describe('Hub modal animation guard (MODAL-01: reduced-motion compliance)', () => {

  it('prefers-reduced-motion: reduce block sets animation: none on .hub-modal-overlay', () => {
    // Locate the `.hub-modal-overlay { ... }` rule directly (not
    // `.hub-modal-overlay--*`) and assert it neutralizes the entering
    // animation. (Portty 02-05: the bare `.hub-modal` panel went with HubModal.) Targeting the rule by its selector is robust against unrelated
    // reduced-motion blocks appearing later in the stylesheet — a brittle
    // `lastIndexOf('prefers-reduced-motion: reduce')` scan matched a trailing
    // later, unrelated `transition: none` block instead.
    const ruleMatch = cssRaw.match(/\.hub-modal-overlay\s*\{[^}]*\}/g) || []
    const reducedMotionRule = ruleMatch.find((r) => r.includes('animation: none'))
    expect(reducedMotionRule, '.hub-modal-overlay rule with animation: none not found').toBeTruthy()
    expect(reducedMotionRule).toContain('animation: none')
  })
})

// The briefing view (send button, tail, respond input) and the status/attention
// header icons were removed with HubBriefingModal (CONTEXT Amendment 2).
describe('Hub modal has no briefing or status CSS (TERM-05)', () => {
  it('briefing and status rules are gone', () => {
    for (const sel of [
      '.hub-modal__send-btn', '.hub-modal__tail', '.hub-modal__respond', '.hub-modal__body--briefing',
      '.hub-modal__status-icon', '.hub-modal__attn-icon', '.hub-modal__attn-badge',
    ]) {
      expect(cssRaw, sel).not.toContain(sel)
    }
  })
})

describe('Hub modal header icons (GAP-134-B: explicit icon sizing — Heroicons have no intrinsic size)', () => {
  // Without explicit width/height the header icons balloon to fill the strip (no Tailwind
  // in this project; w-N/h-N are no-ops). Each header icon class must declare a size.
  const sizedSelectors = [
    '.hub-modal__close svg',
  ]
  for (const sel of sizedSelectors) {
    it(`${sel} declares explicit width and height`, () => {
      const idx = cssRaw.indexOf(sel)
      expect(idx).toBeGreaterThan(-1)
      const blockEnd = cssRaw.indexOf('}', idx)
      const block = cssRaw.slice(idx, blockEnd)
      expect(block).toMatch(/width:\s*\d+px/)
      expect(block).toMatch(/height:\s*\d+px/)
    })
  }
})
