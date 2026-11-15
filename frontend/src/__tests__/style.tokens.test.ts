/**
 * Portty Phase 2 — colour token source gate (THEM-01 / LOOK-01).
 *
 * The user is colour-blind, so colour correctness is proven here at the source
 * level, never by eye. style.css must declare every UI-SPEC colour token twice:
 * Light on :root and Dark in @media (prefers-color-scheme: dark) { :root { … } },
 * with the same names in both blocks. Legacy --hub-* tokens may only alias.
 *
 * Plan 02-12 made this the LOOK-01 proof: no literal colour anywhere in
 * style.css outside the token blocks, no inline colour in component TSX, no
 * custom scrollbars, and one focus ring on the --color-focus-ring token. It
 * replaced the RDS-02 migrated-surface hex scan and the older style.hub.test.ts.
 */

import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const srcDir = resolve(__dirname, '..')
const rawCss = readFileSync(resolve(srcDir, 'style.css'), 'utf8')

/** Replace block comments with spaces, keeping newlines and offsets. */
function stripBlockComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '))
}

interface Block { body: string; start: number; end: number }

/** First rule whose header matches `selectorMatch` (brace-balanced). start/end span header to closing brace. */
function findBlock(css: string, selectorMatch: RegExp): Block | null {
  const m = selectorMatch.exec(css)
  if (!m) return null
  let i = css.indexOf('{', m.index)
  if (i === -1) return null
  const open = i
  let depth = 0
  for (; i < css.length; i++) {
    if (css[i] === '{') depth++
    else if (css[i] === '}') {
      depth--
      if (depth === 0) return { body: css.slice(open + 1, i), start: m.index, end: i + 1 }
    }
  }
  return null
}

function extractBlockBody(css: string, selectorMatch: RegExp): string | null {
  return findBlock(css, selectorMatch)?.body ?? null
}

/** Innermost block of a nested header path (outermost first), with offsets into `css`. */
function findPath(css: string, path: RegExp[]): Block | null {
  let body = css
  let base = 0
  let found: Block | null = null
  for (const sel of path) {
    const b = findBlock(body, sel)
    if (!b) return null
    found = { body: b.body, start: base + b.start, end: base + b.end }
    base = found.end - 1 - b.body.length
    body = b.body
  }
  return found
}

/** name -> value for every custom property declared in a block body. */
function declarations(body: string): Map<string, string> {
  const out = new Map<string, string>()
  const re = /(--[a-zA-Z0-9-]+)\s*:\s*([^;]+);/g
  let m: RegExpExecArray | null
  while ((m = re.exec(body)) !== null) out.set(m[1], m[2].trim())
  return out
}

const cleanCss = stripBlockComments(rawCss)

/**
 * The only places a literal colour may appear: header paths (outermost first)
 * and the UI-SPEC column each block's values must match. Plan 02-13 may
 * register a data-appearance fallback block here.
 */
