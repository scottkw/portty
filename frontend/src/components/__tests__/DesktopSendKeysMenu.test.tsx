// Phase 20-07 — the standalone Send Keys menu and its Lock This Mac confirm (DCTL-03).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
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
