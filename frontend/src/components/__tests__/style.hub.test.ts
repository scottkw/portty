import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 131 — Hub CSS contract tests.
// Pattern: mirrors style.contrast.test.ts (reads raw CSS text, asserts structure).
// These tests verify Phase 131 design contract properties at the source level.
// The user is colorblind: hex constants are verified in source, never by eye.
const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// Phase 131 UAT follow-up — Open button (re-attach) must be styled, not bare.
// Guards against the CR-01 class of bug (TSX emits a class CSS never defines).
describe('Hub card Open button CSS contract (re-attach)', () => {
  it('defines .hub-card__row5 (actions row)', () => {
    expect(cssRaw).toContain('.hub-card__row5')
  })

  it('defines .hub-card__open button rule', () => {
    expect(cssRaw).toContain('.hub-card__open')
  })

  it('defines .hub-card__open:hover state', () => {
    expect(cssRaw).toContain('.hub-card__open:hover')
  })
})

// Phase 132 UAT fix — no Tailwind in this project, so Heroicon w-N/h-N classes are
// no-ops; Hub SVG icons must be sized explicitly or they render oversized.
describe('Hub icon sizing CSS contract (no Tailwind)', () => {
  it('sizes the group-sidebar collapse-toggle chevron svg', () => {
    expect(cssRaw).toContain('.hub__group-sidebar-toggle svg')
  })

  it('sizes the card drag-handle / menu-btn svg', () => {
    expect(cssRaw).toContain('.hub-card__drag-handle svg')
    expect(cssRaw).toContain('.hub-card__menu-btn svg')
  })
})

describe('Hub CSS tokens — dark theme (default :root)', () => {
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

  it('dark theme --hub-bg has correct hex #14151b (comp dark surface)', () => {
    expect(cssRaw).toContain('--hub-bg: #14151b')
  })

  it('dark theme --hub-accent has correct hex #7aa2f7', () => {
    expect(cssRaw).toContain('--hub-accent: #7aa2f7')
  })
})