const TOKEN_BLOCK_SELECTORS: Array<{ name: string; path: RegExp[]; mode: 'light' | 'dark' }> = [
  { name: ':root (Light)', path: [/(^|\n):root\s*\{/], mode: 'light' },
  {
    name: '@media (prefers-color-scheme: dark) { :root }',
    path: [/@media\s*\(prefers-color-scheme:\s*dark\)\s*\{/, /:root\s*\{/],
    mode: 'dark',
  },
]

const tokenBlocks = TOKEN_BLOCK_SELECTORS.map((t) => ({ ...t, block: findPath(cleanCss, t.path) }))

/** style.css without comments and with every token block blanked out. */
const cssOutsideTokenBlocks = tokenBlocks.reduce(
  (css, { block }) =>
    block ? css.slice(0, block.start) + ' '.repeat(block.end - block.start) + css.slice(block.end) : css,
  cleanCss,
)

const isColourToken = (n: string) => /^--(color|shadow|opacity)-/.test(n)
const norm = (v: string) => v.replace(/\s+/g, '').toLowerCase()

// Copied from 02-UI-SPEC.md "Color": [token, Light, Dark].
const UI_SPEC: Array<[string, string, string]> = [
  ['--color-window', '#FFFFFF', '#1E1E1E'],
  ['--color-terminal-bg', '#FFFFFF', '#1C1C1E'],
  ['--color-sidebar', '#ECECEE', '#262628'],
  ['--color-toolbar', '#F6F6F7', '#2B2B2D'],
  ['--color-separator', '#D6D6DA', '#3A3A3C'],
  ['--color-text', '#1D1D1F', '#EBEBF0'],
  ['--color-text-secondary', '#636366', '#A1A1A6'],
  ['--color-accent', '#0063CC', '#4DA3FF'],
  ['--color-selection-fill', '#0063CC', '#0058D0'],
  ['--color-on-accent', '#FFFFFF', '#FFFFFF'],
  ['--color-selection-inactive', '#DCDCE0', '#464649'],
  ['--color-tab-active', '#FFFFFF', '#48484A'],
  ['--shadow-tab-active', '0 0.5px 2px rgba(0,0,0,0.18)', 'none'],
  ['--color-control-hover', 'rgba(0,0,0,0.05)', 'rgba(255,255,255,0.07)'],
  ['--color-chip', '#E5E5EA', '#323234'],
  ['--color-banner-warning-bg', '#FFF4D6', '#3A3020'],
  ['--color-banner-warning-fg', '#6B4E00', '#F2CF7C'],
  ['--color-banner-info-bg', '#E8F1FC', '#1F2A3A'],
  ['--color-banner-info-fg', '#0B4F9C', '#9CCBFF'],
  ['--color-popover', '#FFFFFF', '#2C2C2E'],
  ['--color-control-border', '#D1D1D6', '#48484A'],
  ['--color-field', '#F2F2F4', '#1C1C1E'],
  ['--color-button', '#FFFFFF', '#3A3A3C'],
  ['--color-switch-off', '#C7C7CC', '#5A5A5E'],
  ['--color-switch-knob', '#FFFFFF', '#FFFFFF'],
  ['--color-segment-bg', '#E5E5EA', '#1C1C1E'],
  ['--color-segment-selected', '#FFFFFF', '#636366'],
  ['--color-sheet-content', '#F5F5F7', '#1E1E1E'],
  ['--color-group', '#FFFFFF', '#2A2A2C'],
  ['--color-group-border', '#E0E0E4', '#3A3A3C'],
  ['--color-pane-tab-selected', '#E5E5EA', '#333335'],
  ['--color-status-on', '#30D158', '#30D158'],
  ['--color-status-off', '#8E8E93', '#8E8E93'],
  ['--color-success-text', '#1A7F37', '#30D158'],
  ['--color-error-text', '#C4161C', '#FF6961'],
  ['--color-destructive', '#D70015', '#D93025'],
  ['--color-focus-ring', 'rgba(0,99,204,0.5)', 'rgba(77,163,255,0.6)'],
  ['--color-scrim', 'rgba(0,0,0,0.2)', 'rgba(0,0,0,0.45)'],
  ['--shadow-switch-knob', '0 1px 2px rgba(0,0,0,0.3)', '0 1px 2px rgba(0,0,0,0.3)'],
  ['--shadow-popover', '0 8px 24px rgba(0,0,0,0.16)', '0 12px 32px rgba(0,0,0,0.5)'],
  ['--opacity-disabled', '0.45', '0.45'],
]

/** Every `property: value` declaration as [line, value]. */
function declarationValues(css: string): Array<[number, string]> {
  const out: Array<[number, string]> = []
  const re = /([a-zA-Z-]+)\s*:\s*([^;{}]+)(?=[;}])/g
  let m: RegExpExecArray | null
  while ((m = re.exec(css)) !== null) out.push([css.slice(0, m.index).split('\n').length, m[2].trim()])
  return out
}

const HEX_RE = /#[0-9a-fA-F]{3,8}\b/
const FUNC_COLOUR_RE = /\b(rgba?|hsla?)\(/
// transparent, currentColor, inherit, Canvas and CanvasText stay allowed.
const NAMED_COLOUR_RE = /\b(white|black|red|green|blue|gray|grey|silver|yellow|orange|purple|pink|navy|teal)\b/i

/** Every rule as [selector, body], including rules nested in @media. */
function rules(css: string): Array<[string, string]> {
  const out: Array<[string, string]> = []
  const re = /([^{}]+)\{([^{}]*)\}/g
  let m: RegExpExecArray | null
  while ((m = re.exec(css)) !== null) out.push([m[1].trim(), m[2]])
  return out
}

/** App.tsx and every non-test .tsx under components/. */
function componentSources(): Array<[string, string]> {
  const files = [join(srcDir, 'App.tsx')]
  for (const rel of readdirSync(join(srcDir, 'components'), { recursive: true }) as string[]) {
    if (rel.endsWith('.tsx') && !rel.includes('__tests__') && !/\.test\.tsx$/.test(rel)) {
      files.push(join(srcDir, 'components', rel))
    }
  }
  return files.map((f) => [f.slice(srcDir.length + 1), readFileSync(f, 'utf8')])
}

describe('LOOK-01 / THEM-01: two-block colour tokens', () => {
  it('finds every block in TOKEN_BLOCK_SELECTORS', () => {
    for (const t of tokenBlocks) expect(t.block, `${t.name} not found`).not.toBeNull()
  })

  it('(a) every token block declares exactly the same --color-* / --shadow-* / --opacity-* names', () => {
    const names = tokenBlocks.map((t) => [...declarations(t.block!.body).keys()].filter(isColourToken).sort())
    expect(names[0].length).toBeGreaterThan(0)
    for (let i = 1; i < names.length; i++) expect(names[i], tokenBlocks[i].name).toEqual(names[0])
  })

  it('(b) every UI-SPEC token has its Light or Dark value in each block', () => {
    const wrong: string[] = []
    for (const t of tokenBlocks) {
      const decl = declarations(t.block!.body)
      for (const [name, l, d] of UI_SPEC) {
        const want = t.mode === 'light' ? l : d
        if (norm(decl.get(name) ?? '') !== norm(want)) wrong.push(`${t.name} ${name}=${decl.get(name)} want ${want}`)
      }
    }
    expect(wrong).toEqual([])
  })

  it('(c) no --hub-* declaration carries a literal colour', () => {
    const offenders = cleanCss
      .split('\n')
      .filter((l) => /^\s*--hub-[a-z0-9-]+\s*:/.test(l) && /#[0-9a-fA-F]{3,8}\b|rgba?\(|hsla?\(/.test(l))
      .map((l) => l.trim())
    expect(offenders).toEqual([])
  })

  it('(d) style.css has no data-ui-theme', () => {
    expect(rawCss).not.toContain('data-ui-theme')
  })

  it('(e) :root declares color-scheme: light dark', () => {
    expect(tokenBlocks[0].block!.body).toMatch(/color-scheme:\s*light dark\s*;/)
  })

  it('(f) style.css has no @font-face (system fonts only)', () => {
    expect(rawCss).not.toContain('@font-face')
  })

  it('desktop app makes the sidebar token transparent for vibrancy (plan 02-01)', () => {
    expect(extractBlockBody(cleanCss, /(^|\n)html\.is-desktop\s*\{/)).toMatch(/--color-sidebar:\s*transparent/)
  })
})

describe('LOOK-01: no hardcoded colour (Portty 02-12)', () => {
  it('(g) outside the token blocks, style.css has no hex, rgb()/rgba()/hsl()/hsla() or named colour value', () => {
    const offenders: string[] = []
    for (const [line, value] of declarationValues(cssOutsideTokenBlocks)) {
      const bare = value.replace(/--[\w-]+/g, '')
      if (HEX_RE.test(bare) || FUNC_COLOUR_RE.test(bare) || NAMED_COLOUR_RE.test(bare)) {
        offenders.push(`style.css:${line} ${value}`)
      }
    }
    expect(offenders).toEqual([])
  })

  it('(h) no custom ::-webkit-scrollbar rule and no --hub-scrollbar token (native overlay scrollbars)', () => {
    expect(cleanCss).not.toContain('::-webkit-scrollbar')
    expect(cleanCss).not.toContain('--hub-scrollbar')
  })

  it('(i) one :focus-visible ring on --color-focus-ring, none on mouse focus, and no other ring colour', () => {
    expect(cleanCss).toMatch(
      /(^|\n):focus-visible\s*\{\s*outline:\s*3px solid var\(--color-focus-ring\);\s*outline-offset:\s*1px;\s*\}/,
    )
    expect(cleanCss).toMatch(/(^|\n):focus:not\(:focus-visible\)\s*\{\s*outline:\s*none;\s*\}/)
    const offenders = rules(cleanCss)
      .filter(([sel, body]) => sel.replace(':not(:focus-visible)', '').includes(':focus-visible') && /outline:/.test(body))
      .filter(([, body]) => !/outline:\s*3px solid var\(--color-focus-ring\)/.test(body))
      .map(([sel]) => sel)
    expect(offenders).toEqual([])
  })

  it('(j) no component or App.tsx inline style={{ … }} carries a hex or rgb()/hsl() colour', () => {
    const sources = componentSources()
    expect(sources.length).toBeGreaterThan(20)
    const offenders: string[] = []
    for (const [file, src] of sources) {
      for (const m of src.matchAll(/style=\{\{([\s\S]*?)\}\}/g)) {
        if (HEX_RE.test(m[1]) || FUNC_COLOUR_RE.test(m[1])) offenders.push(`${file}: style={{${m[1].trim()}}}`)
      }
    }
    expect(offenders).toEqual([])
  })
})

describe('Removed UI stays removed', () => {
  // Portty 02-07 (CHAT-01), moved from style.hub.test.ts in 02-12.
  it('has no .chat-*, .mention-popover* or chat-toggle rule (CHAT-01)', () => {
    expect(cleanCss).not.toMatch(/\.(chat-|mention-popover)|chat-toggle|terminal-chat-host/)
  })

  // Portty 01-07 (TEST-03 / STRP-01), moved from style.hub.test.ts in 02-12.
  it('has no agent spine or chip rules (data-agent, .hub-card__badge, .hub-card__chip--agent)', () => {
    expect(cleanCss).not.toMatch(/data-agent|\.hub-card__badge|\.hub-card__chip--agent/)
  })
})

describe('RDS-04 carry-over: reduced motion', () => {
  it('motion is governed by both prefers-reduced-motion arms', () => {
    expect(cleanCss).toMatch(/@media\s*\(prefers-reduced-motion:\s*no-preference\)/)
    expect(cleanCss).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)/)
  })

  it('dialog (incl. Share) animation is gated by prefers-reduced-motion (S-07, Portty 02-09)', () => {
    const noPref = extractBlockBody(
      cleanCss,
      /@media\s*\(prefers-reduced-motion:\s*no-preference\)\s*\{\s*\.dialog\s*\{/,
    )
    expect(noPref).toMatch(/animation:\s*dialog-in/)
    const reduce = extractBlockBody(cleanCss, /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{\s*\.dialog,/)
    expect(reduce).toMatch(/animation:\s*none/)
  })
})
