import React, { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CheckIcon } from '@heroicons/react/24/outline'

export interface ContextMenuItem {
  label: string
  onSelect: () => void
  separatorBefore?: boolean
  /** Visible but inert (Portty 04-05): aria-disabled, dimmed, no highlight, skipped by ↑/↓, ignores Enter and click. */
  disabled?: boolean
  /** id of the visible text that explains a disabled item. */
  describedBy?: string
  /** Phase 20-07: a choice in a group. Renders role menuitemradio with aria-checked; the menu keeps a 16 px check column for every item. */
  checked?: boolean
  /** Phase 20-07: secondary text, right-aligned. */
  hint?: string
  /** Tooltip; defaults to the label when it could be cut off by the menu width. */
  title?: string
}

/** Phase 20-07: a non-interactive row. Not focusable, skipped by the arrow keys. */
export interface ContextMenuRow {
  kind: 'label' | 'caption'
  text: string
  /** A caption's id; the menu links to every caption by aria-describedby. */
  id?: string
}

export type ContextMenuEntry = ContextMenuItem | ContextMenuRow

function isRow(e: ContextMenuEntry): e is ContextMenuRow {
  return 'kind' in e
}

/** Shift+F10 or the context-menu key open a menu from the keyboard. */
export function isMenuKey(e: React.KeyboardEvent): boolean {
  return e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')
}

interface ContextMenuProps {
  items: ContextMenuEntry[]
  /** Viewport point: the pointer, or the bottom-left of the invoker for keyboard. */
  x: number
  y: number
  /** Gets focus back when the menu closes. */
  invoker: HTMLElement | null
  onClose: () => void
  /** Extra class for a styled variant (the viewer's .desktop-menu). */
  className?: string
}

const ENABLED = '[role^="menuitem"]:not([aria-disabled="true"])'

/**
 * Portty 02-08: NSMenu-styled HTML context menu (UI-SPEC "Context menus"),
 * shared by tabs and sidebar rows. Portaled to <body> so no drag region,
 * overflow clip or containing block of the invoker applies to it.
 */
export function ContextMenu({ items, x, y, invoker, onClose, className }: ContextMenuProps): React.ReactElement {
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
    el.querySelector<HTMLElement>(ENABLED)?.focus()
  }, [x, y])

  // A list that arrives later (the viewer's picker) gets its first item focused once it has one.
  useLayoutEffect(() => {
    const el = menuRef.current
    if (el && !el.contains(document.activeElement)) el.querySelector<HTMLElement>(ENABLED)?.focus()
  })

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
    const all = Array.from(menuRef.current?.querySelectorAll<HTMLElement>(ENABLED) ?? [])
    const i = all.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const step = e.key === 'ArrowDown' ? 1 : all.length - 1
      all[(i + step) % all.length]?.focus()
    } else if (e.key === 'Home' || e.key === 'End') {
      e.preventDefault()
      all[e.key === 'Home' ? 0 : all.length - 1]?.focus()
    } else if (e.key === 'Enter') {
      // Handled here so the native button activation cannot fire it twice.
      e.preventDefault()
      all[i]?.click()
    }
  }

  const captionIds = items.filter(isRow).filter((r) => r.kind === 'caption' && r.id).map((r) => r.id).join(' ')
  const hasChecks = items.some((it) => !isRow(it) && it.checked !== undefined)

  return createPortal(
    <div
      ref={menuRef}
      className={className ? `context-menu ${className}` : 'context-menu'}
      role="menu"
      aria-describedby={captionIds || undefined}
      style={{ left: pos.left, top: pos.top }}
      onKeyDown={handleKeyDown}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item, n) => {
        if (isRow(item)) {
          return (
            <div
              key={`${n}-${item.text}`}
              id={item.id}
              role="presentation"
              className={item.kind === 'label' ? 'desktop-menu__label' : 'desktop-menu__caption'}
            >
              {item.text}
            </div>
          )
        }
        return (
          <React.Fragment key={`${n}-${item.label}`}>
            {item.separatorBefore && <div className="context-menu__separator" role="separator" />}
            <button
              type="button"
              role={item.checked === undefined ? 'menuitem' : 'menuitemradio'}
              aria-checked={item.checked === undefined ? undefined : item.checked}
              className="context-menu__item"
              tabIndex={-1}
              title={item.title}
              aria-disabled={item.disabled ? 'true' : undefined}
              aria-describedby={item.disabled ? item.describedBy : undefined}
              onMouseEnter={(e) => { if (!item.disabled) e.currentTarget.focus() }}
              onClick={() => {
                if (item.disabled) return
                item.onSelect()
                close()
              }}
            >
              {hasChecks && (
                <span className="desktop-menu__check" aria-hidden="true">
                  {item.checked && <CheckIcon width={16} height={16} />}
                </span>
              )}
              <span className="desktop-menu__text">{item.label}</span>
              {item.hint && <span className="desktop-menu__hint">{item.hint}</span>}
            </button>
          </React.Fragment>
        )
      })}
    </div>,
    document.body,
  )
}
