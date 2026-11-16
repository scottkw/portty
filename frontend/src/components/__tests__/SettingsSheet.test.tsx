import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { act } from 'react'
import raw from '../../components/SettingsSheet.tsx?raw'
import { readFileSync } from 'fs'
import { resolve } from 'path'
import { SettingsSheet } from '../SettingsSheet'
import { SetAppearance, SetTerminalFont, GetTerminalFont } from '../../wailsjs/go/main/App'

const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf-8')

// Every Wails binding resolves to undefined except the loads the General pane needs.
vi.mock('../../wailsjs/go/main/App', async (importOriginal) => {
  const mod = await importOriginal<Record<string, unknown>>()
  const fns: Record<string, unknown> = Object.fromEntries(Object.keys(mod).map((k) => [k, vi.fn().mockResolvedValue(undefined)]))
  fns.GetStartMinimized = vi.fn().mockResolvedValue(false)
  fns.GetAppearance = vi.fn().mockResolvedValue('system')
  fns.GetTerminalFont = vi.fn().mockResolvedValue({ family: '', size: 13 })
  fns.GetPluginSettings = vi.fn().mockResolvedValue({
    webgl: true, unicode11: false, search: true, webLinks: true, image: false,
    serialize: false, clipboard: false, progress: false,
    searchConfig: { regex: false, caseSensitive: false, wholeWord: false },
    webLinksConfig: { modifier: 'platform', confirmOSC8: true, confirmIDN: true, confirmTyposquat: true },
    imageConfig: { storageLimit: 16 },
  })
  return fns
})
// Wails-generated daemon.* constructors fail under jsdom.
vi.mock('../../wailsjs/go/models', () => {
  class Stub { constructor(o: Record<string, unknown> = {}) { Object.assign(this, o) } }
  return { daemon: { PluginSettings: Stub, SearchConfig: Stub, WebLinksConfig: Stub, ImageConfig: Stub } }
})
vi.mock('../../wailsjs/wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
  ClipboardSetText: vi.fn().mockResolvedValue(undefined),
}))

let container: HTMLDivElement
let root: Root

async function renderSheet(overrides: Partial<Parameters<typeof SettingsSheet>[0]> = {}) {
  const onClose = vi.fn()
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root.render(React.createElement(SettingsSheet, {
      isOpen: true,
      onClose,
      tailscaleHealth: null,
      webServerMode: null,
      onWebServerStateChange: vi.fn().mockResolvedValue(undefined),
      ...overrides,
    }))
  })
  await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
  return { onClose }
}

afterEach(async () => {
  await act(async () => root.unmount())
  container.remove()
})

const tabs = () => Array.from(container.querySelectorAll<HTMLButtonElement>('[role="tab"]'))
const tab = (name: string) => tabs().find((t) => t.textContent === name)!
const panel = (id: string) => container.querySelector<HTMLElement>(`#settings-pane-${id}`)!

