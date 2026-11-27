/**
 * useDialogFocus (Portty 02-09, LOOK-01, UI-SPEC "Dialogs and sheets"):
 * initial focus per mode, Tab / Shift+Tab wrap, Esc cancels, focus returns
 * to the invoker on unmount, and only the topmost modal handles keys.
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import React, { useRef } from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react-dom/test-utils'
import { useDialogFocus, type DialogInitialFocus } from '../useDialogFocus'

function Dialog({
  onCancel,
  initialFocus,
  withInput = true,
}: {
  onCancel: () => void
  initialFocus?: DialogInitialFocus
  withInput?: boolean
}) {
  const ref = useRef<HTMLDivElement>(null)
  useDialogFocus(ref, { onCancel, initialFocus })
  return (
    <div ref={ref} role="dialog" aria-modal="true" tabIndex={-1}>
      {withInput && <input aria-label="field" />}
      <button tabIndex={-1}>roving</button>
      <button data-dialog-cancel="">Cancel</button>
      <button data-dialog-primary="">OK</button>
    </div>
  )
}

let container: HTMLElement
let root: ReturnType<typeof createRoot>
let invoker: HTMLButtonElement

function mount(node: React.ReactElement): void {
  invoker = document.createElement('button')
  invoker.textContent = 'invoker'
  document.body.appendChild(invoker)
  invoker.focus()
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => root.render(node))
}

function key(k: string, shiftKey = false): void {
  act(() => {
    document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: k, shiftKey, bubbles: true }))
  })
}

const focusedText = () => (document.activeElement as HTMLElement | null)?.textContent || document.activeElement?.getAttribute('aria-label')

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  invoker?.remove()
})

describe('useDialogFocus', () => {
  it('first-input focuses the first input', () => {
    mount(<Dialog onCancel={vi.fn()} />)
    expect(focusedText()).toBe('field')
  })

  it('first-input falls back to the primary button when there is no input', () => {
    mount(<Dialog onCancel={vi.fn()} withInput={false} />)
    expect(focusedText()).toBe('OK')
  })

  it('primary focuses the primary button', () => {
    mount(<Dialog onCancel={vi.fn()} initialFocus="primary" />)
    expect(focusedText()).toBe('OK')
  })

  it('cancel focuses the cancel button', () => {
    mount(<Dialog onCancel={vi.fn()} initialFocus="cancel" />)
    expect(focusedText()).toBe('Cancel')
  })

  it('Tab on the last item wraps to the first, skipping tabindex=-1', () => {
    mount(<Dialog onCancel={vi.fn()} initialFocus="primary" />)
    key('Tab')
    expect(focusedText()).toBe('field')
  })

  it('Shift+Tab on the first item wraps to the last', () => {
    mount(<Dialog onCancel={vi.fn()} />)
    key('Tab', true)
    expect(focusedText()).toBe('OK')
  })

  it('dialog mode focuses the dialog element itself, not a button', () => {
    mount(<Dialog onCancel={vi.fn()} initialFocus="dialog" />)
    expect(document.activeElement).toBe(container.querySelector('[role="dialog"]'))
  })

  it('dialog mode: Tab goes to the first item, Shift+Tab to the last, Esc still calls onCancel', () => {
    const onCancel = vi.fn()
    mount(<Dialog onCancel={onCancel} initialFocus="dialog" />)
    key('Tab')
    expect(focusedText()).toBe('field')
    act(() => (container.querySelector('[role="dialog"]') as HTMLElement).focus())
    key('Tab', true)
    expect(focusedText()).toBe('OK')
    key('Escape')
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('Tab from outside the dialog moves focus back in', () => {
    mount(<Dialog onCancel={vi.fn()} />)
    invoker.focus()
    key('Tab')
    expect(focusedText()).toBe('field')
  })

  it('Esc calls onCancel', () => {
    const onCancel = vi.fn()
    mount(<Dialog onCancel={onCancel} />)
    key('Escape')
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('returns focus to the invoker on unmount', () => {
    mount(<Dialog onCancel={vi.fn()} />)
    expect(document.activeElement).not.toBe(invoker)
    act(() => root.unmount())
    expect(document.activeElement).toBe(invoker)
  })

  it('only the topmost modal handles Esc', () => {
    const outer = vi.fn()
    const inner = vi.fn()
    mount(
      <>
        <Dialog onCancel={outer} />
        <Dialog onCancel={inner} />
      </>,
    )
    key('Escape')
    expect(inner).toHaveBeenCalledTimes(1)
    expect(outer).not.toHaveBeenCalled()
  })
})
