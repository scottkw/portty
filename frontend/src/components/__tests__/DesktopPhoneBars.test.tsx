// Phase 21-10 (DVWR-03) — the phone's bottom bar and key bar (21-UI-SPEC Surface 4).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import type { DesktopClient } from '../../lib/desktopClient'
import { DesktopPhoneBars, type DesktopPhoneBarsProps, type PhoneModifier } from '../DesktopPhoneBars'

;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

let container: HTMLDivElement
let root: Root

function props(over: Partial<DesktopPhoneBarsProps> = {}): DesktopPhoneBarsProps {
  return {
    keyboardUp: false,
    zoom: 1,
    onFit: vi.fn(),
    onShowKeyboard: vi.fn(),
    onKey: vi.fn(),
    armed: new Set<PhoneModifier>(),
    onToggleArm: vi.fn(),
    sendChord: vi.fn(),
    onSelectSurface: vi.fn(),
    surfaceName: 'Main Display',
    surface: { kind: 'display', id: 1 },
    canControl: true,
    canPickSurface: true,
    disabledReason: null,
    client: {
      requestSurfaces: vi.fn(() =>
        Promise.resolve({
          displays: [
            { id: 1, name: 'Main Display', w: 1470, h: 956, main: true },
            { id: 2, name: 'Display 2', w: 1920, h: 1080, main: false },
          ],
          windows: [{ id: 4242, app: 'Terminal', title: 'zsh', name: 'Terminal — zsh' }],
        }),
      ),
      selectSurface: vi.fn(),
    } as unknown as DesktopClient,
    ...over,
  }
}

const render = (p: DesktopPhoneBarsProps) => act(() => root.render(<DesktopPhoneBars {...p} />))
const toolbar = () => container.querySelector('[role="toolbar"]') as HTMLElement
const btn = (label: string) =>
  Array.from(container.querySelectorAll<HTMLButtonElement>('button')).find(
    (b) => b.getAttribute('aria-label') === label || b.textContent === label,
  )!
const labels = () => Array.from(toolbar().querySelectorAll('button')).map((b) => b.getAttribute('aria-label') ?? b.textContent)

function press(el: HTMLElement) {
  const ev = new MouseEvent('pointerdown', { bubbles: true, cancelable: true })
  act(() => {
    el.dispatchEvent(ev)
  })
  return ev
}
const click = (el: HTMLElement, detail: number) =>
  act(() => {
    el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, detail }))
  })