describe('LOOK-01 / NAV-03: Settings is one sheet over the window', () => {
  it('renders nothing while closed', async () => {
    await renderSheet({ isOpen: false })
    expect(container.innerHTML).toBe('')
  })

  it('is a modal dialog labelled by the centred pane name', async () => {
    await renderSheet()
    const dialog = container.querySelector('[role="dialog"]')!
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    expect(dialog.className).toContain('dialog--settings')
    const title = container.querySelector(`#${dialog.getAttribute('aria-labelledby')}`)!
    expect(title.textContent).toBe('General')
  })

  it('has General / Sharing / Security pane tabs with General selected', async () => {
    await renderSheet()
    const list = container.querySelector('[role="tablist"]')!
    expect(list.getAttribute('aria-label')).toBe('Settings sections')
    expect(tabs().map((t) => t.textContent)).toEqual(['General', 'Sharing', 'Security'])
    expect(tab('General').getAttribute('aria-selected')).toBe('true')
    expect(tabs().every((t) => t.querySelector('svg'))).toBe(true)
    expect(panel('general').hidden).toBe(false)
    expect(panel('sharing').hidden).toBe(true)
    expect(panel('security').hidden).toBe(true)
  })

  it('switches panes on click and with the arrow keys', async () => {
    await renderSheet()
    await act(async () => tab('Sharing').click())
    expect(tab('Sharing').getAttribute('aria-selected')).toBe('true')
    expect(panel('sharing').hidden).toBe(false)
    expect(panel('general').hidden).toBe(true)
    expect(container.querySelector('#settings-sheet-title')!.textContent).toBe('Sharing')

    await act(async () => {
      tab('Sharing').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    })
    expect(tab('Security').getAttribute('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(tab('Security'))
    await act(async () => {
      tab('Security').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    })
    expect(tab('General').getAttribute('aria-selected')).toBe('true')
    await act(async () => {
      tab('General').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true }))
    })
    expect(tab('Security').getAttribute('aria-selected')).toBe('true')
  })

  it('Done (focused on open) closes the sheet', async () => {
    const { onClose } = await renderSheet()
    const done = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Done')!
    expect(done.className).toContain('button--primary')
    expect(document.activeElement).toBe(done)
    await act(async () => done.click())
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('Esc closes the sheet', async () => {
    const { onClose } = await renderSheet()
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    })
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})

describe('General pane', () => {
  const radios = () => Array.from(container.querySelectorAll<HTMLButtonElement>('[role="radiogroup"][aria-label="Appearance"] [role="radio"]'))
  const radio = (name: string) => radios().find((r) => r.textContent === name)!
  const checked = () => radios().filter((r) => r.getAttribute('aria-checked') === 'true').map((r) => r.textContent)
  const flush = () => act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })

  it('THEM-01: Appearance is Automatic / Light / Dark with Automatic checked, and no theme picker', async () => {
    await renderSheet()
    expect(radios().map((r) => r.textContent)).toEqual(['Automatic', 'Light', 'Dark'])
    expect(checked()).toEqual(['Automatic'])
    expect(radios().map((r) => r.tabIndex)).toEqual([0, -1, -1])
    // Equal-width segments sized to the widest label (M-68: "Automatic" was cramped).
    expect(container.querySelector('[role="radiogroup"][aria-label="Appearance"]')!.classList).toContain('settings-appearance')
    expect(css).toMatch(/\.settings-appearance \{[^}]*grid-auto-columns: 1fr;/)
    expect(panel('general').textContent).toContain("Automatic follows your Mac's Light or Dark setting in System Settings.")
    expect(container.textContent).not.toMatch(/Interface Theme|Terminal Theme|Stay on Hub/i)
    expect(raw).not.toMatch(/ALLOWED_THEMES|selectedTheme|onThemeChange|uiTheme|StayOnHub/)
  })

  it('THEM-01: clicking Dark saves it and notifies onAppearanceChange', async () => {
    const onAppearanceChange = vi.fn()
    vi.mocked(SetAppearance).mockClear()
    await renderSheet({ onAppearanceChange })
    await act(async () => radio('Dark').click())
    await flush()
    expect(SetAppearance).toHaveBeenCalledWith('dark')
    expect(onAppearanceChange).toHaveBeenCalledWith('dark')
    expect(checked()).toEqual(['Dark'])
  })

  it('THEM-01: a rejected SetAppearance restores the previous choice and shows the error', async () => {
    const onAppearanceChange = vi.fn()
    vi.mocked(SetAppearance).mockRejectedValueOnce(new Error('daemon not connected'))
    await renderSheet({ onAppearanceChange })
    await act(async () => radio('Light').click())
    await flush()
    expect(checked()).toEqual(['Automatic'])
    expect(onAppearanceChange).not.toHaveBeenCalled()
    expect(container.querySelector('.settings-sheet__error')!.textContent).toContain('daemon not connected')
  })

  it('THEM-01: ArrowRight from Automatic selects Light and moves focus', async () => {
    vi.mocked(SetAppearance).mockClear()
    await renderSheet()
    await act(async () => {
      radio('Automatic').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    })
    await flush()
    expect(checked()).toEqual(['Light'])
    expect(document.activeElement).toBe(radio('Light'))
    expect(SetAppearance).toHaveBeenCalledWith('light')
  })

  // Review WR-04: overlapping saves could persist and apply out of order.
  it('THEM-01: ignores further choices while a SetAppearance is in flight', async () => {
    let resolve!: () => void
    vi.mocked(SetAppearance).mockClear()
    vi.mocked(SetAppearance).mockImplementationOnce(() => new Promise<void>((r) => { resolve = r }))
    await renderSheet()
    await act(async () => radio('Light').click())
    await act(async () => {
      radio('Light').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }))
    })
    await act(async () => radio('Dark').click())
    expect(SetAppearance).toHaveBeenCalledTimes(1)
    expect(checked()).toEqual(['Light'])
    await act(async () => { resolve() })
    await flush()
    await act(async () => radio('Dark').click())
    await flush()
    expect(SetAppearance).toHaveBeenLastCalledWith('dark')
    expect(checked()).toEqual(['Dark'])
  })

  // Quick 261116-ku9 R1: Terminal Font commits on blur or Enter.
  describe('Terminal Font', () => {
    const family = () => container.querySelector<HTMLInputElement>('#terminal-font-family')!
    const size = () => container.querySelector<HTMLInputElement>('#terminal-font-size')!
    function type(input: HTMLInputElement, value: string) {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
      setter.call(input, value)
      input.dispatchEvent(new Event('input', { bubbles: true }))
    }
    const blur = (input: HTMLInputElement) => input.dispatchEvent(new FocusEvent('focusout', { bubbles: true }))

    it('shows a Terminal Font row with name and size fields holding the loaded values', async () => {
      vi.mocked(GetTerminalFont).mockResolvedValueOnce({ family: 'Menlo', size: 15 })
      await renderSheet()
      expect(container.querySelector('label[for="terminal-font-family"]')!.textContent).toBe('Terminal Font')
      expect(panel('general').textContent).toContain('Leave the name empty to use SF Mono. Nerd Font icons keep working.')
      expect(family().getAttribute('aria-label')).toBe('Font name')
      expect(family().placeholder).toBe('SF Mono')
      expect(family().value).toBe('Menlo')
      expect(size().getAttribute('aria-label')).toBe('Font size')
      expect(size().type).toBe('number')
      expect(size().min).toBe('6')
      expect(size().max).toBe('32')
      expect(size().value).toBe('15')
    })

    it('blur commits the edited name once and notifies onTerminalFontChange', async () => {
      const onTerminalFontChange = vi.fn()
      vi.mocked(SetTerminalFont).mockClear()
      await renderSheet({ onTerminalFontChange })
      await act(async () => type(family(), 'Menlo'))
      await act(async () => blur(family()))
      await flush()
      expect(SetTerminalFont).toHaveBeenCalledTimes(1)
      expect(SetTerminalFont).toHaveBeenCalledWith('Menlo', 13)
      expect(onTerminalFontChange).toHaveBeenCalledWith({ family: 'Menlo', size: 13 })
      await act(async () => blur(family()))
      await flush()
      expect(SetTerminalFont).toHaveBeenCalledTimes(1)
    })

    it('Enter commits the same way', async () => {
      const onTerminalFontChange = vi.fn()
      vi.mocked(SetTerminalFont).mockClear()
      await renderSheet({ onTerminalFontChange })
      await act(async () => type(family(), 'Menlo'))
      await act(async () => {
        family().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
      })
      await flush()
      expect(SetTerminalFont).toHaveBeenCalledWith('Menlo', 13)
      expect(onTerminalFontChange).toHaveBeenCalledWith({ family: 'Menlo', size: 13 })
    })

    it('blurring with no change makes no call', async () => {
      vi.mocked(SetTerminalFont).mockClear()
      await renderSheet()
      await act(async () => blur(family()))
      await act(async () => blur(size()))
      await flush()
      expect(SetTerminalFont).not.toHaveBeenCalled()
    })

    it('a rejected save shows the error and restores both fields', async () => {
      const onTerminalFontChange = vi.fn()
      vi.mocked(SetTerminalFont).mockRejectedValueOnce(new Error('invalid terminal font size 40: want 6 to 32'))
      await renderSheet({ onTerminalFontChange })
      await act(async () => type(family(), 'Menlo'))
      await act(async () => type(size(), '40'))
      await act(async () => blur(size()))
      await flush()
      expect(family().value).toBe('')
      expect(size().value).toBe('13')
      expect(onTerminalFontChange).not.toHaveBeenCalled()
      const alerts = Array.from(container.querySelectorAll('[role="alert"]')).map((a) => a.textContent)
      expect(alerts).toContain("Couldn't save the terminal font: invalid terminal font size 40: want 6 to 32.")
    })
  })

  it('"Start hidden in the menu bar" is a labelled switch', async () => {
    await renderSheet()
    const sw = container.querySelector<HTMLButtonElement>('#startMinimized')!
    expect(sw.getAttribute('role')).toBe('switch')
    expect(sw.getAttribute('aria-checked')).toBe('false')
    expect(container.querySelector('label[for="startMinimized"]')!.textContent).toBe('Start hidden in the menu bar')
    expect(panel('general').textContent).toContain('Opens with no window. Click the menu bar icon to show it.')
  })

  it('renders one switch per terminal plugin and the Save Plugins button', async () => {
    await renderSheet()
    const general = panel('general')
    expect(general.textContent).toContain('Terminal Plugins')
    const pluginSwitches = general.querySelectorAll('[role="switch"][id^="plugin-"]')
    expect(pluginSwitches).toHaveLength(8)
    expect(general.querySelector('label[for="plugin-webgl"]')!.textContent).toBe('WebGL renderer')
    expect(Array.from(general.querySelectorAll('button')).some((b) => b.textContent === 'Save Plugins')).toBe(true)
  })
})

