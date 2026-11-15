import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { act } from 'react'
import raw from '../../components/SettingsSheet.tsx?raw'
import { SettingsSheet } from '../SettingsSheet'

// Every Wails binding resolves to undefined except the two loads the General pane needs.
vi.mock('../../wailsjs/go/main/App', async (importOriginal) => {
  const mod = await importOriginal<Record<string, unknown>>()
  const fns: Record<string, unknown> = Object.fromEntries(Object.keys(mod).map((k) => [k, vi.fn().mockResolvedValue(undefined)]))
  fns.GetStartMinimized = vi.fn().mockResolvedValue(false)
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
  it('THEM-01: Appearance reads "Automatic" with no theme picker', async () => {
    await renderSheet()
    const general = panel('general')
    expect(general.textContent).toContain('Automatic')
    expect(general.textContent).toContain("Follows your Mac's Light or Dark setting in System Settings.")
    expect(container.textContent).not.toMatch(/Interface Theme|Terminal Theme|Stay on Hub/i)
    expect(raw).not.toMatch(/ALLOWED_THEMES|selectedTheme|onThemeChange|uiTheme|StayOnHub/)
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
