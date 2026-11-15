import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Source-inspection tests for the Settings sheet CSS (Portty 02-11, UI-SPEC
// "Settings sheet", "Switch", "Inputs and native controls"). Retargeted from the
// old Settings-tab rules (.settings-tab, .settings-panel__body h3, description
// font-size), which were deleted with the tab.
// Uses fs.readFileSync because vitest/jsdom does not support ?raw imports for CSS.
const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

function rule(selector: string): string {
  const idx = css.indexOf(`${selector} {`)
  expect(idx, `missing rule ${selector}`).toBeGreaterThan(-1)
  return css.slice(idx, css.indexOf('}', idx) + 1)
}

describe('UI-02 Gap 7: retired modal and tab classes stay gone', () => {
  it('has no settings overlay, panel, tab, jump bar or search rules', () => {
    expect(css).not.toMatch(/\.settings-(overlay|tab\b|jump-bar|search)/)
    expect(css).not.toMatch(/\.settings-panel(__(header|footer|close|tabs|tab-btn|body|toggle-[a-z]+|btn[a-z-]*|field-group|label))?\s*[{,.:]/)
  })

  it('has no Settings → Paths rules (Portty 01-11)', () => {
    expect(css).not.toMatch(/settings-panel__(table|cli-name|path-row|browse-btn)/)
  })
})

describe('LOOK-01: Settings sheet layout', () => {
  it('is 720×560 within calc(100% - 48px), with no outer padding', () => {
    const r = rule('.dialog--settings')
    expect(r).toContain('width: 720px')
    expect(r).toContain('height: 560px')
    expect(r).toContain('calc(100% - 48px)')
    expect(r).toContain('padding: 0')
  })

  it('only the pane content scrolls (header and footer stay visible on short windows)', () => {
    const content = rule('.settings-sheet__content')
    expect(content).toContain('overflow-y: auto')
    expect(content).toContain('min-height: 0')
    expect(content).toContain('padding: 24px')
    expect(content).toContain('var(--color-sheet-content)')
    expect(rule('.settings-sheet__header')).toContain('flex-shrink: 0')
    expect(rule('.settings-sheet__footer')).toContain('flex-shrink: 0')
    expect(rule('.settings-sheet__footer')).toContain('height: 52px')
  })

  it('header is the toolbar colour; pane tabs are 84×52 with the selected fill and accent', () => {
    expect(rule('.settings-sheet__header')).toContain('var(--color-toolbar)')
    const tab = rule('.settings-sheet__tab')
    expect(tab).toContain('width: 84px')
    expect(tab).toContain('height: 52px')
    const selected = rule(".settings-sheet__tab[aria-selected='true']")
    expect(selected).toContain('var(--color-pane-tab-selected)')
    expect(selected).toContain('var(--color-accent)')
  })

  it('groups and rows follow the grouped-form metrics', () => {
    const group = rule('.settings-group')
    expect(group).toContain('var(--color-group)')
    expect(group).toContain('1px solid var(--color-group-border)')
    expect(group).toContain('border-radius: 10px')
    const row = rule('.settings-row')
    expect(row).toContain('min-height: 44px')
    expect(row).toContain('padding: 12px 16px')
    expect(row).toContain('gap: 12px')
    expect(rule('.settings-row__description')).toContain('font-size: 12px')
  })

  it('the port input is 72px and right-aligned; selects keep the native popup', () => {
    const port = rule('.settings-input--number')
    expect(port).toContain('width: 72px')
    expect(port).toContain('text-align: right')
    expect(rule('.settings-field select')).toContain('appearance: auto')
  })
})

describe('LOOK-01: Switch uses the token colours', () => {
  it('is 38×22 with an 18px knob; off and on tracks use the switch tokens', () => {
    const sw = rule('.switch')
    expect(sw).toContain('width: 38px')
    expect(sw).toContain('height: 22px')
    expect(sw).toContain('var(--color-switch-off)')
    expect(rule(".switch[aria-checked='true']")).toContain('var(--color-selection-fill)')
    const knob = rule('.switch__knob')
    expect(knob).toContain('width: 18px')
    expect(knob).toContain('var(--color-switch-knob)')
    expect(knob).toContain('var(--shadow-switch-knob)')
  })

  it('saving dims the switch and blocks pointer events', () => {
    expect(css).toMatch(/\n\.switch--saving\s*\{[^}]*pointer-events: none/)
    expect(css).toMatch(/\.switch:disabled,\s*\.switch--saving\s*\{[^}]*var\(--opacity-disabled\)/)
  })
})
