import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { ContextMenu, isMenuKey } from '../ContextMenu'

// Portty 02-08 (TERM-03): the NSMenu-style menu shared by tabs and sidebar rows.

let container: HTMLDivElement
let root: Root
let invoker: HTMLButtonElement

function open(onClose = vi.fn(), onRename = vi.fn(), onEnd = vi.fn()) {
  invoker = document.createElement('button')
  document.body.appendChild(invoker)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  flushSync(() =>
    root.render(
      React.createElement(ContextMenu, {
        x: 10,
        y: 20,
        invoker,
        onClose,
        items: [
          { label: 'Rename', onSelect: onRename },
          { label: 'Close Tab', onSelect: vi.fn() },
          { label: 'End Session…', onSelect: onEnd, separatorBefore: true },
        ],
      }),
    ),
  )
  return { onClose, onRename, onEnd }
}

const menu = () => document.querySelector<HTMLElement>('[role="menu"]')!
const key = (k: string) => flushSync(() => document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true })))

afterEach(() => {
  flushSync(() => root.unmount())
  container.remove()
  invoker.remove()
})

describe('ContextMenu', () => {
  it('renders role=menu with menuitems and a separator at the given point, first item focused', () => {
    open()
    expect(menu().querySelectorAll('[role="menuitem"]')).toHaveLength(3)
    expect(menu().querySelector('[role="separator"]')?.nextElementSibling?.textContent).toBe('End Session…')
    expect(menu().style.left).toBe('10px')
    expect(menu().style.top).toBe('20px')
    expect(document.activeElement?.textContent).toBe('Rename')
  })

  it('ArrowDown / ArrowUp move focus with wraparound', () => {
    open()
    key('ArrowDown')
    expect(document.activeElement?.textContent).toBe('Close Tab')
    key('ArrowDown')
    key('ArrowDown')
    expect(document.activeElement?.textContent).toBe('Rename')
    key('ArrowUp')
    expect(document.activeElement?.textContent).toBe('End Session…')
  })

  it('Enter activates the focused item once, closes, and returns focus to the invoker', () => {
    const { onClose, onEnd } = open()
    key('ArrowUp')
    key('Enter')
    expect(onEnd).toHaveBeenCalledTimes(1)
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(document.activeElement).toBe(invoker)
  })

  it('Escape closes and returns focus to the invoker without selecting', () => {
    const { onClose, onRename } = open()
    key('Escape')
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onRename).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(invoker)
  })

  it('an outside mousedown closes; a mousedown inside does not', () => {
    const { onClose } = open()
    flushSync(() => menu().dispatchEvent(new MouseEvent('mousedown', { bubbles: true })))
    expect(onClose).not.toHaveBeenCalled()
    flushSync(() => document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true })))
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})

// Portty 04-05: a disabled item (Share while Tailscale is off).
describe('ContextMenu disabled items', () => {
  function openWithDisabled(onShare = vi.fn()) {
    invoker = document.createElement('button')
    document.body.appendChild(invoker)
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() =>
      root.render(
        React.createElement(ContextMenu, {
          x: 10,
          y: 20,
          invoker,
          onClose: vi.fn(),
          items: [
            { label: 'Rename', onSelect: vi.fn() },
            { label: 'Share', onSelect: onShare, disabled: true, describedBy: 'ts-off-reason' },
            { label: 'End Session…', onSelect: vi.fn(), separatorBefore: true },
          ],
        }),
      ),
    )
    return Array.from(menu().querySelectorAll<HTMLButtonElement>('[role="menuitem"]')).find((b) => b.textContent === 'Share')!
  }

  it('renders aria-disabled and aria-describedby, and is not natively disabled', () => {
    const share = openWithDisabled()
    expect(share.getAttribute('aria-disabled')).toBe('true')
    expect(share.getAttribute('aria-describedby')).toBe('ts-off-reason')
    expect(share.disabled).toBe(false)
  })

  it('is skipped by ArrowDown / ArrowUp', () => {
    openWithDisabled()
    expect(document.activeElement?.textContent).toBe('Rename')
    key('ArrowDown')
    expect(document.activeElement?.textContent).toBe('End Session…')
    key('ArrowUp')
    expect(document.activeElement?.textContent).toBe('Rename')
  })

  it('ignores click and hover', () => {
    const onShare = vi.fn()
    const share = openWithDisabled(onShare)
    flushSync(() => share.dispatchEvent(new MouseEvent('mouseover', { bubbles: true })))
    expect(document.activeElement).not.toBe(share)
    flushSync(() => share.click())
    expect(onShare).not.toHaveBeenCalled()
    expect(menu()).not.toBeNull()
  })

  it('enabled items carry neither attribute', () => {
    openWithDisabled()
    const rename = menu().querySelector('[role="menuitem"]')!
    expect(rename.hasAttribute('aria-disabled')).toBe(false)
    expect(rename.hasAttribute('aria-describedby')).toBe(false)
  })
})

