/**
 * New Session modal Shell line (01-UI-SPEC section 1 + UI Considerations):
 * read-only, non-focusable, mono; shows the GetShellPath() result, falling back
 * to "Default login shell" while pending, on rejection and on an empty result.
 */
import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { act } from 'react-dom/test-utils'
import { NewSessionModal } from '../NewSessionModal'
import * as AppMock from '../../wailsjs/go/main/App'

vi.mock('../../wailsjs/go/main/App', () => ({
  OpenDirectoryDialog: vi.fn().mockResolvedValue(''),
  GetShellPath: vi.fn().mockResolvedValue('/bin/zsh'),
}))

function renderModal() {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  flushSync(() => {
    root.render(React.createElement(NewSessionModal, { isOpen: true, onConfirm: vi.fn(), onClose: vi.fn() }))
  })
  return { container, root }
}

const shellLine = (container: HTMLElement) =>
  container.querySelector('.new-session-modal__folder-display--mono') as HTMLElement

describe('New Session modal — Shell line', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>

  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    root?.unmount()
    container?.remove()
    localStorage.clear()
  })

  it('shows "Default login shell" while GetShellPath is pending, then the resolved path', async () => {
    let resolve: (v: string) => void = () => {}
    vi.mocked(AppMock.GetShellPath).mockReturnValue(new Promise<string>((r) => { resolve = r }))
    ;({ container, root } = renderModal())
    expect(shellLine(container).textContent).toBe('Default login shell')
    await act(async () => { resolve('/opt/homebrew/bin/zsh') })
    expect(shellLine(container).textContent).toBe('/opt/homebrew/bin/zsh')
  })

  it('shows "Default login shell" and keeps Create Session enabled when GetShellPath rejects', async () => {
    vi.mocked(AppMock.GetShellPath).mockRejectedValue(new Error('daemon down'))
    await act(async () => { ({ container, root } = renderModal()) })
    expect(shellLine(container).textContent).toBe('Default login shell')
    const create = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'Create Session')!
    expect(create.disabled).toBe(false)
  })

  it('shows "Default login shell" when GetShellPath resolves to ""', async () => {
    vi.mocked(AppMock.GetShellPath).mockResolvedValue('')
    await act(async () => { ({ container, root } = renderModal()) })
    expect(shellLine(container).textContent).toBe('Default login shell')
  })

  it('is a read-only, non-focusable line (no button or textbox) in the folder-display box', async () => {
    vi.mocked(AppMock.GetShellPath).mockResolvedValue('/bin/zsh')
    await act(async () => { ({ container, root } = renderModal()) })
    const line = shellLine(container)
    expect(line.tagName).toBe('DIV')
    expect(line.className).toContain('new-session-modal__folder-display')
    expect(line.className).not.toContain('--active')
    expect(line.hasAttribute('tabindex')).toBe(false)
    expect(line.getAttribute('role')).toBeNull()
    const section = line.closest('.new-session-modal__section') as HTMLElement
    expect(section.querySelector('button, input, textarea, [role="button"], [role="textbox"]')).toBeNull()
    expect(section.querySelector('.new-session-modal__helper')?.textContent)
      .toBe('Starts as a login shell. Change it in Settings → Paths.')
  })

  it('re-fetches the shell path on every open', async () => {
    vi.mocked(AppMock.GetShellPath).mockResolvedValue('/bin/zsh')
    await act(async () => { ({ container, root } = renderModal()) })
    await act(async () => {
      root.render(React.createElement(NewSessionModal, { isOpen: false, onConfirm: vi.fn(), onClose: vi.fn() }))
    })
    vi.mocked(AppMock.GetShellPath).mockResolvedValue('/bin/bash')
    await act(async () => {
      root.render(React.createElement(NewSessionModal, { isOpen: true, onConfirm: vi.fn(), onClose: vi.fn() }))
    })
    expect(AppMock.GetShellPath).toHaveBeenCalledTimes(2)
    expect(shellLine(container).textContent).toBe('/bin/bash')
  })
})
