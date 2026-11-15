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

describe('isMenuKey', () => {
  it('matches Shift+F10 and the ContextMenu key only', () => {
    const k = (key: string, shiftKey = false) => isMenuKey({ key, shiftKey } as React.KeyboardEvent)
    expect(k('F10', true)).toBe(true)
    expect(k('ContextMenu')).toBe(true)
    expect(k('F10')).toBe(false)
    expect(k('Enter', true)).toBe(false)
  })
})
