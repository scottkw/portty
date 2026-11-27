// Phase 20-07 — the viewer's control bar: the display and window picker (DESK-05, DESK-06).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopSurfacesMsg } from '../../lib/desktopClient'
import { DesktopControlBar, type DesktopControlBarProps } from '../DesktopControlBar'

let container: HTMLDivElement
let root: Root

const LIST: DesktopSurfacesMsg = {
  displays: [
    { id: 1, name: 'Main Display', w: 1470, h: 956, main: true },
    { id: 2, name: 'Display 2', w: 1920, h: 1080, main: false },
  ],
  windows: [{ id: 4242, app: 'Terminal', title: 'zsh', name: 'Terminal — zsh' }],
}

function props(over: Partial<DesktopControlBarProps> = {}): DesktopControlBarProps {
  return {
    sendChord: vi.fn(),
    onMenuOpenChange: vi.fn(),
    onSendKeysDone: vi.fn(),
    windowShare: false,
    surface: { kind: 'display', id: 1, name: 'Main Display' },
    switching: false,
    requestSurfaces: vi.fn(() => Promise.resolve(LIST)),
    onSelectSurface: vi.fn(),
    ...over,
  }
}

function render(p: DesktopControlBarProps) {
  act(() => root.render(<DesktopControlBar {...p} />))
  return p
}

const picker = () => container.querySelector<HTMLButtonElement>('.desktop-bar__picker')!
const menu = () => document.querySelector<HTMLElement>('[role="menu"]')
const radios = () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitemradio"]'))
const text = (el: Element) => el.querySelector('.desktop-menu__text')?.textContent
const hint = (el: Element) => el.querySelector('.desktop-menu__hint')?.textContent
const labels = () => Array.from(document.querySelectorAll('.desktop-menu__label')).map((e) => e.textContent)
const caption = () => document.querySelector('.desktop-menu__caption')?.textContent

async function openPicker() {
  await act(async () => {
    picker().click()
  })
}

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
})