beforeEach(() => {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})
afterEach(() => {
  act(() => root.unmount())
  container.remove()
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('bottom bar (keyboard down)', () => {
  it('is the Desktop controls toolbar with the surface, Send Keys and Show Keyboard, and no Fit at 1x', () => {
    render(props())
    expect(toolbar().getAttribute('aria-label')).toBe('Desktop controls')
    expect(labels()).toEqual(['Main Display', 'Send Keys', 'Show Keyboard'])
    expect(btn('Main Display').getAttribute('aria-haspopup')).toBe('menu')
    expect(btn('Send Keys').getAttribute('aria-haspopup')).toBe('menu')
  })

  it('Fit to Screen shows only above 1x and its label names the zoom; it fires through the firing rule', () => {
    const p = props({ zoom: 2.5 })
    render(p)
    const fit = btn('Fit to screen, zoom 2.5x')
    expect(fit.textContent).toBe('Fit to Screen')
    press(fit)
    expect(p.onFit).toHaveBeenCalledTimes(1)
    render(props({ zoom: 3 }))
    expect(btn('Fit to screen, zoom 3x')).toBeTruthy()
  })

  it('without the control right Send Keys and Show Keyboard are hidden; without the surface right the picker is', () => {
    render(props({ canControl: false }))
    expect(labels()).toEqual(['Main Display'])
    render(props({ canControl: false, canPickSurface: false, zoom: 2 }))
    expect(labels()).toEqual(['Fit to screen, zoom 2x'])
  })

  it.each(['Locked', 'Reconnecting'] as const)('%s: the sending items are aria-disabled and do nothing; Fit stays live; Show Keyboard is greyed only while Locked', (reason) => {
    const p = props({ disabledReason: reason, zoom: 2 })
    render(p)
    for (const name of ['Main Display', 'Send Keys']) {
      expect(btn(name).getAttribute('aria-disabled')).toBe('true')
      press(btn(name))
    }
    expect(document.querySelector('[role="menu"]')).toBeNull()
    // 23-08: Show Keyboard is greyed while Locked (the Unlock field has its own keyboard) and stays live while Reconnecting.
    expect(btn('Show Keyboard').hasAttribute('aria-disabled')).toBe(reason === 'Locked')
    click(btn('Show Keyboard'), 1)
    expect(p.onShowKeyboard).toHaveBeenCalledTimes(reason === 'Locked' ? 0 : 1)
    press(btn('Fit to screen, zoom 2x'))
    expect(p.onFit).toHaveBeenCalledTimes(1)
  })

  it('Show Keyboard acts in its click, not on pointerdown', () => {
    const p = props()
    render(p)
    press(btn('Show Keyboard'))
    expect(p.onShowKeyboard).not.toHaveBeenCalled()
    click(btn('Show Keyboard'), 1)
    expect(p.onShowKeyboard).toHaveBeenCalledTimes(1)
  })

  it('a long surface name keeps its full name in the aria-label and truncates only the visible text', () => {
    const long = 'Terminal — a very long window title that cannot fit'
    render(props({ surfaceName: long }))
    expect(btn(long).querySelector('.desktop-phone__name')?.textContent).toBe(long)
  })
})

describe('key bar (keyboard up)', () => {
  it('is the Desktop keys toolbar with the ten keys in order, in one row', () => {
    render(props({ keyboardUp: true }))
    expect(toolbar().getAttribute('aria-label')).toBe('Desktop keys')
    expect(labels()).toEqual(['Esc', 'Tab', 'Ctrl', 'Opt', 'Cmd', 'Left arrow', 'Up arrow', 'Down arrow', 'Right arrow', 'Send Keys'])
    expect(btn('Send Keys').getAttribute('aria-haspopup')).toBe('menu')
    expect(container.querySelector('.keybar--kb-up')).not.toBeNull()
    expect(toolbar().querySelectorAll('button')).toHaveLength(10)
  })

  it('pointerdown prevents default and fires once; a click with detail 0 fires once; the touch click after a pointerdown does not fire again', () => {
    const p = props({ keyboardUp: true })
    render(p)
    const esc = btn('Esc')
    expect(press(esc).defaultPrevented).toBe(true)
    expect(p.onKey).toHaveBeenCalledTimes(1)
    expect(p.onKey).toHaveBeenLastCalledWith('Escape')
    click(esc, 1)
    expect(p.onKey).toHaveBeenCalledTimes(1)
    click(esc, 0)
    expect(p.onKey).toHaveBeenCalledTimes(2)
  })

  it('each key sends its code', () => {
    const p = props({ keyboardUp: true })
    render(p)
    for (const name of ['Tab', 'Left arrow', 'Up arrow', 'Down arrow', 'Right arrow']) press(btn(name))
    expect((p.onKey as ReturnType<typeof vi.fn>).mock.calls.map((c) => c[0])).toEqual(['Tab', 'ArrowLeft', 'ArrowUp', 'ArrowDown', 'ArrowRight'])
  })

  it('Ctrl, Opt and Cmd arm and disarm by tap, show aria-pressed and the armed class, and several can be armed', () => {
    const p = props({ keyboardUp: true, armed: new Set<PhoneModifier>(['Control', 'Meta']) })
    render(p)
    expect(btn('Ctrl').getAttribute('aria-pressed')).toBe('true')
    expect(btn('Cmd').getAttribute('aria-pressed')).toBe('true')
    expect(btn('Opt').getAttribute('aria-pressed')).toBe('false')
    expect(btn('Ctrl').className).toContain('keybar__key--armed')
    press(btn('Opt'))
    press(btn('Ctrl'))
    expect((p.onToggleArm as ReturnType<typeof vi.fn>).mock.calls.map((c) => c[0])).toEqual(['Alt', 'Control'])
    expect(p.onKey).not.toHaveBeenCalled()
  })

  it.each(['Locked', 'Reconnecting'] as const)('%s: every key is aria-disabled and sends nothing', (reason) => {
    const p = props({ keyboardUp: true, disabledReason: reason })
    render(p)
    for (const b of Array.from(toolbar().querySelectorAll('button'))) {
      expect(b.getAttribute('aria-disabled')).toBe('true')
      press(b)
      click(b, 0)
    }
    expect(p.onKey).not.toHaveBeenCalled()
    expect(p.onToggleArm).not.toHaveBeenCalled()
  })
})

describe('menus open upward', () => {
  const menuItem = (t: string) =>
    Array.from(document.querySelectorAll<HTMLElement>('[role^="menuitem"]')).find((i) => i.textContent?.startsWith(t))!

  function rects(buttonTop: number) {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      const isMenu = this.getAttribute('role') === 'menu'
      return { left: 10, top: isMenu ? 0 : buttonTop, width: 200, height: isMenu ? 120 : 44, right: 0, bottom: 0, x: 0, y: 0, toJSON() {} } as DOMRect
    })
  }

  it('Send Keys opens the Phase 20 menu above the bar and Cmd+Tab goes through sendChord', () => {
    rects(300)
    const p = props()
    render(p)
    press(btn('Send Keys'))
    const menu = document.querySelector('[role="menu"]') as HTMLElement
    expect(menu).not.toBeNull()
    expect(menu.style.top).toBe('180px') // anchored at the bar's top (300), opened upward by the menu's height (120)
    act(() => menuItem('Cmd+Tab').click())
    expect(p.sendChord).toHaveBeenCalledWith(['MetaLeft', 'Tab'])
  })

  it('the key bar opens the same menu from its icon-only Send Keys', () => {
    rects(300)
    const p = props({ keyboardUp: true })
    render(p)
    press(btn('Send Keys'))
    expect(document.querySelector('[role="menu"]')).not.toBeNull()
    act(() => menuItem('Cmd+Space').click())
    expect(p.sendChord).toHaveBeenCalledWith(['MetaLeft', 'Space'])
  })

  it('a guest phone gets no Lock Screen… in Send Keys; an own-account phone does', () => {
    rects(300)
    render(props({ guest: true }))
    press(btn('Send Keys'))
    expect(menuItem('Cmd+Tab')).toBeDefined()
    expect(menuItem('Lock Screen')).toBeUndefined()
    act(() => root.unmount())
    root = createRoot(container)
    render(props())
    press(btn('Send Keys'))
    expect(menuItem('Lock Screen')).toBeDefined()
  })

  it('in a window share the Send Keys items are inert', () => {
    rects(300)
    const p = props({ surface: { kind: 'window', id: 7 } })
    render(p)
    press(btn('Send Keys'))
    act(() => menuItem('Cmd+Tab').click())
    expect(p.sendChord).not.toHaveBeenCalled()
  })

  it("the picker lists displays and windows as radio items above the bar, checks the current one, and selects through the view's pick handler", async () => {
    rects(300)
    const p = props()
    render(p)
    await act(async () => {
      press(btn('Main Display'))
    })
    const menu = document.querySelector('[role="menu"]') as HTMLElement
    expect(menu.style.top).toBe('180px')
    expect(menuItem('Main Display').getAttribute('aria-checked')).toBe('true')
    expect(menuItem('Terminal — zsh').getAttribute('aria-checked')).toBe('false')
    expect(menu.textContent).toContain('Displays')
    expect(menu.textContent).toContain('Windows')
    act(() => menuItem('Terminal — zsh').click())
    expect(p.onSelectSurface).toHaveBeenCalledWith({ kind: 'window', id: 4242, name: 'Terminal — zsh' })
    expect(p.client.selectSurface).not.toHaveBeenCalled()
  })

  it('the picker says so when the list cannot be loaded', async () => {
    rects(300)
    const p = props()
    ;(p.client.requestSurfaces as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('no'))
    render(p)
    await act(async () => {
      press(btn('Main Display'))
    })
    expect(document.querySelector('[role="menu"]')?.textContent).toContain("Couldn't load the list")
  })
})
