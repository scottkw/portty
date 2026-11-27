// Phase 20-07 — the viewer's control bar: the display and window picker (DESK-05, DESK-06).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopSurfacesMsg } from '../../lib/desktopClient'
import {
  DesktopControlBar,
  HIDE_AFTER_MS,
  REVEAL_CHANGE_MS,
  REVEAL_FIRST_MS,
  barAvailability,
  type BarState,
  type DesktopControlBarProps,
} from '../DesktopControlBar'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

let container: HTMLDivElement
let root: Root

const LIST: DesktopSurfacesMsg = {
  displays: [
    { id: 1, name: 'Main Display', w: 1470, h: 956, main: true },
    { id: 2, name: 'Display 2', w: 1920, h: 1080, main: false },
  ],
  windows: [{ id: 4242, app: 'Terminal', title: 'zsh', name: 'Terminal — zsh' }],
}

const STREAMING: BarState = {
  locked: false, reconnecting: false, accessNeeded: false, switching: false, mode: 'control', canSurface: true, fullscreenAvailable: true, canClipboard: true,
}

function props(over: Partial<Omit<DesktopControlBarProps, 'state'>> & { state?: Partial<BarState> } = {}): DesktopControlBarProps {
  const { state, ...rest } = over
  return {
    state: { ...STREAMING, ...state },
    sendChord: vi.fn(),
    onMenuOpenChange: vi.fn(),
    onSendKeysDone: vi.fn(),
    surface: { kind: 'display', id: 1, name: 'Main Display' },
    requestSurfaces: vi.fn(() => Promise.resolve(LIST)),
    onSelectSurface: vi.fn(),
    swapOn: false,
    onSwapChange: vi.fn(),
    onModeChange: vi.fn(),
    fullscreen: false,
    onFullscreen: vi.fn(),
    controlRef: { current: null },
    onCopy: vi.fn(),
    onPaste: vi.fn(),
    clipBusy: false,
    ...rest,
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
    const p = render(props({ state: { switching: true } }))
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

describe('barAvailability (Task 3)', () => {
  const a = (over: Partial<BarState> = {}) => barAvailability({ ...STREAMING, ...over })
  const view = (r: ReturnType<typeof a>) =>
    Object.fromEntries(Object.entries(r).map(([k, v]) => [k, v.hidden ? 'hidden' : v.disabled ? (v.reason ?? 'greyed') : 'ok']))

  it('streaming: everything enabled', () => {
    expect(view(a())).toEqual({ picker: 'ok', sendKeys: 'ok', clipboard: 'ok', swap: 'ok', control: 'ok', fullscreen: 'ok' })
  })
  it('locked: what sends is greyed Locked; Swap and Full Screen stay', () => {
    expect(view(a({ locked: true }))).toEqual({
      picker: 'Locked', sendKeys: 'Locked', clipboard: 'Locked', swap: 'ok', control: 'Locked', fullscreen: 'ok',
    })
  })
  it('reconnecting: the same with Reconnecting', () => {
    expect(view(a({ reconnecting: true }))).toEqual({
      picker: 'Reconnecting', sendKeys: 'Reconnecting', clipboard: 'Reconnecting', swap: 'ok', control: 'Reconnecting', fullscreen: 'ok',
    })
  })
  it('needs accessibility: Send Keys and the Control segment only', () => {
    const r = a({ accessNeeded: true })
    expect(view(r)).toMatchObject({ picker: 'ok', sendKeys: 'Control needs Accessibility', clipboard: 'ok', control: 'Control needs Accessibility' })
    expect(r.control.controlOnly).toBe(true)
  })
  it('switching greys the picker only; View Only greys Send Keys with Turn on Control first', () => {
    expect(view(a({ switching: true })).picker).toBe('Switching…')
    expect(view(a({ switching: true })).sendKeys).toBe('ok')
    expect(view(a({ mode: 'view' }))).toMatchObject({ sendKeys: 'Turn on Control first', picker: 'ok', control: 'ok' })
  })
  it('hides what this viewer can never use', () => {
    expect(view(a({ canSurface: false, fullscreenAvailable: false }))).toMatchObject({ picker: 'hidden', fullscreen: 'hidden' })
    expect(view(a({ canClipboard: false })).clipboard).toBe('hidden')
  })
  it('locked wins over View Only', () => {
    expect(a({ locked: true, mode: 'view' }).sendKeys.reason).toBe('Locked')
  })
})

describe('bar behaviour (Task 3)', () => {
  const bar = () => container.querySelector('.desktop-bar') as HTMLElement
  const hidden = () => bar().classList.contains('desktop-bar--hidden')
  const adv = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })
  const move = (x: number, y: number) =>
    act(() => { document.dispatchEvent(new PointerEvent('pointermove', { clientX: x, clientY: y, bubbles: true })) })
  const btn = (name: string) => container.querySelector<HTMLButtonElement>(`button[aria-label="${name}"]`)!
  const radio = (name: string) => Array.from(container.querySelectorAll<HTMLElement>('[role="radio"]')).find((r) => r.textContent === name)!
  const mid = () => window.innerWidth / 2

  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows for 5 s after the first frame, then hides, and reveals again by focus', () => {
    render(props())
    expect(hidden()).toBe(false)
    adv(REVEAL_FIRST_MS - 1)
    expect(hidden()).toBe(false)
    adv(1)
    expect(hidden()).toBe(true)
    act(() => btn('Send Keys').focus())
    expect(hidden()).toBe(false)
    act(() => btn('Send Keys').blur())
    expect(hidden()).toBe(true)
  })

  it('a pointermove into the top 16 px middle half reveals it; leaving hides it 1.5 s later', () => {
    render(props())
    adv(REVEAL_FIRST_MS)
    expect(hidden()).toBe(true)
    move(100, 5)
    expect(hidden()).toBe(true)
    move(mid(), 20)
    expect(hidden()).toBe(true)
    move(mid(), 5)
    expect(hidden()).toBe(false)
    move(mid(), 300)
    adv(HIDE_AFTER_MS - 1)
    expect(hidden()).toBe(false)
    adv(1)
    expect(hidden()).toBe(true)
  })

  it('returning to the zone before the delay cancels the hide', () => {
    render(props())
    adv(REVEAL_FIRST_MS)
    move(mid(), 5)
    move(mid(), 300)
    adv(1000)
    move(mid(), 5)
    adv(2000)
    expect(hidden()).toBe(false)
  })

  it('stays shown while a menu is open', () => {
    render(props())
    adv(REVEAL_FIRST_MS)
    act(() => btn('Send Keys').click())
    expect(hidden()).toBe(false)
    adv(10000)
    expect(hidden()).toBe(false)
  })

  it('shows for 3 s when the lock state, control availability or surface changes', () => {
    render(props())
    adv(REVEAL_FIRST_MS)
    render(props({ state: { locked: true } }))
    expect(hidden()).toBe(false)
    adv(REVEAL_CHANGE_MS - 1)
    expect(hidden()).toBe(false)
    adv(1)
    expect(hidden()).toBe(true)
    render(props({ state: { locked: true, accessNeeded: true } }))
    expect(hidden()).toBe(false)
    adv(REVEAL_CHANGE_MS)
    expect(hidden()).toBe(true)
    render(props({ state: { locked: true, accessNeeded: true }, surface: { kind: 'window', id: 9, name: 'A — B' } }))
    expect(hidden()).toBe(false)
  })

  it('is a toolbar named Desktop controls with 1 px separators between the groups', () => {
    render(props())
    expect(bar().getAttribute('role')).toBe('toolbar')
    expect(bar().getAttribute('aria-label')).toBe('Desktop controls')
    expect(bar().querySelectorAll('.desktop-bar__divider')).toHaveLength(3)
    const seen = Array.from(bar().querySelectorAll('button, .desktop-bar__divider')).map((e) =>
      e.classList.contains('desktop-bar__divider') ? '|' : e.getAttribute('aria-label'),
    )
    expect(seen).toEqual(['Main Display', '|', 'Send Keys', 'Clipboard', '|', 'Swap Ctrl and Cmd', null, null, '|', 'Full Screen'])
  })

  it('greyed items are aria-disabled, never disabled, carry the reason as title and as linked text, and ignore clicks', () => {
    const p = render(props({ state: { locked: true } }))
    const sk = btn('Send Keys')
    expect(sk.getAttribute('aria-disabled')).toBe('true')
    expect(sk.disabled).toBe(false)
    expect(sk.getAttribute('title')).toBe('Locked')
    expect(document.getElementById(sk.getAttribute('aria-describedby')!)?.textContent).toBe('Locked')
    act(() => sk.click())
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(p.onMenuOpenChange).not.toHaveBeenCalled()
    expect(btn('Swap Ctrl and Cmd').getAttribute('aria-disabled')).toBeNull()
  })

  it('every bar button keeps its full aria-label and title (the narrow layout hides only the text)', () => {
    render(props())
    for (const n of ['Send Keys', 'Swap Ctrl and Cmd', 'Full Screen']) {
      expect(btn(n).getAttribute('title')).toBe(n)
      expect(btn(n).querySelector('.desktop-bar__label')?.textContent).toBe(n)
    }
    const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf8')
    expect(css).toMatch(/@media \(max-width: 879px\)\s*\{\s*\.desktop-bar__item[\s\S]*?\.desktop-bar__label\s*\{\s*display: none/)
  })

  it('Swap is a toggle: aria-pressed, a pressed look, and it reports the new value', () => {
    const p = render(props())
    const swap = btn('Swap Ctrl and Cmd')
    expect(swap.getAttribute('aria-pressed')).toBe('false')
    act(() => swap.click())
    expect(p.onSwapChange).toHaveBeenCalledWith(true)
    render({ ...p, swapOn: true })
    expect(btn('Swap Ctrl and Cmd').getAttribute('aria-pressed')).toBe('true')
    expect(btn('Swap Ctrl and Cmd').classList.contains('is-on')).toBe(true)
  })

  it('View Only and Control are a two-radio group; choosing one reports it; the Control tooltip carries the release chord', () => {
    const p = render(props())
    expect(radio('Control').getAttribute('aria-checked')).toBe('true')
    expect(radio('View Only').getAttribute('aria-checked')).toBe('false')
    expect(radio('Control').getAttribute('title')).toBe(
      'Control: click the picture to type. To give the keyboard back, press Ctrl+Option+Cmd+Return.',
    )
    act(() => radio('View Only').click())
    expect(p.onModeChange).toHaveBeenCalledWith('view')
  })

  it('arrow keys move between the segments', () => {
    const p = render(props())
    act(() => radio('Control').focus())
    act(() => { radio('Control').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true })) })
    expect(p.onModeChange).toHaveBeenCalledWith('view')
  })

  it('locked greys both segments; needs Accessibility greys only Control and View Only stays selectable', () => {
    let p = render(props({ state: { locked: true } }))
    expect(radio('View Only').getAttribute('aria-disabled')).toBe('true')
    expect(radio('Control').getAttribute('aria-disabled')).toBe('true')
    act(() => radio('View Only').click())
    expect(p.onModeChange).not.toHaveBeenCalled()
    p = render(props({ state: { accessNeeded: true, mode: 'view' } }))
    expect(radio('Control').getAttribute('aria-disabled')).toBe('true')
    expect(radio('Control').getAttribute('title')).toBe('Control needs Accessibility')
    expect(radio('View Only').getAttribute('aria-disabled')).toBeNull()
    act(() => radio('Control').click())
    expect(p.onModeChange).not.toHaveBeenCalled()
  })

  it('Send Keys is greyed in View Only with Turn on Control first', () => {
    render(props({ state: { mode: 'view' } }))
    expect(btn('Send Keys').getAttribute('title')).toBe('Turn on Control first')
    expect(btn('Send Keys').getAttribute('aria-disabled')).toBe('true')
  })

  it('Full Screen toggles its name, calls the view, and is absent when fullscreen is unavailable', () => {
    const p = render(props())
    act(() => btn('Full Screen').click())
    expect(p.onFullscreen).toHaveBeenCalledTimes(1)
    render({ ...p, fullscreen: true })
    expect(btn('Exit Full Screen')).not.toBeNull()
    render(props({ state: { fullscreenAvailable: false } }))
    expect(container.querySelector('button[aria-label$="Full Screen"]')).toBeNull()
  })

  it('the picker is not offered without the surface right', () => {
    render(props({ state: { canSurface: false } }))
    expect(container.querySelector('.desktop-bar__picker')).toBeNull()
    expect(btn('Send Keys')).not.toBeNull()
  })

  it('in a window share the three Send Keys items are greyed with the caption', () => {
    render(props({ surface: { kind: 'window', id: 3, name: 'Terminal — zsh' } }))
    act(() => btn('Send Keys').click())
    const items = Array.from(document.querySelectorAll('[role="menuitem"]'))
    expect(items).toHaveLength(3)
    expect(items.every((i) => i.getAttribute('aria-disabled') === 'true')).toBe(true)
    expect(document.querySelector('.desktop-menu__caption')?.textContent).toBe(
      'Not available in a window share. Share the display to use these.',
    )
  })

  it('the style sheet has no transition on the bar under reduced motion', () => {
    const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf8')
    expect(css).toMatch(/@media \(prefers-reduced-motion: reduce\)\s*\{\s*\.desktop-bar\s*\{\s*transition: none/)
  })
})

