import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 141-07 — Comp design-language adoption source gate.
// Pattern: mirrors style.hub.test.ts / style.contrast.test.ts (reads raw CSS text, asserts structure).
// These tests verify that the comp font/radii/type tokens are APPLIED at source level — not merely
// declared as custom properties. A future "token values reverted" or "font-family removed from surface"
// regression cannot silently pass this gate.
//
// User is colorblind: all color and font verification is done at source hex/var level, not by eye.
const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// ─── Font infrastructure: system fonts only, no bundled or CDN fonts ─────────

describe('Portty 02-03: no bundled or CDN fonts', () => {
  it('declares no @font-face rule', () => {
    expect(cssRaw).not.toMatch(/@font-face/)
  })

  it('references no ./assets/fonts/ path', () => {
    expect(cssRaw).not.toContain('./assets/fonts/')
  })

  it('no Google Fonts CDN URL present (fonts.googleapis.com)', () => {
    expect(cssRaw).not.toContain('fonts.googleapis.com')
  })

  it('no Google Static CDN URL present (fonts.gstatic.com)', () => {
    expect(cssRaw).not.toContain('fonts.gstatic.com')
  })
})

// ─── Font tokens: system fonts (Portty 02-03, LOOK-01) ───────────────────────

describe('Portty 02-03: font tokens use the system fonts', () => {
  it('--hub-font-ui is the system UI stack', () => {
    expect(cssRaw).toContain('--hub-font-ui: -apple-system, BlinkMacSystemFont, system-ui, sans-serif;')
  })

  it('--hub-font-mono is ui-monospace / SF Mono', () => {
    expect(cssRaw).toContain('--hub-font-mono: ui-monospace, "SF Mono", Menlo, monospace;')
  })
})

// ─── Colour aliases (Portty 02-03): --hub-* resolve to the --color-* tokens ──

describe('Portty 02-03: legacy --hub-* colour tokens alias the UI-SPEC tokens', () => {
  it('surfaces alias window / toolbar / popover', () => {
    expect(cssRaw).toContain('--hub-bg: var(--color-window);')
    expect(cssRaw).toContain('--hub-surface: var(--color-toolbar);')
    expect(cssRaw).toContain('--hub-surface-elevated: var(--color-popover);')
  })

  it('accent aliases the macOS-blue --color-accent', () => {
    expect(cssRaw).toContain('--hub-accent: var(--color-accent);')
  })

  it('#7C8CFF (rejected violet accent) is absent', () => {
    expect(cssRaw.toLowerCase()).not.toContain('#7c8cff')
  })

  it('semantic success / warning alias the UI-SPEC text tokens', () => {
    expect(cssRaw).toContain('--hub-success: var(--color-success-text);')
    expect(cssRaw).toContain('--hub-warning: var(--color-banner-warning-fg);')
  })
})

// ─── Font application: surface selectors must consume the tokens ─────────────

describe('Phase 141-07: ≥8 surface selectors apply var(--hub-font-ui) or var(--hub-font-mono)', () => {
  it('at least 8 font-family declarations use var(--hub-font-ui) or var(--hub-font-mono)', () => {
    const matches = cssRaw.match(/font-family:\s*var\(--hub-font-(ui|mono)\)/g) ?? []
    expect(matches.length).toBeGreaterThanOrEqual(8)
  })

  it('var(--hub-font-ui) is applied to at least one surface selector', () => {
    expect(cssRaw).toContain('font-family: var(--hub-font-ui)')
  })

  it('var(--hub-font-mono) is applied to at least one code/path/credential context', () => {
    expect(cssRaw).toContain('font-family: var(--hub-font-mono)')
  })

  it('no remaining system-ui literal stack on a chrome surface (no -apple-system or Segoe UI standalone stacks)', () => {
    // After restyle, no chrome selector should still have the old literal UI stack
    // (the token declarations themselves contain 'system-ui' as fallback — those are fine)
    // Verify that standalone font-family properties use the token, not the literal
    // Check that no 'font-family: system-ui' declarations (without var()) exist
    expect(cssRaw).not.toMatch(/font-family:\s*system-ui,\s*-apple-system/)
    expect(cssRaw).not.toMatch(/font-family:\s*-apple-system,\s*BlinkMacSystemFont/)
  })
})

// ─── Radii tokens: consumed by surface selectors ─────────────────────────────

describe('Phase 141-07: comp radii tokens applied to surfaces', () => {
  it('--hub-radius-pill is consumed by at least one selector (filter chips/pills)', () => {
    expect(cssRaw).toContain('border-radius: var(--hub-radius-pill)')
  })

  it('--hub-radius-md is consumed by at least one selector (cards/panels)', () => {
    expect(cssRaw).toContain('border-radius: var(--hub-radius-md)')
  })

  it('--hub-radius-sm is consumed by at least one selector (inputs/buttons)', () => {
    expect(cssRaw).toContain('border-radius: var(--hub-radius-sm)')
  })

  it('--hub-radius-lg is consumed by at least one selector (large modal containers)', () => {
    expect(cssRaw).toContain('border-radius: var(--hub-radius-lg)')
  })
})

// ─── Type scale: heading token consumed ──────────────────────────────────────

describe('Phase 141-07: comp type scale tokens applied', () => {
  it('--hub-font-size-heading is consumed by at least one heading selector', () => {
    expect(cssRaw).toContain('font-size: var(--hub-font-size-heading)')
  })

  it('--hub-font-size-sm is consumed by at least one small-text selector', () => {
    expect(cssRaw).toContain('font-size: var(--hub-font-size-sm)')
  })

  it('--hub-font-weight-emphasis is consumed by at least one emphasis selector', () => {
    expect(cssRaw).toContain('font-weight: var(--hub-font-weight-emphasis)')
  })
})

// ─── D-03 colorblind fences — MUST remain unchanged ──────────────────────────

describe('Phase 141-07: D-03 colorblind-safe fences — status-state', () => {
  it('agent-badge palette is gone (01-07 STRP-01 — no agent identity)', () => {
    expect(cssRaw).not.toContain('.tab__agent-badge')
  })

  it('.tab-status-bar__state--on is still present (D-03 fence — tmux state color)', () => {
    expect(cssRaw).toContain('.tab-status-bar__state--on')
  })

  it('.tab-status-bar__state--off is still present (D-03 fence)', () => {
    expect(cssRaw).toContain('.tab-status-bar__state--off')
  })

  it('.tab-status-bar__state--inactive is still present (D-03 fence)', () => {
    expect(cssRaw).toContain('.tab-status-bar__state--inactive')
  })
})

// ─── 01-04: New Session modal (01-UI-SPEC section 1) ────────────────────────

describe('01-04: New Session modal CSS', () => {
  const rule = (selector: string) => {
    const m = cssRaw.match(new RegExp(selector.replace(/[.-]/g, '\\$&') + '\\s*\\{([^}]*)\\}'))
    return m ? m[1] : ''
  }

  it('folder display truncates on one line and can shrink in the flex row', () => {
    const body = rule('.new-session-modal__folder-display')
    expect(body).toMatch(/overflow:\s*hidden/)
    expect(body).toMatch(/text-overflow:\s*ellipsis/)
    expect(body).toMatch(/white-space:\s*nowrap/)
    expect(body).toMatch(/min-width:\s*0/)
  })

  it('shell-line rules (--mono display, helper text) are gone', () => {
    expect(cssRaw).not.toMatch(/new-session-modal__(folder-display--mono|helper)/)
  })

  it('agent-button and args rule families are gone', () => {
    expect(cssRaw).not.toMatch(/new-session-modal__(agent|args)/)
  })
})
