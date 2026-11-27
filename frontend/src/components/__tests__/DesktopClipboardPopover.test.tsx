// Phase 20-08 Task 2 — the Copy fallback popover and the Paste popover (20-UI-SPEC Surface D).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { DesktopCopyPopover, DesktopPastePopover } from '../DesktopClipboardPopover'

let container: HTMLDivElement
let root: Root
let anchor: HTMLButtonElement

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  container = document.createElement('div')
  document.body.appendChild(container)
  anchor = document.createElement('button')
  anchor.textContent = 'Clipboard'
  document.body.appendChild(anchor)
  root = createRoot(container)
})

afterEach(() => {
  act(() => root.unmount())
  container.remove()
  anchor.remove()
})

const dialog = () => document.querySelector<HTMLElement>('[role="dialog"]')
const textarea = () => document.querySelector<HTMLTextAreaElement>('textarea')!
const button = (name: string) => Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find((b) => b.textContent === name)!
const key = (k: string, init: KeyboardEventInit = {}) =>
  act(() => {
    document.activeElement?.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...init }))
  })
/** A React-controlled textarea only sees a value set through the native setter. */
function type(el: HTMLTextAreaElement, value: string) {
  act(() => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(el, value)
    el.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

describe('DesktopCopyPopover', () => {
  function open(text = "the Mac's text", onClose = vi.fn()) {
    act(() => root.render(<DesktopCopyPopover text={text} anchor={anchor} onClose={onClose} />))
    return onClose
  }

  it('is a dialog labelled Copy from Mac with its explanation, the text in a read-only textarea, and Done', () => {
    open()
    expect(dialog()?.getAttribute('aria-modal')).not.toBe('true')
    const title = document.getElementById(dialog()!.getAttribute('aria-labelledby')!)
    expect(title?.textContent).toBe('Copy from Mac')
    expect(dialog()?.textContent).toContain("Your browser didn't let Portty write to your clipboard.")
    expect(dialog()?.textContent).toContain('Select the text below and copy it.')
    expect(textarea().readOnly).toBe(true)
    expect(textarea().value).toBe("the Mac's text")
    expect(button('Done')).toBeTruthy()
  })

  it('selects the whole text and focuses it on open', () => {
    open('hello, world')
    expect(document.activeElement).toBe(textarea())
    expect(textarea().selectionStart).toBe(0)
    expect(textarea().selectionEnd).toBe('hello, world'.length)
  })

  it('renders markup as text only', () => {
    open('<img src=x onerror=alert(1)><b>x</b>')
    expect(dialog()?.querySelector('img')).toBeNull()
    expect(dialog()?.querySelector('b')).toBeNull()
    expect(textarea().value).toBe('<img src=x onerror=alert(1)><b>x</b>')
  })

  it('Done closes and gives focus back to the Clipboard button', () => {
    const onClose = open()
    act(() => button('Done').click())
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(document.activeElement).toBe(anchor)
  })

  it('Esc closes and gives focus back', () => {
    const onClose = open()
    key('Escape')
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(document.activeElement).toBe(anchor)
  })

  it('a press outside closes it; a press inside does not', () => {
    const onClose = open()
    act(() => { textarea().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })) })
    expect(onClose).not.toHaveBeenCalled()
    act(() => { document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })) })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('Tab wraps between the textarea and Done', () => {
    open()
    act(() => button('Done').focus())
    key('Tab')
    expect(document.activeElement).toBe(textarea())
    key('Tab', { shiftKey: true })
    expect(document.activeElement).toBe(button('Done'))
  })

  it('is placed 4 px under the anchor, inside the page', () => {
    anchor.getBoundingClientRect = () => ({ left: 100, right: 180, top: 10, bottom: 46, width: 80, height: 36, x: 100, y: 10, toJSON() {} })
    open()
    expect(dialog()!.style.top).toBe('50px')
    expect(dialog()!.style.left).toBe('100px')
  })

  it('portals into the full-screen element when there is one', () => {
    const fs = document.createElement('div')
    document.body.appendChild(fs)
    Object.defineProperty(document, 'fullscreenElement', { configurable: true, get: () => fs })
    try {
      open()
      expect(fs.querySelector('[role="dialog"]')).not.toBeNull()
    } finally {
      delete (document as unknown as { fullscreenElement?: unknown }).fullscreenElement
      act(() => root.unmount())
      fs.remove()
      root = createRoot(container)
    }
  })

  it('logs nothing', () => {
    const spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) => vi.spyOn(console, m).mockImplementation(() => {}))
    open('hunter2-secret')
    act(() => button('Done').click())
    for (const s of spies) expect(s).not.toHaveBeenCalled()
    vi.restoreAllMocks()
  })
})

