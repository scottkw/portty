import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Phase 14-10 (MOB-01): source gate for the phone key bar's CSS (14-UI-SPEC "Key Bar", "Color").
// Reads raw CSS text, like style.redesign.test.ts. Tokens only: a literal colour here would break
// light/dark following the system.
const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')
const rule = (selector: string): string => {
  const i = css.indexOf(`\n${selector} {`)
  expect(i, `rule ${selector} exists`).toBeGreaterThan(-1)
  return css.slice(i, css.indexOf('}', i))
}

describe('Phase 14-10: .keybar CSS', () => {
  it('the bar is a 44px toolbar row with 4px top and bottom padding with safe-area sides and a separator top edge', () => {
    const r = rule('.keybar')
    expect(r).toContain('height: 44px')
    expect(r).toContain('padding: 4px env(safe-area-inset-right) calc(4px + env(safe-area-inset-bottom)) env(safe-area-inset-left)')
    expect(r).toContain('background: var(--color-toolbar)')
    expect(r).toContain('border-top: 1px solid var(--color-separator)')
    expect(r).toContain('flex-shrink: 0')
    expect(r).toContain('overflow-x: auto')
    expect(r).toContain('user-select: none')
    expect(r).toContain('-webkit-touch-callout: none')
  })

  it('a key is a 44px hit box that shrinks no smaller than 44px wide', () => {
    const r = rule('.keybar__key')
    expect(r).toContain('flex: 1 1 0')
    expect(r).toContain('min-width: 44px')
    expect(r).toContain('height: 44px')
    expect(r).toContain('font-size: 13px')
    expect(r).toContain('font-weight: 600')
  })

  it('the pill is inset 2px, radius --hub-radius-sm, token fill and border', () => {
    const r = rule('.keybar__key::before')
    expect(r).toContain('inset: 2px')
    expect(r).toContain('border: 1px solid var(--color-control-border)')
    expect(r).toContain('border-radius: var(--hub-radius-sm)')
    expect(r).toContain('background: var(--color-button)')
  })

  it('pressed uses --color-selection-inactive', () => {
    expect(rule('.keybar__key:active::before')).toContain('background: var(--color-selection-inactive)')
  })

  it('armed Ctrl is the accent fill with on-accent text and an 8px on-accent dot', () => {
    expect(rule('.keybar__key--armed[aria-pressed="true"]')).toContain('color: var(--color-on-accent)')
    expect(rule('.keybar__key--armed[aria-pressed="true"]::before')).toContain('background: var(--color-selection-fill)')
    const dot = rule('.keybar__key--armed[aria-pressed="true"]::after')
    expect(dot).toContain('width: 8px')
    expect(dot).toContain('height: 8px')
    expect(dot).toContain('background: var(--color-on-accent)')
  })

  it('the pipe glyph is 17px/400', () => {
    const r = rule('.keybar__key--pipe')
    expect(r).toContain('font-size: 17px')
    expect(r).toContain('font-weight: 400')
  })

  it('has the desktop backstop', () => {
    expect(css).toMatch(/@media not all and \(pointer: coarse\) and \(hover: none\) \{\s*\.keybar \{ display: none \}/)
  })

  it('uses no literal colour in any .keybar rule', () => {
    const start = css.indexOf('/* ─── Phone key bar')
    const block = css.slice(start)
    expect(block).not.toMatch(/#[0-9a-fA-F]{3,8}\b|rgba?\(/)
  })
})