describe('Clipboard menu (Phase 20-08 Task 1)', () => {
  const clipBtn = () => container.querySelector<HTMLButtonElement>('button[aria-label="Clipboard"]')
  const items = () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]'))
  const item = (t: string) => items().find((i) => i.textContent === t)!
  const CAPTION = "Cmd+C and Cmd+V in the picture use the Mac's own clipboard. Use these to move text between the two."

  it('sits after Send Keys and offers Copy from Mac, Paste to Mac and the caption', () => {
    render(props())
    const labels = Array.from(container.querySelectorAll('.desktop-bar__item')).map((b) => b.getAttribute('aria-label'))
    expect(labels.indexOf('Clipboard')).toBe(labels.indexOf('Send Keys') + 1)
    act(() => clipBtn()!.click())
    expect(items().map((i) => i.textContent)).toEqual(['Copy from Mac', 'Paste to Mac'])
    expect(caption()).toBe(CAPTION)
    expect(clipBtn()!.getAttribute('aria-haspopup')).toBe('menu')
    expect(clipBtn()!.getAttribute('aria-expanded')).toBe('true')
  })

  it('reads and writes nothing until an item is chosen, and an item runs its handler once, in the click', () => {
    const p = render(props())
    act(() => clipBtn()!.click())
    expect(p.onCopy).not.toHaveBeenCalled()
    expect(p.onPaste).not.toHaveBeenCalled()
    act(() => item('Copy from Mac').click())
    expect(p.onCopy).toHaveBeenCalledTimes(1)
    expect(p.onPaste).not.toHaveBeenCalled()
    expect(menu()).toBeNull()
    act(() => clipBtn()!.click())
    act(() => item('Paste to Mac').click())
    expect(p.onPaste).toHaveBeenCalledTimes(1)
  })

  it('tells the view while its menu is open so everything held is released', () => {
    const p = render(props())
    act(() => clipBtn()!.click())
    expect(p.onMenuOpenChange).toHaveBeenLastCalledWith(true)
    act(() => item('Copy from Mac').click())
    expect(p.onMenuOpenChange).toHaveBeenLastCalledWith(false)
  })

  it('greys both items while a transfer is in flight, and they do nothing', () => {
    const p = render(props({ clipBusy: true }))
    act(() => clipBtn()!.click())
    expect(items().map((i) => i.getAttribute('aria-disabled'))).toEqual(['true', 'true'])
    act(() => item('Copy from Mac').click())
    act(() => item('Paste to Mac').click())
    expect(p.onCopy).not.toHaveBeenCalled()
    expect(p.onPaste).not.toHaveBeenCalled()
  })

  it.each([
    ['locked', { locked: true }, 'Locked'],
    ['reconnecting', { reconnecting: true }, 'Reconnecting'],
  ])('is greyed while %s, with the reason as its title and as text, and opens no menu', (_n, state, reason) => {
    render(props({ state }))
    expect(clipBtn()!.getAttribute('aria-disabled')).toBe('true')
    expect(clipBtn()!.getAttribute('title')).toBe(reason)
    const describedBy = clipBtn()!.getAttribute('aria-describedby')!
    expect(document.getElementById(describedBy)!.textContent).toBe(reason)
    act(() => clipBtn()!.click())
    expect(menu()).toBeNull()
  })

  it('is not offered to a viewer without the clipboard right', () => {
    render(props({ state: { canClipboard: false } }))
    expect(clipBtn()).toBeNull()
  })

  it('stays available in a window share and while the Mac needs Accessibility (it does not inject input)', () => {
    render(props({ surface: { kind: 'window', id: 7, name: 'Terminal — zsh' }, state: { accessNeeded: true } }))
    expect(clipBtn()!.getAttribute('aria-disabled')).toBeNull()
  })
})
