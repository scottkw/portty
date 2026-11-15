/**
 * New Session modal contract (01-UI-SPEC section 1, CONTEXT Amendment 1):
 * Working Directory section only, Cancel / Create Session footer. No shell line,
 * no agent list and no Extra Arguments field.
 * Portty 02-09 (LOOK-01): sheet style, no header ×, the scrim does nothing,
 * Esc cancels, and a failed create shows the error inline with the dialog open.
 */
import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { act } from 'react-dom/test-utils'
import raw from '../NewSessionModal.tsx?raw'
import { NewSessionModal } from '../NewSessionModal'
import * as AppMock from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  OpenDirectoryDialog: vi.fn().mockResolvedValue(''),
}))

function renderModal(onConfirm = vi.fn().mockResolvedValue(undefined), onClose = vi.fn()) {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(React.createElement(NewSessionModal, { isOpen: true, onConfirm, onClose }))
  })
  return { container, root, onConfirm, onClose }
}

const byText = (container: HTMLElement, text: string) =>
  Array.from(container.querySelectorAll('button')).find((b) => b.textContent === text) as HTMLButtonElement

describe('NewSessionModal source contract', () => {
  it('keeps the agenthub:lastWorkDir key (Phase 3 renames it)', () => {
    expect(raw).toContain("'agenthub:lastWorkDir'")
  })
  it('has no agent picker, args field or per-agent args persistence', () => {
    expect(raw).not.toMatch(/ARGS_KEY|SHELL_ARGS_KEY|Extra Arguments|Select Agent|DetectedCLI|DetectedShell/)
  })
  it('has no shell lookup or Settings → Paths reference', () => {
    expect(raw).not.toMatch(/GetShellPath|Settings → Paths/)
  })
})

describe('NewSessionModal rendering and behavior', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(AppMock.OpenDirectoryDialog).mockResolvedValue('')
  })

  afterEach(() => {
    root?.unmount()
    container?.remove()
    localStorage.clear()
  })

  it('renders nothing when closed', () => {
    container = document.createElement('div')
    root = createRoot(container)
    flushSync(() => {
      root.render(React.createElement(NewSessionModal, { isOpen: false, onConfirm: vi.fn(), onClose: vi.fn() }))
    })
    expect(container.innerHTML).toBe('')
  })

  it('renders title, only the Working Directory section and footer buttons', () => {
    ;({ container, root } = renderModal())
    expect(container.querySelector('h2')?.textContent).toBe('New Session')
    const labels = Array.from(container.querySelectorAll('.new-session-modal__section-label')).map((l) => l.textContent)
    expect(labels).toEqual(['Working Directory'])
    expect(container.textContent).not.toContain('Shell')
    expect(container.textContent).not.toContain('login shell')
    expect(byText(container, 'Cancel')).toBeDefined()
    expect(byText(container, 'Close')).toBeUndefined()
    expect(byText(container, 'Create Session')).toBeDefined()
    expect(container.textContent).not.toContain('Select Agent')
    expect(container.textContent).not.toContain('Extra Arguments')
  })

  it('shows "Home directory (default)" when no directory was used before', () => {
    ;({ container, root } = renderModal())
    const display = container.querySelector('.new-session-modal__folder-row .new-session-modal__folder-display')
    expect(display?.textContent).toBe('Home directory (default)')
    expect(display?.className).not.toContain('--active')
  })

  it('Create Session is enabled, calls onConfirm with "" when none chosen, then shows Creating…', () => {
    let onConfirm: ReturnType<typeof vi.fn>
    ;({ container, root, onConfirm } = renderModal())
    expect(byText(container, 'Create Session').disabled).toBe(false)
    flushSync(() => byText(container, 'Create Session').click())
    expect(onConfirm).toHaveBeenCalledWith('')
    expect(byText(container, 'Creating…').disabled).toBe(true)
  })

  it('pre-fills the last-used directory and passes it to onConfirm', () => {
    localStorage.setItem('agenthub:lastWorkDir', '/Users/me/project')
    let onConfirm: ReturnType<typeof vi.fn>
    ;({ container, root, onConfirm } = renderModal())
    const display = container.querySelector('.new-session-modal__folder-display--active')
    expect(display?.textContent).toBe('/Users/me/project')
    flushSync(() => byText(container, 'Create Session').click())
    expect(onConfirm).toHaveBeenCalledWith('/Users/me/project')
  })

  it('Browse… reads "Browsing…" and is disabled while the picker is open, then persists the choice', async () => {
    let resolvePick: (v: string) => void = () => {}
    vi.mocked(AppMock.OpenDirectoryDialog).mockReturnValue(new Promise<string>((r) => { resolvePick = r }))
    let onConfirm: ReturnType<typeof vi.fn>
    ;({ container, root, onConfirm } = renderModal())

    await act(async () => { byText(container, 'Browse…').click() })
    const browsing = byText(container, 'Browsing…')
    expect(browsing).toBeDefined()
    expect(browsing.disabled).toBe(true)

    await act(async () => { resolvePick('/tmp/picked') })
    expect(byText(container, 'Browse…').disabled).toBe(false)
    expect(localStorage.getItem('agenthub:lastWorkDir')).toBe('/tmp/picked')
    flushSync(() => byText(container, 'Create Session').click())
    expect(onConfirm).toHaveBeenCalledWith('/tmp/picked')
  })

  it('is a labelled modal dialog with no header close button', () => {
    ;({ container, root } = renderModal())
    const dialog = container.querySelector('[role="dialog"]')!
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    expect(document.getElementById(dialog.getAttribute('aria-labelledby')!)?.textContent).toBe('New Session')
    expect(container.querySelector('[aria-label="Close"]')).toBeNull()
    expect(container.innerHTML).not.toContain('×')
  })

  it('focuses Create Session on open', () => {
    ;({ container, root } = renderModal())
    expect(document.activeElement?.textContent).toBe('Create Session')
  })

  it('Cancel and Esc call onClose; scrim and in-dialog clicks do not', () => {
    let onClose: ReturnType<typeof vi.fn>
    ;({ container, root, onClose } = renderModal())
    flushSync(() => (container.querySelector('.dialog') as HTMLElement).click())
    flushSync(() => (container.querySelector('.dialog-scrim') as HTMLElement).click())
    expect(onClose).not.toHaveBeenCalled()
    flushSync(() => byText(container, 'Cancel').click())
    expect(onClose).toHaveBeenCalledTimes(1)
    act(() => {
      document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    })
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('a rejected create keeps the dialog open and shows the error copy', async () => {
    const onConfirm = vi.fn().mockRejectedValue(new Error('no such directory'))
    ;({ container, root } = renderModal(onConfirm))
    await act(async () => { byText(container, 'Create Session').click() })
    expect(container.querySelector('[role="dialog"]')).not.toBeNull()
    expect(container.querySelector('.dialog__error')?.textContent).toBe(
      "Couldn't create the session: no such directory. Check that the folder exists, then try again.",
    )
    expect(byText(container, 'Create Session').disabled).toBe(false)
  })
})