describe('Security pane', () => {
  it('Certificate Transparency: "I Understand", then "Acknowledged" in the success style', async () => {
    await renderSheet()
    const security = panel('security')
    expect(security.textContent).toContain(
      "Starting the web server gets a Let's Encrypt certificate for your Mac's tailnet name (for example hostname.ts.net). That name becomes permanently visible in public Certificate Transparency logs.",
    )
    const understand = Array.from(security.querySelectorAll('button')).find((b) => b.textContent === 'I Understand')!
    expect(understand.className).toContain('button--secondary')
    await act(async () => understand.click())
    await act(async () => { await new Promise<void>((r) => setTimeout(r, 0)) })
    const ack = security.querySelector('.settings-row__success')!
    expect(ack.textContent).toBe('Acknowledged')
    expect(security.textContent).toContain("Your tailnet certificate's name is published in public CT logs.")
    expect(Array.from(security.querySelectorAll('button')).some((b) => b.textContent === 'I Understand')).toBe(false)
  })

  it('Session keys: "Regenerate…" opens the confirm with a destructive "Invalidate All Links"', async () => {
    await renderSheet()
    const security = panel('security')
    expect(security.textContent).toContain('Regenerating signs out every browser and Mac attached to your sessions.')
    const regen = Array.from(security.querySelectorAll('button')).find((b) => b.textContent === 'Regenerate…')!
    await act(async () => regen.click())
    const confirm = container.querySelector('[aria-labelledby="regen-key-title"]')!
    expect(confirm.textContent).toContain('Regenerate Signing Key?')
    const invalidate = Array.from(confirm.querySelectorAll('button')).find((b) => b.textContent === 'Invalidate All Links')!
    expect(invalidate.className).toContain('button--destructive')
    // Cancel first (T-02-36), and the confirm sits after the sheet so it owns Esc.
    expect(document.activeElement?.textContent).toBe('Keep Links')
  })
})

describe('LOOK-01: no inline colour or check glyph in SettingsSheet.tsx', () => {
  it('has no inline colour/background style and no #9ece6a', () => {
    expect(raw).not.toMatch(/style=\{\{[^}]*(color|background)/)
    expect(raw).not.toMatch(/#[0-9a-fA-F]{3,6}\b/)
    expect(raw).not.toContain('&#10003;')
  })
})