// Phase 20-07: the viewer's bar menus extend the menu with checked items, hints, label and caption rows.
describe('ContextMenu extended entries', () => {
  function openExtended() {
    invoker = document.createElement('button')
    document.body.appendChild(invoker)
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    flushSync(() =>
      root.render(
        React.createElement(ContextMenu, {
          x: 10,
          y: 20,
          invoker,
          onClose: vi.fn(),
          className: 'desktop-menu',
          items: [
            { kind: 'label', text: 'Displays' },
            { label: 'Main Display', onSelect: vi.fn(), checked: true, hint: '1470 x 956' },
            { label: 'Display 2', onSelect: vi.fn(), checked: false, hint: '1920 x 1080' },
            { kind: 'label', text: 'Windows' },
            { label: 'Terminal — zsh', onSelect: vi.fn(), checked: false },
            { kind: 'caption', text: 'A caption', id: 'cap-1' },
          ],
        }),
      ),
    )
  }

  it('checked items are menuitemradio with aria-checked and every item keeps the check column', () => {
    openExtended()
    const radios = Array.from(menu().querySelectorAll<HTMLElement>('[role="menuitemradio"]'))
    expect(radios.map((r) => r.getAttribute('aria-checked'))).toEqual(['true', 'false', 'false'])
    expect(menu().querySelectorAll('.desktop-menu__check')).toHaveLength(3)
    expect(radios[0].querySelector('.desktop-menu__check svg')).not.toBeNull()
    expect(radios[1].querySelector('.desktop-menu__check svg')).toBeNull()
    expect(radios[0].querySelector('.desktop-menu__hint')?.textContent).toBe('1470 x 956')
  })

  it('takes the extra class and links the caption from the menu', () => {
    openExtended()
    expect(menu().className).toBe('context-menu desktop-menu')
    expect(menu().getAttribute('aria-describedby')).toBe('cap-1')
    expect(document.getElementById('cap-1')?.getAttribute('role')).toBe('presentation')
  })

  it('label and caption rows are not focusable and the arrow keys skip them', () => {
    openExtended()
    expect(document.activeElement?.textContent).toContain('Main Display')
    key('ArrowDown')
    expect(document.activeElement?.textContent).toContain('Display 2')
    key('ArrowDown')
    expect(document.activeElement?.textContent).toContain('Terminal — zsh')
    key('ArrowDown')
    expect(document.activeElement?.textContent).toContain('Main Display')
    key('End')
    expect(document.activeElement?.textContent).toContain('Terminal — zsh')
    key('Home')
    expect(document.activeElement?.textContent).toContain('Main Display')
  })

  it('a plain menu keeps role=menuitem and no check column', () => {
    open()
    expect(menu().querySelector('[role="menuitemradio"]')).toBeNull()
    expect(menu().querySelector('.desktop-menu__check')).toBeNull()
  })

  it('focuses the first enabled item once a list that started empty arrives', () => {
    invoker = document.createElement('button')
    document.body.appendChild(invoker)
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    const render = (items: React.ComponentProps<typeof ContextMenu>['items']) =>
      flushSync(() => root.render(React.createElement(ContextMenu, { x: 0, y: 0, invoker, onClose: vi.fn(), items })))
    render([{ label: 'Loading…', onSelect: vi.fn(), disabled: true }])
    expect(menu().contains(document.activeElement)).toBe(false)
    render([{ label: 'Main Display', onSelect: vi.fn() }])
    expect(document.activeElement?.textContent).toBe('Main Display')
  })
})

describe('isMenuKey', () => {
  it('matches Shift+F10 and the ContextMenu key only', () => {
    const k = (key: string, shiftKey = false) => isMenuKey({ key, shiftKey } as React.KeyboardEvent)
    expect(k('F10', true)).toBe(true)
    expect(k('ContextMenu')).toBe(true)
    expect(k('F10')).toBe(false)
    expect(k('Enter', true)).toBe(false)
  })
})