describe('Hub CSS tokens — light theme ([data-ui-theme="light"])', () => {
  it('[data-ui-theme="light"] block is present', () => {
    expect(cssRaw).toContain('[data-ui-theme="light"]')
  })

  it('light theme --hub-accent is declared inside [data-ui-theme="light"] block', () => {
    // Target the token-defining block opener (selector + ` {`), not a later
    // light-theme override rule (Phase 142 POL-02 added one earlier in the file).
    const lightIdx = cssRaw.indexOf('[data-ui-theme="light"] {')
    expect(lightIdx).toBeGreaterThan(-1)
    // Find the closing brace of the light block
    const blockEnd = cssRaw.indexOf('}', lightIdx + 1)
    const lightBlock = cssRaw.slice(lightIdx, blockEnd)
    expect(lightBlock).toContain('--hub-accent')
  })

  it('light theme --hub-destructive is declared inside [data-ui-theme="light"] block', () => {
    const lightIdx = cssRaw.indexOf('[data-ui-theme="light"] {')
    expect(lightIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', lightIdx + 1)
    const lightBlock = cssRaw.slice(lightIdx, blockEnd)
    expect(lightBlock).toContain('--hub-destructive')
  })

  it('HUB-04 WCAG AA comment for --hub-accent in light theme (#3d6fe8 on #ffffff = 4.5:1)', () => {
    // Source-level verification: hex constant + contrast comment must be present
    expect(cssRaw).toContain('#3d6fe8')
    expect(cssRaw).toContain('4.5:1')
  })

  it('HUB-04 WCAG AA comment for --hub-destructive in light theme (#c0394f on #ffffff = 4.7:1)', () => {
    expect(cssRaw).toContain('#c0394f')
    expect(cssRaw).toContain('4.7:1')
  })

  it('light theme --hub-bg has correct hex #f5f5f7', () => {
    expect(cssRaw).toContain('--hub-bg: #f5f5f7')
  })
})

describe('Hub layout — responsive grid (GRID-01)', () => {
  it('grid uses repeat(auto-fill, minmax(240px, 1fr)) (GRID-01)', () => {
    expect(cssRaw).toContain('repeat(auto-fill, minmax(240px, 1fr))')
  })

  it('.hub__card-row declares display: grid', () => {
    const hubCardRowIdx = cssRaw.indexOf('.hub__card-row')
    expect(hubCardRowIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', hubCardRowIdx)
    const block = cssRaw.slice(hubCardRowIdx, blockEnd)
    expect(block).toContain('display: grid')
  })

  it('.hub__card-row has gap: 8px', () => {
    const hubCardRowIdx = cssRaw.indexOf('.hub__card-row')
    expect(hubCardRowIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', hubCardRowIdx)
    const block = cssRaw.slice(hubCardRowIdx, blockEnd)
    expect(block).toContain('gap: 8px')
  })

  it('.hub__card-row has max-width: 1440px', () => {
    const hubCardRowIdx = cssRaw.indexOf('.hub__card-row')
    expect(hubCardRowIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', hubCardRowIdx)
    const block = cssRaw.slice(hubCardRowIdx, blockEnd)
    expect(block).toContain('max-width: 1440px')
  })

  it('.hub-card has min-width: 240px', () => {
    expect(cssRaw).toContain('min-width: 240px')
  })

  it('.hub-card has max-width: 360px', () => {
    expect(cssRaw).toContain('max-width: 360px')
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

describe('Hub sidebar active state (Pitfall 8 / HUB-01)', () => {
  it('.sidebar__item--active rule is present', () => {
    expect(cssRaw).toContain('.sidebar__item--active')
  })

  it('.sidebar__item--active uses var(--hub-accent) for color', () => {
    const activeIdx = cssRaw.indexOf('.sidebar__item--active')
    expect(activeIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', activeIdx)
    const block = cssRaw.slice(activeIdx, blockEnd)
    expect(block).toContain('var(--hub-accent')
  })
})

describe('Hub group header (mirrors .remote-panel__peer-header)', () => {
  it('.hub__group-header is declared', () => {
    expect(cssRaw).toContain('.hub__group-header')
  })

  it('.hub__group-header has font-size: 11px', () => {
    const headerIdx = cssRaw.indexOf('.hub__group-header')
    expect(headerIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', headerIdx)
    const block = cssRaw.slice(headerIdx, blockEnd)
    expect(block).toContain('font-size: 11px')
  })

  it('.hub__group-header has font-weight: 600', () => {
    const headerIdx = cssRaw.indexOf('.hub__group-header')
    expect(headerIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', headerIdx)
    const block = cssRaw.slice(headerIdx, blockEnd)
    expect(block).toContain('font-weight: 600')
  })

  it('.hub__group-header has text-transform: uppercase', () => {
    const headerIdx = cssRaw.indexOf('.hub__group-header')
    expect(headerIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', headerIdx)
    const block = cssRaw.slice(headerIdx, blockEnd)
    expect(block).toContain('text-transform: uppercase')
  })

  it('.hub__group-header has letter-spacing: 0.08em', () => {
    const headerIdx = cssRaw.indexOf('.hub__group-header')
    expect(headerIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', headerIdx)
    const block = cssRaw.slice(headerIdx, blockEnd)
    expect(block).toContain('letter-spacing: 0.08em')
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
  it('A11Y-02: .hub-card:focus-visible rule is present (upgraded from :focus)', () => {
    expect(cssRaw).toContain('.hub-card:focus-visible')
  })

  it('A11Y-02: .hub-card:focus (bare, without -visible) outline rule is NOT present (Pitfall 2 guard)', () => {
    // The only :focus without -visible on hub-card must be :focus-within (structural, not outline)
    // Assert no standalone .hub-card:focus { outline rule exists
    expect(cssRaw).not.toMatch(/\.hub-card:focus\s*\{[^}]*outline/)
  })

  it('STRP-01: the status filter pill rules are gone', () => {
    expect(cssRaw).not.toContain('.hub-filter__pill')
    expect(cssRaw).not.toContain('hub-needs-input-badge')
  })

  it('A11Y-02: .hub-card__open:focus-visible rule is present', () => {
    expect(cssRaw).toContain('.hub-card__open:focus-visible')
  })

  it('A11Y-02: .hub-modal__close:focus-visible rule is present', () => {
    expect(cssRaw).toContain('.hub-modal__close:focus-visible')
  })

  it('A11Y-02: .hub__group-sidebar-item:focus-visible rule is present', () => {
    expect(cssRaw).toContain('.hub__group-sidebar-item:focus-visible')
  })

  it('A11Y-02: all new :focus-visible rules use var(--hub-accent) token (colorblind constraint)', () => {
    // Extract the GAP-135-A grouped rule block — find it by locating .hub-filter__new-session:focus-visible
    const pillIdx = cssRaw.indexOf('.hub-filter__new-session:focus-visible')
    expect(pillIdx).toBeGreaterThan(-1)
    // Find the closing brace of the group rule
    const blockEnd = cssRaw.indexOf('}', pillIdx)
    const block = cssRaw.slice(pillIdx, blockEnd)
    expect(block).toContain('var(--hub-accent)')
  })

  it('A11Y-02: .hub-filter__search:focus is still present and unchanged (inputs keep :focus per WCAG 2.4.7)', () => {
    expect(cssRaw).toContain('.hub-filter__search:focus')
  })
})

describe('Phase 135 — GAP-135-E/F: prefers-reduced-motion reduce blocks', () => {
  it('GAP-135-F: prefers-reduced-motion: reduce block targets .hub-card with transition: none', () => {
    // Must be inside a reduce block — find a reduce block containing .hub-card { transition: none
    expect(cssRaw).toMatch(/prefers-reduced-motion:\s*reduce[^@]*\.hub-card\s*\{\s*transition:\s*none/)
  })

  it('GAP-135-F: card-hover reduce block appears AFTER the no-preference .hub-card 400ms transition override', () => {
    // The no-preference block sets .hub-card transition to 400ms ease (ATTN-03)
    const noPrefIdx = cssRaw.indexOf('border-color 400ms ease, box-shadow 400ms ease, background 100ms ease')
    expect(noPrefIdx).toBeGreaterThan(-1)

    // GAP-135-F card hover reduce block must also come after no-preference
    const cardTransitionNoneIdx = cssRaw.indexOf('.hub-card {\n    transition: none')
    expect(cardTransitionNoneIdx).toBeGreaterThan(noPrefIdx)
  })

  it('GAP-135-E/F: existing no-preference 400ms hub-card transition override is still present (not overwritten)', () => {
    expect(cssRaw).toContain('border-color 400ms ease, box-shadow 400ms ease, background 100ms ease')
  })
})

// Phase 133 CSS contract (IN-01)
describe('Hub Phase 133 CSS contract', () => {
  it('IN-01: .hub-card has position: relative (anchor for drag-handle/menu-btn)', () => {
    const cardIdx = cssRaw.indexOf('.hub-card {')
    expect(cardIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', cardIdx)
    const block = cssRaw.slice(cardIdx, blockEnd)
    expect(block).toContain('position: relative')
  })
})

// Phase 138 / CARD-03: Connection indicator CSS — remote card connected/available states.
// These assertions are RED until Plan 03 adds the CSS classes to style.css.
// COLORBLIND-SAFE: icon shape + text carry the state; color is reinforcement only.
describe('CARD-03: Connection indicator CSS (hub-card__conn)', () => {
  it('defines .hub-card__conn class', () => {
    expect(cssRaw).toContain('.hub-card__conn')
  })
  it('defines .hub-card__conn--connected modifier', () => {
    expect(cssRaw).toContain('.hub-card__conn--connected')
  })
  it('defines .hub-card__conn-icon class', () => {
    expect(cssRaw).toContain('.hub-card__conn-icon')
  })
})

// Phase 138 / CARD-04: Preserved grid CSS — anti-regression against accidental deletion.
// These must PASS against current style.css (they already exist).
describe('CARD-04: Preserved grid CSS (anti-regression)', () => {
  it('preserves .hub__card-row grid definition', () => {
    expect(cssRaw).toContain('.hub__card-row')
  })
  it('preserves hub-card min-width 240px constraint', () => {
    expect(cssRaw).toContain('240px')
  })
})

// Phase 138 / CARD-04: Kill menu item CSS — destructive action styling.
// These assertions are RED until Plan 03 adds the CSS classes to style.css.
describe('CARD-04: Kill menu item CSS', () => {
  it('defines .hub-card__menu-item--destructive modifier', () => {
    expect(cssRaw).toContain('.hub-card__menu-item--destructive')
  })
  it('destructive color uses --hub-destructive custom property', () => {
    expect(cssRaw).toContain('--hub-destructive')
  })
})

// ============================================================
// Phase 142 comp-fidelity CSS tokens (GAP-04 anti-regression)
// Tokens verified against frontend/src/style.css on 2026-06-21.
// COLORBLIND-SAFE convention: hex literals are asserted at source, not by eye.
// ============================================================

describe('Phase 142 comp-fidelity CSS tokens (GAP-04 anti-regression)', () => {
  // (a) .hub-card block contains border-radius: 16px
  it('GAP-04: .hub-card block contains border-radius: 16px (pronounced rounded corners)', () => {
    // Find the .hub-card { rule block and assert border-radius is present
    const hubCardIdx = cssRaw.indexOf('.hub-card {')
    expect(hubCardIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', hubCardIdx)
    const block = cssRaw.slice(hubCardIdx, blockEnd)
    expect(block).toContain('border-radius: 16px')
  })

  // (b)/(c) session-type spine and badge chip removed in 01-07 (STRP-01 — no agent identity)
  it('01-07: no data-agent spine/chip rules and no .hub-card__badge / .hub-card__chip--agent remain', () => {
    expect(cssRaw).not.toContain('data-agent')
    expect(cssRaw).not.toContain('.hub-card__badge')
    expect(cssRaw).not.toContain('.hub-card__chip--agent')
  })

  // (d) mini-preview window: height: 150px (note: height, not min-height — per interfaces block)
  it('GAP-04: .hub-card__preview block declares height: 150px (not min-height)', () => {
    const previewIdx = cssRaw.indexOf('.hub-card__preview {')
    expect(previewIdx).toBeGreaterThan(-1)
    const blockEnd = cssRaw.indexOf('}', previewIdx)
    const block = cssRaw.slice(previewIdx, blockEnd)
    expect(block).toContain('height: 150px')
    // Negative: must not be min-height only (the interface block specifies `height`)
    expect(block).not.toMatch(/min-height:\s*150px/)
  })
})

// Phase 159 (WEBCHAT-06) — chat author header layout. A long author name (e.g. a
// tailnet hostname used as the fallback identity before an alias is set) plus the
// raw nodekey must truncate with an ellipsis on one line, not wrap char-by-char in
// the narrow chat panel. Source-gate: assert the truncation properties on the
// alias + tailnet-id rules.
describe('Chat author header truncation (WEBCHAT-06)', () => {
  function ruleBody(selector: string): string {
    const re = new RegExp(selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\{([^}]+)\\}')
    const m = cssRaw.match(re)
    expect(m, `rule ${selector} must exist`).not.toBeNull()
    return m![1]
  }

  it('.chat-msg__alias truncates with ellipsis on one line', () => {
    const body = ruleBody('.chat-msg__alias')
    expect(body).toContain('text-overflow: ellipsis')
    expect(body).toContain('white-space: nowrap')
    expect(body).toContain('overflow: hidden')
    expect(body).toContain('min-width: 0')
  })

  it('.chat-msg__tailnet-id truncates and gives up space first (high flex-shrink)', () => {
    const body = ruleBody('.chat-msg__tailnet-id')
    expect(body).toContain('text-overflow: ellipsis')
    expect(body).toContain('white-space: nowrap')
    expect(body).toContain('flex-shrink: 1000')
  })

  it('.chat-msg__header allows children to shrink (min-width: 0)', () => {
    expect(ruleBody('.chat-msg__header')).toContain('min-width: 0')
  })
})