describe('DesktopPastePopover', () => {
  function open(onSend = vi.fn(), onClose = vi.fn()) {
    act(() => root.render(<DesktopPastePopover anchor={anchor} onSend={onSend} onClose={onClose} />))
    return { onSend, onClose }
  }
  const send = () => button('Send to Mac')

  it('is a dialog labelled Paste to Mac with its explanation, a focused empty textarea with its placeholder, Discard Text and Send to Mac', () => {
    open()
    const title = document.getElementById(dialog()!.getAttribute('aria-labelledby')!)
    expect(title?.textContent).toBe('Paste to Mac')
    expect(dialog()?.textContent).toContain("Your browser didn't let Portty read your clipboard. Paste your text here, then choose Send to Mac.")
    expect(dialog()?.textContent).toContain("This replaces what's on the Mac's clipboard.")
    expect(textarea().value).toBe('')
    expect(textarea().readOnly).toBe(false)
    expect(textarea().placeholder).toBe('Paste text here')
    expect(document.activeElement).toBe(textarea())
    expect(button('Discard Text')).toBeTruthy()
    expect(send()).toBeTruthy()
  })

  it('Send to Mac is aria-disabled while empty and sends nothing when pressed', () => {
    const { onSend, onClose } = open()
    expect(send().getAttribute('aria-disabled')).toBe('true')
    act(() => send().click())
    expect(onSend).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('with text, Send to Mac is live, calls onSend once with the text and closes', () => {
    const { onSend, onClose } = open()
    type(textarea(), 'abc')
    expect(send().getAttribute('aria-disabled')).toBeNull()
    act(() => send().click())
    expect(onSend).toHaveBeenCalledTimes(1)
    expect(onSend).toHaveBeenCalledWith('abc')
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('1048577 bytes shows the inline alert and keeps Send to Mac disabled; exactly 1048576 does not', () => {
    const { onSend } = open()
    type(textarea(), 'a'.repeat(1048576))
    expect(dialog()?.querySelector('[role="alert"]')).toBeNull()
    expect(send().getAttribute('aria-disabled')).toBeNull()
    type(textarea(), 'a'.repeat(1048577))
    expect(dialog()?.querySelector('[role="alert"]')?.textContent).toBe('Too long to send (limit 1 MB).')
    expect(send().getAttribute('aria-disabled')).toBe('true')
    expect(textarea().getAttribute('aria-invalid')).toBe('true')
    act(() => send().click())
    expect(onSend).not.toHaveBeenCalled()
  })

  it('counts UTF-8 bytes, not characters', () => {
    open()
    type(textarea(), 'é'.repeat(524289)) // 1048578 bytes
    expect(dialog()?.querySelector('[role="alert"]')).not.toBeNull()
    type(textarea(), 'é'.repeat(524288)) // exactly 1048576
    expect(dialog()?.querySelector('[role="alert"]')).toBeNull()
  })

  it('Discard Text closes without sending, and gives focus back', () => {
    const { onSend, onClose } = open()
    type(textarea(), 'abc')
    act(() => button('Discard Text').click())
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onSend).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(anchor)
  })

  it('Esc discards: closes without sending', () => {
    const { onSend, onClose } = open()
    type(textarea(), 'abc')
    key('Escape')
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onSend).not.toHaveBeenCalled()
  })

  it('Tab order is the textarea, Discard Text, Send to Mac, and it wraps inside the popover', () => {
    open()
    const order = Array.from(dialog()!.querySelectorAll<HTMLElement>('textarea, button')).map((e) => e.textContent || e.tagName)
    expect(order).toEqual(['TEXTAREA', 'Discard Text', 'Send to Mac'])
    act(() => send().focus())
    key('Tab')
    expect(document.activeElement).toBe(textarea())
    key('Tab', { shiftKey: true })
    expect(document.activeElement).toBe(send())
  })

  it('does not stop a user paste into its textarea: the value takes whatever was pasted', () => {
    open()
    const ev = new Event('paste', { bubbles: true, cancelable: true })
    act(() => { textarea().dispatchEvent(ev) })
    expect(ev.defaultPrevented).toBe(false)
  })

  it('a press outside closes it, discarding the text', () => {
    const { onSend, onClose } = open()
    act(() => { document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })) })
    expect(onClose).toHaveBeenCalledTimes(1)
    expect(onSend).not.toHaveBeenCalled()
  })

  it('the textareas scroll inside the popover from 160 px (stylesheet)', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const css = readFileSync(resolve(__dirname, '../../style.css'), 'utf8')
    const rule = css.match(/\.desktop-popover__textarea\s*\{[^}]*\}/)?.[0] ?? ''
    expect(rule).toMatch(/max-height:\s*160px/)
    expect(rule).toMatch(/min-height:\s*96px/)
    expect(rule).toMatch(/overflow(-y)?:\s*auto/)
  })
})
