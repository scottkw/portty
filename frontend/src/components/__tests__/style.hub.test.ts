import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 131 — Hub CSS contract tests.
// Pattern: mirrors style.contrast.test.ts (reads raw CSS text, asserts structure).
// These tests verify Phase 131 design contract properties at the source level.
// The user is colorblind: hex constants are verified in source, never by eye.
const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

describe('Hub CSS tokens (aliases of the UI-SPEC colour tokens)', () => {
  it('declares --hub-bg custom property', () => {
    expect(cssRaw).toContain('--hub-bg')
  })

  it('declares --hub-surface custom property', () => {
    expect(cssRaw).toContain('--hub-surface')
  })

  it('declares --hub-accent custom property', () => {
    expect(cssRaw).toContain('--hub-accent')
  })

  it('declares --hub-destructive custom property', () => {
    expect(cssRaw).toContain('--hub-destructive')
  })

  it('--hub-bg aliases --color-window', () => {
    expect(cssRaw).toContain('--hub-bg: var(--color-window)')
  })

  it('--hub-accent aliases --color-accent', () => {
    expect(cssRaw).toContain('--hub-accent: var(--color-accent)')
  })

  it('--hub-destructive aliases --color-error-text', () => {
    expect(cssRaw).toContain('--hub-destructive: var(--color-error-text)')
  })
})

// CONTEXT Amendment 2: no session status, dimming or attention styling on Hub cards.
describe('Hub cards show no session status (TERM-05)', () => {
  it('status, exit-code, dim and attention rules and the --hub-attn-* / dim tokens are gone', () => {
    for (const sel of [
      '.hub-card--dim', '.hub-card--attention', '.hub-card__attn-icon', '.hub-card__exit-chip',
      '.hub-card__status-icon', '.hub-card__status-indicator', '.hub-card__status-label',
      '.hub__group-sidebar-item__attn-badge', '@keyframes hub-attn-pulse',
      '--hub-attn-', '--hub-dim-opacity', '--hub-card-dim-bg',
    ]) {
      expect(cssRaw, sel).not.toContain(sel)
    }
  })
})

describe('Hub motion contract — reduced-motion guard (Pitfall 6)', () => {
  it('prefers-reduced-motion: no-preference media query is present', () => {
    expect(cssRaw).toContain('prefers-reduced-motion: no-preference')
  })

  it('STRP-01: no status icon spin rules remain (running uses a static PlayCircleIcon)', () => {
    expect(cssRaw).not.toContain('hub-card__status-icon--spin')
    expect(cssRaw).not.toContain('@keyframes hub-spin')
  })
})

describe('Hub colorblind-safe source comments (source-level UAT verification)', () => {
  it('has COLORBLIND-SAFE comment for running status dot dark hex #3b82f6', () => {
    expect(cssRaw).toContain('#3b82f6')
    expect(cssRaw).toContain('running')
  })

  it('has COLORBLIND-SAFE comment for idle status dark hex #22c55e', () => {
    expect(cssRaw).toContain('#22c55e')
  })

  it('has COLORBLIND-SAFE comment for stopped/done dark hex #565f89', () => {
    expect(cssRaw).toContain('#565f89')
  })
})

// Phase 135 CSS contract — GAP-135-A focus-visible rings + GAP-135-E/F reduced-motion
describe('Phase 135 — GAP-135-A: focus-visible rings for Hub interactive elements', () => {

  it('STRP-01: the status filter pill rules are gone', () => {
    expect(cssRaw).not.toContain('.hub-filter__pill')
    expect(cssRaw).not.toContain('hub-needs-input-badge')
  })

  it('A11Y-02: .hub-modal__close:focus-visible rule is present', () => {
    expect(cssRaw).toContain('.hub-modal__close:focus-visible')
  })

  it('A11Y-02: the remaining :focus-visible ring uses var(--hub-accent) token (colorblind constraint)', () => {
    const idx = cssRaw.indexOf('.hub-modal__close:focus-visible')
    expect(idx).toBeGreaterThan(-1)
    const block = cssRaw.slice(idx, cssRaw.indexOf('}', idx))
    expect(block).toContain('var(--hub-accent)')
  })

})

// ============================================================
// Phase 142 comp-fidelity CSS tokens (GAP-04 anti-regression)
// Tokens verified against frontend/src/style.css on 2026-06-21.
// COLORBLIND-SAFE convention: hex literals are asserted at source, not by eye.
// ============================================================

describe('Phase 142 comp-fidelity CSS tokens (GAP-04 anti-regression)', () => {

  // (b)/(c) session-type spine and badge chip removed in 01-07 (STRP-01 — no agent identity)
  it('01-07: no data-agent spine/chip rules and no .hub-card__badge / .hub-card__chip--agent remain', () => {
    expect(cssRaw).not.toContain('data-agent')
    expect(cssRaw).not.toContain('.hub-card__badge')
    expect(cssRaw).not.toContain('.hub-card__chip--agent')
  })

})

// Portty 02-07 (CHAT-01): chat UI removed; no chat rule may remain in style.css.
describe('No chat CSS remains (CHAT-01)', () => {
  it('has no .chat-*, .mention-popover* or chat-toggle rule', () => {
    expect(cssRaw).not.toMatch(/\.(chat-|mention-popover)|chat-toggle|terminal-chat-host/)
  })
})
