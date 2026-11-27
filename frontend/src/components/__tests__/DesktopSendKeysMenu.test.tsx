// Phase 20-07 — the standalone Send Keys menu and its Lock This Mac confirm (DCTL-03).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { createRoot, type Root } from 'react-dom/client'
import { createDesktopInput, type InputMsg } from '../../lib/desktopInput'
import { DesktopSendKeysMenu, type DesktopSendKeysMenuProps } from '../DesktopSendKeysMenu'

let container: HTMLDivElement
let root: Root
let invoker: HTMLButtonElement

function open(over: Partial<DesktopSendKeysMenuProps> = {}) {
  const props: DesktopSendKeysMenuProps = {
    anchor: { x: 10, y: 20 },
    invoker,
    onClose: vi.fn(),
    sendChord: vi.fn(),
    windowShare: false,
    ...over,
  }
  act(() => root.render(<DesktopSendKeysMenu {...props} />))
  return props
}

const menu = () => document.querySelector<HTMLElement>('[role="menu"]')
const items = () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]'))
const byLabel = (t: string) => items().find((i) => i.textContent?.startsWith(t))!
const press = (k: string) =>
  act(() => {
    document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }))
  })
const click = (el: HTMLElement) => act(() => el.click())

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  invoker = document.createElement('button')
  document.body.appendChild(invoker)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  invoker.remove()
})

describe('Send Keys items (Task 1)', () => {
  it('offers exactly Cmd+Tab, Cmd+Space and Lock Screen… with the Ctrl+Cmd+Q hint, in a role=menu', () => {
    open()
    expect(menu()).not.toBeNull()
    expect(menu()!.className).toContain('desktop-menu')
    expect(items().map((i) => i.querySelector('.desktop-menu__text')?.textContent)).toEqual([
      'Cmd+Tab', 'Cmd+Space', 'Lock Screen…',
    ])
    expect(byLabel('Lock Screen…').querySelector('.desktop-menu__hint')?.textContent).toBe('Ctrl+Cmd+Q')
    expect(menu()!.querySelectorAll('[role="separator"]')).toHaveLength(1)
  })

  it('choosing Cmd+Tab taps MetaLeft and Tab once and closes', () => {
    const p = open()
    click(byLabel('Cmd+Tab'))
    expect(p.sendChord).toHaveBeenCalledTimes(1)
    expect(p.sendChord).toHaveBeenCalledWith(['MetaLeft', 'Tab'])
    expect(p.onClose).toHaveBeenCalledTimes(1)
    expect(menu()).toBeNull()
  })

  it('choosing Cmd+Space taps MetaLeft and Space', () => {
    const p = open()
    click(byLabel('Cmd+Space'))
    expect(p.sendChord).toHaveBeenCalledWith(['MetaLeft', 'Space'])
  })

  it('arrow keys step over the separator and wrap', () => {
    open()
    expect(document.activeElement).toBe(byLabel('Cmd+Tab'))
    press('ArrowDown')
    expect(document.activeElement).toBe(byLabel('Cmd+Space'))
    press('ArrowDown')
    expect(document.activeElement).toBe(byLabel('Lock Screen…'))
    press('ArrowDown')
    expect(document.activeElement).toBe(byLabel('Cmd+Tab'))
    press('End')
    expect(document.activeElement).toBe(byLabel('Lock Screen…'))
    press('Home')
    expect(document.activeElement).toBe(byLabel('Cmd+Tab'))
  })

  it('Esc closes without sending and focuses the invoker', () => {
    const p = open()
    press('Escape')
    expect(p.sendChord).not.toHaveBeenCalled()
    expect(p.onClose).toHaveBeenCalledTimes(1)
    expect(document.activeElement).toBe(invoker)
  })

  it('through a real input controller the tap is key MetaLeft down m 8, Tab down m 8, Tab up m 8, MetaLeft up m 0', () => {
    const stage = document.createElement('div')
    const input = document.createElement('textarea')
    const sent: InputMsg[] = []
    const ctl = createDesktopInput(
      stage,
      input,
      {
        send: (m) => { sent.push(m); return true },
        getRect: () => ({ x: 0, y: 0, width: 100, height: 100 }),
        getEpoch: () => 1,
        storage: null,
      },
      {},
    )
    ctl.setControl(true)
    sent.length = 0
    // Swap on: the menu names Cmd explicitly, so the swap must not apply.
    ctl.setSwap(true)
    open({ sendChord: (codes) => ctl.sendChord(codes) })
    click(byLabel('Cmd+Tab'))
    expect(sent.map((m) => (m.t === 'key' ? [m.code, m.d, m.m] : m.t))).toEqual([
      ['MetaLeft', true, 8],
      ['Tab', true, 8],
      ['Tab', false, 8],
      ['MetaLeft', false, 0],
    ])
    ctl.destroy()
  })
})