describe('picker (Task 2)', () => {
  it('asks the daemon for the list each time it opens, and not before', async () => {
    const p = render(props())
    expect(p.requestSurfaces).not.toHaveBeenCalled()
    await openPicker()
    expect(p.requestSurfaces).toHaveBeenCalledTimes(1)
    act(() => document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true })))
    await openPicker()
    expect(p.requestSurfaces).toHaveBeenCalledTimes(2)
  })

  it('shows the group labels with a greyed Loading… while the list is on its way', () => {
    let resolve!: (l: DesktopSurfacesMsg) => void
    const p = render(props({ requestSurfaces: vi.fn(() => new Promise<DesktopSurfacesMsg>((r) => { resolve = r })) }))
    act(() => picker().click())
    expect(labels()).toEqual(['Displays', 'Windows'])
    const loading = document.querySelector('[aria-disabled="true"]')!
    expect(loading.textContent).toBe('Loading…')
    expect(caption()).toBe("Menus and pop-ups don't appear in a window share. Share the display to see them.")
    expect(p.onMenuOpenChange).toHaveBeenCalledWith(true)
    act(() => resolve(LIST))
  })

  it('lists Main Display (checked) then Display 2 with their size right-aligned, and windows as App — Title', async () => {
    render(props())
    await openPicker()
    expect(menu()?.getAttribute('role')).toBe('menu')
    expect(radios().map((r) => [text(r), hint(r), r.getAttribute('aria-checked')])).toEqual([
      ['Main Display', '1470 x 956', 'true'],
      ['Display 2', '1920 x 1080', 'false'],
      ['Terminal — zsh', undefined, 'false'],
    ])
    expect(labels()).toEqual(['Displays', 'Windows'])
    expect(caption()).toContain('Menus and pop-ups')
    expect(document.activeElement).toBe(radios()[0])
  })

  it('checks the current window', async () => {
    render(props({ surface: { kind: 'window', id: 4242, name: 'Terminal — zsh' } }))
    await openPicker()
    expect(radios().map((r) => r.getAttribute('aria-checked'))).toEqual(['false', 'false', 'true'])
  })

  it('one display still shows Main Display checked and the windows', async () => {
    render(props({ requestSurfaces: vi.fn(() => Promise.resolve({ ...LIST, displays: [LIST.displays[0]] })) }))
    await openPicker()
    expect(radios().map((r) => text(r))).toEqual(['Main Display', 'Terminal — zsh'])
    expect(radios()[0].getAttribute('aria-checked')).toBe('true')
  })

  it('an empty window list shows a greyed No windows to show, and the displays still work', async () => {
    const p = render(props({ requestSurfaces: vi.fn(() => Promise.resolve({ ...LIST, windows: [] })) }))
    await openPicker()
    const none = Array.from(document.querySelectorAll('[aria-disabled="true"]')).find((e) => e.textContent === 'No windows to show')
    expect(none).toBeDefined()
    expect(caption()).toContain('Menus and pop-ups')
    act(() => radios()[1].click())
    expect(p.onSelectSurface).toHaveBeenCalledWith({ kind: 'display', id: 2, name: 'Display 2' })
  })

  it('a failed request shows one greyed item and still the caption', async () => {
    render(props({ requestSurfaces: vi.fn(() => Promise.reject(new Error('timeout'))) }))
    await openPicker()
    const greyed = Array.from(document.querySelectorAll('[aria-disabled="true"]'))
    expect(greyed.map((e) => e.textContent)).toEqual(["Couldn't load the list. Close the menu and try again."])
    expect(radios()).toHaveLength(0)
    expect(caption()).toContain('Menus and pop-ups')
  })

  it('choosing a window calls onSelectSurface with kind, id and name, and closes the menu', async () => {
    const p = render(props())
    await openPicker()
    act(() => radios()[2].click())
    expect(p.onSelectSurface).toHaveBeenCalledWith({ kind: 'window', id: 4242, name: 'Terminal — zsh' })
    expect(menu()).toBeNull()
    expect(p.onMenuOpenChange).toHaveBeenLastCalledWith(false)
  })

  it('choosing the surface already shown does nothing but close', async () => {
    const p = render(props())
    await openPicker()
    act(() => radios()[0].click())
    expect(p.onSelectSurface).not.toHaveBeenCalled()
    expect(menu()).toBeNull()
  })

  it('the button reads Switching… and is greyed, and does not open, while a pick is in flight', async () => {
    const p = render(props({ switching: true }))
    expect(picker().textContent).toBe('Switching…')
    expect(picker().getAttribute('aria-disabled')).toBe('true')
    expect(picker().disabled).toBe(false)
    await openPicker()
    expect(p.requestSurfaces).not.toHaveBeenCalled()
    expect(menu()).toBeNull()
  })

  it('the button shows the current name with the full name in title and the accessible name', () => {
    const long = 'Safari — ' + 'x'.repeat(300)
    render(props({ surface: { kind: 'window', id: 7, name: long } }))
    expect(picker().getAttribute('title')).toBe(long)
    expect(picker().getAttribute('aria-label')).toBe(long)
    expect(picker().textContent).toBe(long)
  })

  it('a window name of 300 characters renders with the full text in its title attribute, as text', async () => {
    const long = 'App — ' + 'y'.repeat(300)
    const evil = '<img src=x onerror=alert(1)>'
    render(props({
      requestSurfaces: vi.fn(() => Promise.resolve({ ...LIST, windows: [{ id: 1, app: 'App', title: evil, name: long }, { id: 2, app: 'A', title: evil, name: evil }] })),
    }))
    await openPicker()
    const [, , w1, w2] = radios()
    expect(w1.getAttribute('title')).toBe(long)
    expect(text(w1)).toBe(long)
    expect(text(w2)).toBe(evil)
    expect(w2.querySelector('img')).toBeNull()
  })

  it('Esc closes the picker and returns focus to the button', async () => {
    render(props())
    await openPicker()
    act(() => { document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(menu()).toBeNull()
    expect(document.activeElement).toBe(picker())
  })

  it('a list that arrives after the menu closed does not reopen or change anything', async () => {
    let resolve!: (l: DesktopSurfacesMsg) => void
    render(props({ requestSurfaces: vi.fn(() => new Promise<DesktopSurfacesMsg>((r) => { resolve = r })) }))
    act(() => picker().click())
    act(() => { document.activeElement?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    act(() => document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })))
    await act(async () => resolve(LIST))
    expect(menu()).toBeNull()
  })
})
