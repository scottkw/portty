// Phase 147-01: HelpTab source-gate test stubs (RED until Plans 02/03).
//
// These tests assert that App.tsx declares HELP_TAB/handleOpenHelp and that
// style.css declares the --hub-search-highlight-bg token in both :root and
// [data-ui-theme="light"]. They WILL fail until Plans 02/03 add those
// implementations — this is the intended RED state for Wave 0.

import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

const cssRaw = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// Portty 02-04: App no longer renders a Help tab (Help opens the docs site);
// the App.tsx HELP_TAB gates were dropped. HelpTab itself goes in plan 02-05.

// ============================================================
// style.css source gates — --hub-search-highlight-bg token (RED until Plan 03)
// ============================================================

describe('HelpTab source gate: --hub-search-highlight-bg CSS token (Phase 147)', () => {
  it('style.css declares --hub-search-highlight-bg in :root', () => {
    // Token must be present in the :root block
    expect(cssRaw).toContain('--hub-search-highlight-bg')
  })

  it('--hub-search-highlight-bg aliases a two-mode token (Portty 02-03)', () => {
    expect(cssRaw).toMatch(/--hub-search-highlight-bg:\s*var\(--color-[a-z-]+\)/)
  })
})
