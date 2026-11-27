// Phase 22-07 — the guest's bar shows only what the grants allow (22-UI-SPEC Surface 6, "What the guest can do, by grant").

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { DesktopControlBar, barAvailability, type BarState, type DesktopControlBarProps } from '../DesktopControlBar'
import { DesktopSendKeysMenu } from '../DesktopSendKeysMenu'

let container: HTMLDivElement
let root: Root

const STREAMING: BarState = {
  locked: false, reconnecting: false, accessNeeded: false, switching: false, mode: 'control',
  canSurface: false, fullscreenAvailable: true, canClipboard: false,
}

function props(over: Partial<Omit<DesktopControlBarProps, 'state'>> & { state?: Partial<BarState> } = {}): DesktopControlBarProps {
  const { state, ...rest } = over
  return {
    state: { ...STREAMING, ...state },
    sendChord: vi.fn(),
    onMenuOpenChange: vi.fn(),
    onSendKeysDone: vi.fn(),
    surface: { kind: 'display', id: 1, name: 'Main Display' },
    requestSurfaces: vi.fn(() => Promise.resolve({ displays: [], windows: [] })),
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

const render = (p: DesktopControlBarProps) => act(() => root.render(<DesktopControlBar {...p} />))
const btn = (name: string) => container.querySelector<HTMLButtonElement>(`button[aria-label="${name}"]`)
/** The bar's items in order: button labels, with the segment as one entry. */
const barItems = () =>
  Array.from(container.querySelector('.desktop-bar')!.children)
    .filter((e) => !e.classList.contains('desktop-bar__divider') && !e.classList.contains('visually-hidden'))
    .map((e) => (e.getAttribute('role') === 'radiogroup' ? 'segment' : e.getAttribute('aria-label')))
const menuItems = () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).map((e) => e.querySelector('.desktop-menu__text')?.textContent)

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

describe('guest bar by grant', () => {
  it('control only: Send Keys, Swap, the segment and Full Screen; no picker and no Clipboard menu', () => {
    render(props({ guest: true, state: { canControl: true, canSurface: true } }))
    expect(barItems()).toEqual(['Send Keys', 'Swap Ctrl and Cmd', 'segment', 'Full Screen'])
    expect(container.querySelector('.desktop-bar__picker')).toBeNull()
  })

  it('clipboard only: the bar holds the Clipboard menu alone', () => {
    render(props({ guest: true, state: { canControl: false, canClipboard: true } }))
    expect(barItems()).toEqual(['Clipboard'])
    expect(container.querySelector('[role="radiogroup"]')).toBeNull()
    expect(container.querySelectorAll('[role="separator"]')).toHaveLength(0)
  })

  it('control and clipboard: both sets, still no picker', () => {
    render(props({ guest: true, state: { canControl: true, canClipboard: true, canSurface: true } }))
    expect(barItems()).toEqual(['Send Keys', 'Clipboard', 'Swap Ctrl and Cmd', 'segment', 'Full Screen'])
  })

  it('no grants: availability hides every item, so there is nothing to render', () => {
    const a = barAvailability({ ...STREAMING, guest: true, canControl: false })
    for (const k of Object.keys(a) as (keyof typeof a)[]) expect(a[k].hidden, k).toBe(true)
  })

  it('Clipboard opens Copy from Mac and Paste to Mac', async () => {
    render(props({ guest: true, state: { canControl: false, canClipboard: true } }))
    await act(async () => btn('Clipboard')!.click())
    expect(Array.from(document.querySelectorAll('[role="menuitem"]')).map((e) => e.textContent)).toEqual(
      expect.arrayContaining([expect.stringContaining('Copy from Mac'), expect.stringContaining('Paste to Mac')]),
    )
  })

  it('Send Keys on a guest bar has Cmd+Tab and Cmd+Space and no Lock Screen…', async () => {
    const p = props({ guest: true, state: { canControl: true } })
    render(p)
    await act(async () => btn('Send Keys')!.click())
    expect(menuItems()).toEqual(['Cmd+Tab', 'Cmd+Space'])
    expect(document.querySelector('.desktop-menu__hint')).toBeNull()
  })

  it('a guest who chooses View Only keeps the segment and gets Send Keys greyed with its reason', () => {
    render(props({ guest: true, state: { canControl: true, mode: 'view' } }))
    expect(btn('Send Keys')!.getAttribute('title')).toBe('Turn on Control first')
    expect(container.querySelector('[role="radiogroup"]')).not.toBeNull()
  })
})

describe('own-account bar is unchanged', () => {
  it('keeps the picker and, in Send Keys, Lock Screen…', async () => {
    render(props({ state: { canControl: true, canSurface: true, canClipboard: true } }))
    expect(barItems()).toEqual(['Main Display', 'Send Keys', 'Clipboard', 'Swap Ctrl and Cmd', 'segment', 'Full Screen'])
    await act(async () => btn('Send Keys')!.click())
    expect(menuItems()).toEqual(['Cmd+Tab', 'Cmd+Space', 'Lock Screen…'])
  })

  it('a bar state without canControl or guest reads as before', () => {
    const a = barAvailability({ ...STREAMING, canSurface: true, canClipboard: true })
    for (const k of Object.keys(a) as (keyof typeof a)[]) expect(a[k].hidden, k).toBe(false)
  })
})

describe('DesktopSendKeysMenu guest prop', () => {
  const anchor = { x: 10, y: 10 }
  it('guest drops Lock Screen… and its separator; the default keeps it', () => {
    for (const [guest, want] of [[true, ['Cmd+Tab', 'Cmd+Space']], [false, ['Cmd+Tab', 'Cmd+Space', 'Lock Screen…']]] as const) {
      act(() => root.render(<DesktopSendKeysMenu anchor={anchor} invoker={null} onClose={() => {}} sendChord={() => {}} windowShare={false} guest={guest} />))
      expect(menuItems()).toEqual(want)
      act(() => root.render(<div />))
    }
  })
})