describe('Lock Screen confirm and window share (Task 3)', () => {
  const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]')
  const dlgButton = (name: string) => Array.from(dialog()!.querySelectorAll('button')).find((b) => b.textContent === name)!

  it('Lock Screen… opens the Lock This Mac dialog with Don\'t Lock focused, and sends nothing yet', () => {
    const p = open()
    click(byLabel('Lock Screen…'))
    expect(dialog()).not.toBeNull()
    expect(dialog()!.getAttribute('aria-modal')).toBe('true')
    expect(dialog()!.querySelector('.dialog__title')?.textContent).toBe('Lock This Mac')
    expect(dialog()!.textContent).toContain(
      "The Mac will lock and stay locked until someone unlocks it at the Mac. Remote unlock isn't available yet.",
    )
    expect(dlgButton("Don't Lock")).toBeDefined()
    expect(dlgButton('Lock Screen')).toBeDefined()
    expect(document.activeElement).toBe(dlgButton("Don't Lock"))
    expect(p.sendChord).not.toHaveBeenCalled()
    expect(p.onClose).not.toHaveBeenCalled()
    expect(menu()).toBeNull()
  })

  it('Esc does what Don\'t Lock does: closes with nothing sent', () => {
    const p = open()
    click(byLabel('Lock Screen…'))
    press('Escape')
    expect(dialog()).toBeNull()
    expect(p.sendChord).not.toHaveBeenCalled()
    expect(p.onClose).toHaveBeenCalledTimes(1)
  })

  it('Don\'t Lock closes with nothing sent', () => {
    const p = open()
    click(byLabel('Lock Screen…'))
    click(dlgButton("Don't Lock"))
    expect(dialog()).toBeNull()
    expect(p.sendChord).not.toHaveBeenCalled()
    expect(p.onClose).toHaveBeenCalledTimes(1)
  })

  it('only the Lock Screen button sends the Ctrl+Cmd+Q tap, once, and closes', async () => {
    const p = open()
    click(byLabel('Lock Screen…'))
    await act(async () => { dlgButton('Lock Screen').click() })
    expect(p.sendChord).toHaveBeenCalledTimes(1)
    expect(p.sendChord).toHaveBeenCalledWith(['ControlLeft', 'MetaLeft', 'KeyQ'])
    expect(p.onClose).toHaveBeenCalledTimes(1)
    expect(dialog()).toBeNull()
  })

  it('in a window share all three items are greyed with the caption and the dialog cannot open', () => {
    const p = open({ windowShare: true })
    expect(items().every((i) => i.getAttribute('aria-disabled') === 'true')).toBe(true)
    expect(document.querySelector('.desktop-menu__caption')?.textContent).toBe(
      'Not available in a window share. Share the display to use these.',
    )
    click(byLabel('Lock Screen…'))
    click(byLabel('Cmd+Tab'))
    expect(dialog()).toBeNull()
    expect(p.sendChord).not.toHaveBeenCalled()
  })

  it('a window share that starts while the dialog is open still never sends the chord', async () => {
    const sendChord = vi.fn()
    open({ sendChord })
    click(byLabel('Lock Screen…'))
    open({ sendChord, windowShare: true })
    await act(async () => { dlgButton('Lock Screen').click() })
    expect(sendChord).not.toHaveBeenCalled()
  })

  it('a disabledReason (Locked) greys the items with that title', () => {
    const p = open({ disabledReason: 'Locked' })
    expect(items().every((i) => i.getAttribute('aria-disabled') === 'true' && i.getAttribute('title') === 'Locked')).toBe(true)
    click(byLabel('Cmd+Space'))
    expect(p.sendChord).not.toHaveBeenCalled()
  })

  it('renders inside the full-screen element when there is one', () => {
    const fs = document.createElement('div')
    document.body.appendChild(fs)
    Object.defineProperty(document, 'fullscreenElement', { configurable: true, get: () => fs })
    try {
      open()
      expect(fs.contains(menu())).toBe(true)
      click(byLabel('Lock Screen…'))
      expect(fs.contains(dialog())).toBe(true)
    } finally {
      delete (document as unknown as { fullscreenElement?: unknown }).fullscreenElement
      act(() => root.unmount())
      fs.remove()
      root = createRoot(container)
    }
  })

  it('the component adds no markup injection path', () => {
    const src = readFileSync(resolve(__dirname, '../DesktopSendKeysMenu.tsx'), 'utf8')
    expect(src).not.toContain('dangerouslySetInnerHTML')
  })
})
