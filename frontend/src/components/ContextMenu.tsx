import React, { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

export interface ContextMenuItem {
  label: string
  onSelect: () => void
  separatorBefore?: boolean
}

/** Shift+F10 or the context-menu key open a menu from the keyboard. */
export function isMenuKey(e: React.KeyboardEvent): boolean {
  return e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')
}

interface ContextMenuProps {
  items: ContextMenuItem[]
  /** Viewport point: the pointer, or the bottom-left of the invoker for keyboard. */
  x: number
  y: number
  /** Gets focus back when the menu closes. */
  invoker: HTMLElement | null
  onClose: () => void
}

/**
 * Portty 02-08: NSMenu-styled HTML context menu (UI-SPEC "Context menus"),
 * shared by tabs and sidebar rows. Portaled to <body> so no drag region,
 * overflow clip or containing block of the invoker applies to it.
 */
export function ContextMenu({ items, x, y, invoker, onClose }: ContextMenuProps): React.ReactElement {
  const menuRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState({ left: x, top: y })

  function close() {
    invoker?.focus()
    onClose()
  }
  const closeRef = useRef(close)
  closeRef.current = close

  // Flip left/up to stay inside the window, then focus the first item.
  useLayoutEffect(() => {
    const el = menuRef.current
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    setPos({
      left: x + width > window.innerWidth ? Math.max(0, x - width) : x,
      top: y + height > window.innerHeight ? Math.max(0, y - height) : y,
    })
    el.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
  }, [x, y])

  useEffect(() => {
    function onMouseDown(e: MouseEvent) {
      if (!menuRef.current?.contains(e.target as Node)) closeRef.current()
    }
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') closeRef.current()
    }
    document.addEventListener('mousedown', onMouseDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onMouseDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [])

  function handleKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    const all = Array.from(menuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
    const i = all.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const step = e.key === 'ArrowDown' ? 1 : all.length - 1
      all[(i + step) % all.length]?.focus()
    } else if (e.key === 'Enter') {
      // Handled here so the native button activation cannot fire it twice.
      e.preventDefault()
      all[i]?.click()
    }
  }

  return createPortal(
    <div
      ref={menuRef}
      className="context-menu"
      role="menu"
      style={{ left: pos.left, top: pos.top }}
      onKeyDown={handleKeyDown}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item) => (
        <React.Fragment key={item.label}>
          {item.separatorBefore && <div className="context-menu__separator" role="separator" />}
          <button
            type="button"
            role="menuitem"
            className="context-menu__item"
            tabIndex={-1}
            onMouseEnter={(e) => e.currentTarget.focus()}
            onClick={() => {
              item.onSelect()
              close()
            }}
          >
            {item.label}
          </button>
        </React.Fragment>
      ))}
    </div>,
    document.body,
  )
}
