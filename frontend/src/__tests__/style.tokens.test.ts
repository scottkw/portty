/**
 * Portty Phase 2 plan 02-03 — colour token source gate (THEM-01 / LOOK-01).
 *
 * The user is colour-blind, so colour correctness is proven here at the source
 * level, never by eye. style.css must declare every UI-SPEC colour token twice:
 * Light on :root and Dark in @media (prefers-color-scheme: dark) { :root { … } },
 * with the same names in both blocks. Legacy --hub-* tokens may only alias.
 * Replaces themeTokens.test.ts (the [data-ui-theme="light"] parity gate); its
 * RDS-02 migrated-surface hex scan and reduced-motion checks are carried over.
 */

import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const rawCss = readFileSync(resolve(__dirname, '..', 'style.css'), 'utf8')

const HEX_RE = /#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?\b/g

/** Replace block comments with spaces, keeping newlines so line numbers hold. */
function stripBlockComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '))
}

/** Body of the first rule whose header matches `selectorMatch` (brace-balanced). */
function extractBlockBody(css: string, selectorMatch: RegExp): string | null {
  const m = selectorMatch.exec(css)
  if (!m) return null
  let i = css.indexOf('{', m.index)
  if (i === -1) return null
  let depth = 0
  const start = i + 1
  for (; i < css.length; i++) {
    if (css[i] === '{') depth++
    else if (css[i] === '}') {
      depth--
      if (depth === 0) return css.slice(start, i)
    }
  }
  return null
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
const lightBody = extractBlockBody(cleanCss, /(^|\n):root\s*\{/)
const darkMedia = extractBlockBody(cleanCss, /@media\s*\(prefers-color-scheme:\s*dark\)\s*\{/)
const darkBody = darkMedia === null ? null : extractBlockBody(darkMedia, /:root\s*\{/)

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

describe('LOOK-01 / THEM-01: two-block colour tokens', () => {
  it('finds the Light :root block and the Dark prefers-color-scheme :root block', () => {
    expect(lightBody, ':root block not found').not.toBeNull()
    expect(darkBody, '@media (prefers-color-scheme: dark) { :root { … } } not found').not.toBeNull()
  })

  it('(a) both blocks declare exactly the same --color-* / --shadow-* / --opacity-* names', () => {
    const light = [...declarations(lightBody!).keys()].filter(isColourToken).sort()
    const dark = [...declarations(darkBody!).keys()].filter(isColourToken).sort()
    expect(light.length).toBeGreaterThan(0)
    expect(dark).toEqual(light)
  })

  it('(b) every UI-SPEC token has its Light and Dark value', () => {
    const light = declarations(lightBody!)
    const dark = declarations(darkBody!)
    const wrong: string[] = []
    for (const [name, l, d] of UI_SPEC) {
      if (norm(light.get(name) ?? '') !== norm(l)) wrong.push(`${name} light=${light.get(name)} want ${l}`)
      if (norm(dark.get(name) ?? '') !== norm(d)) wrong.push(`${name} dark=${dark.get(name)} want ${d}`)
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
    expect(lightBody!).toMatch(/color-scheme:\s*light dark\s*;/)
  })

  it('(f) style.css has no @font-face (system fonts only)', () => {
    expect(rawCss).not.toContain('@font-face')
  })

  it('desktop app makes the sidebar token transparent for vibrancy (plan 02-01)', () => {
    expect(extractBlockBody(cleanCss, /(^|\n)html\.is-desktop\s*\{/)).toMatch(/--color-sidebar:\s*transparent/)
  })
})

// Carried over from themeTokens.test.ts (Phase 141 RDS-02): surfaces that were
// migrated to tokens must not regain raw hex. Token blocks sit under :root, which
// is out of scope, so they are never flagged.
const MIGRATED_SURFACE_SELECTORS = [
  '.sidebar', '.welcome-tab', '.tab-bar', '.terminal-container',
  '.hub-share-modal',
  // Portty 02-11: the Settings sheet, its rows, the Switch and the Share dialog switch rows.
  '.settings-', '.switch', '.share-toggle-row', '.ts-status',
  // Portty 02-08: toolbar, tab pills, context menu, empty state, End Session note.
  '.tab__', '.tab-list', '.toolbar-', '.context-menu', '.empty-state', '.end-session',
  // Portty 02-09: every dialog and the Share dialog's content.
  '.dialog', '.button', '.new-session-', '.quit-modal', '.remote-join-modal', '.share-seg',
  '.hub-funnel-risk-panel', '.share-field', '.session-share-panel', '.daemon-panel__btn', '.link-confirm-popover',
]

describe('RDS-02 / RDS-04 carry-over', () => {
  it('migrated surfaces contain no raw hex', () => {
    const lines = cleanCss.split('\n')
    const nearestSelector = (i: number) => {
      for (let j = i; j >= 0; j--) {
        if (lines[j].includes('{') && !/^\s*(--|[a-z-]+\s*:)/.test(lines[j])) return lines[j].trim()
      }
      return '<file scope>'
    }
    const offenders: string[] = []
    lines.forEach((line, i) => {
      const hex = line.match(HEX_RE)
      if (!hex) return
      const sel = nearestSelector(i)
      if (!MIGRATED_SURFACE_SELECTORS.some((s) => sel.includes(s))) return
      offenders.push(`style.css:${i + 1} ${sel} -> ${hex.join(', ')}`)
    })
    expect(offenders).toEqual([])
  })

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
