import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 14-11 (MOB-01): source gate for the phone chrome (14-UI-SPEC "Phone Layout"). Each phone rule
// must sit inside a phone media query, so the desktop browser view and the Wails app are untouched.
const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// The body of every @media block whose query is exactly `query` (no nesting inside these blocks).
function blocks(query: string): string[] {
  const out: string[] = []
  const head = `@media ${query} {`
  for (let i = css.indexOf(head); i !== -1; i = css.indexOf(head, i + 1)) {
    let depth = 1
    let j = i + head.length
    while (depth > 0 && j < css.length) {
      if (css[j] === '{') depth++
      else if (css[j] === '}') depth--
      j++
    }
    out.push(css.slice(i + head.length, j - 1))
  }
  return out
}
const phone = blocks('(pointer: coarse) and (hover: none)').join('\n')
const coarse = blocks('(pointer: coarse)').join('\n')
const rule = (within: string, selector: string): string => {
  const i = within.indexOf(`${selector} {`)
  expect(i, `${selector} rule is inside the media query`).toBeGreaterThan(-1)
  return within.slice(i, within.indexOf('}', i))
}

describe('Phase 14-11: phone chrome CSS', () => {
  it('the single-tab strip is hidden on a phone', () => {
    expect(rule(phone, '.tab-bar--single')).toContain('display: none')
  })
  it('the tab strip sides clear the safe areas, never under 12px', () => {
    const r = rule(phone, '.tab-bar')
    expect(r).toContain('padding-left: max(12px, env(safe-area-inset-left))')
    expect(r).toContain('padding-right: max(12px, env(safe-area-inset-right))')
  })
  it('terminal padding is 4px 8px on a phone', () => {
    expect(rule(phone, '.terminal-session-container')).toContain('padding: 4px 8px')
  })
  it('the root pads its top by the safe area and follows visualViewport', () => {
    const r = rule(phone, '.app')
    expect(r).toContain('padding-top: env(safe-area-inset-top)')
    expect(r).toContain('top: var(--vv-top, 0px)')
    expect(css).toMatch(/\.app \{[^}]*height: var\(--vv-height, 100%\)/)
  })
  it('html allows only panning, so the page cannot zoom', () => {
    expect(rule(phone, 'html')).toContain('touch-action: pan-x pan-y')
  })
  it('the key bar drops its bottom inset with the keyboard up', () => {
    expect(rule(phone, '.keybar--kb-up')).toContain('padding-bottom: 0')
  })
  it('form controls, including the xterm helper textarea, are 16px on a coarse pointer', () => {
    const r = rule(coarse, 'input,\n  textarea,\n  select,\n  .xterm-helper-textarea')
    expect(r).toContain('font-size: 16px')
  })
  it('the desktop base rules are unchanged: tab strip padding 0 12px and terminal padding 12px 16px', () => {
    expect(css).toMatch(/\n\.tab-bar \{[^}]*padding: 0 12px;/)
    expect(css).toMatch(/\n\.terminal-session-container \{\s*padding: 12px 16px;/)
  })
})
