/**
 * RegenerateKeyModal — copy customization tests (Phase 150 gap-closure, SET-01).
 *
 * Regression context: Phase 150-02 reused RegenerateKeyModal as the
 * confirm-on-disable dialog for the shell web-share warning toggle, but the
 * component had hardcoded "Regenerate Signing Key?" / "Invalidate All Links"
 * copy. The disable-warning surface therefore rendered a dialog claiming it
 * would invalidate ALL shared links — a misleading, action-mismatched prompt
 * (found in live UAT). These tests pin both the default (regen-key) copy AND
 * the parameterized copy so the component can be safely reused.
 *
 * Uses the project's createRoot + act DOM render convention (no
 * @testing-library/react dependency).
 */
import { describe, it, expect, afterEach, vi } from 'vitest'
import React from 'react'
import { createRoot } from 'react-dom/client'
import { act } from 'react-dom/test-utils'
import { RegenerateKeyModal } from '../RegenerateKeyModal'

const noop = async (): Promise<void> => {}

let container: HTMLElement
let root: ReturnType<typeof createRoot>

function mount(node: React.ReactElement): void {
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  act(() => {
    root.render(node)
  })
}

afterEach(() => {
  act(() => {
    root?.unmount()
  })
  container?.remove()
})

describe('RegenerateKeyModal copy', () => {
  it('renders the default signing-key copy when no copy props are supplied (regression guard for the Security surface)', () => {
    mount(<RegenerateKeyModal isOpen onConfirm={noop} onCancel={() => {}} />)
    const text = container.textContent ?? ''
    expect(text).toContain('Regenerate Signing Key?')
    expect(text).toContain('invalidates ALL shared links')
    expect(text).toContain('Keep Links')
    expect(text).toContain('Invalidate All Links')
  })

  it('renders supplied copy props instead of the signing-key defaults', () => {
    mount(
      <RegenerateKeyModal
        isOpen
        onConfirm={noop}
        onCancel={() => {}}
        title="Disable shell web-share warning?"
        body="AgentHub won't show the one-time security reminder before web-sharing a shell session."
        confirmLabel="Disable warning"
        cancelLabel="Keep warning"
      />
    )
    const text = container.textContent ?? ''
    expect(text).toContain('Disable shell web-share warning?')
    expect(text).toContain('one-time security reminder')
    expect(text).toContain('Disable warning')
    expect(text).toContain('Keep warning')
    // The misleading signing-key copy must NOT leak into a non-signing-key use.
    expect(text).not.toContain('Regenerate Signing Key')
    expect(text).not.toContain('Invalidate All Links')
    expect(text).not.toContain('invalidates ALL shared links')
  })

  it('confirm button invokes onConfirm regardless of label', async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined)
    mount(
      <RegenerateKeyModal
        isOpen
        onConfirm={onConfirm}
        onCancel={() => {}}
        confirmLabel="Disable warning"
      />
    )
    const buttons = Array.from(container.querySelectorAll<HTMLButtonElement>('button'))
    const confirmBtn = buttons.find((b) => b.textContent?.includes('Disable warning'))
    expect(confirmBtn).toBeTruthy()
    await act(async () => {
      confirmBtn!.click()
    })
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })
})

// Portty 02-08 (TERM-03): third consumer, the End Session confirm.
describe('RegenerateKeyModal: ReactNode body and destructive confirm (Portty 02-08)', () => {
  it('renders a ReactNode body, marks the confirm destructive, and still focuses Cancel first', () => {
    mount(
      <RegenerateKeyModal
        isOpen
        onConfirm={noop}
        onCancel={() => {}}
        title="End “shell”?"
        titleId="end-session-title"
        body={<>The shell will stop.<span className="end-session__note">Anyone attached will be disconnected.</span></>}
        confirmLabel="End Session"
        cancelLabel="Cancel"
        destructive
      />,
    )
    expect(container.querySelector('#end-session-title')?.textContent).toBe('End “shell”?')
    expect(container.querySelector('.end-session__note')?.textContent).toBe('Anyone attached will be disconnected.')
    const confirm = Array.from(container.querySelectorAll('button')).find((b) => b.textContent === 'End Session')!
    expect(confirm.classList.contains('button--destructive')).toBe(true)
    expect(document.activeElement?.textContent).toBe('Cancel')
  })

  it('is not destructive by default', () => {
    mount(<RegenerateKeyModal isOpen onConfirm={noop} onCancel={() => {}} />)
    expect(container.querySelector('.button--destructive')).toBeNull()
    expect(container.querySelector('[data-dialog-primary]')?.classList.contains('button--primary')).toBe(true)
  })
})

// Portty 02-09 (LOOK-01): the sheet shell shared by Regenerate, the disable-warning confirm and End Session.
describe('RegenerateKeyModal: sheet shell and focus (Portty 02-09)', () => {
  it('is a labelled modal dialog with no header ×, Cancel focused, buttons [Cancel, confirm]', () => {
    mount(<RegenerateKeyModal isOpen onConfirm={noop} onCancel={() => {}} />)
    const dialog = container.querySelector('[role="dialog"]')!
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    expect(document.getElementById(dialog.getAttribute('aria-labelledby')!)?.textContent).toBe('Regenerate Signing Key?')
    expect(container.querySelector('[aria-label="Close"]')).toBeNull()
    expect(container.innerHTML).not.toContain('×')
    expect(document.activeElement?.textContent).toBe('Keep Links')
    const buttons = Array.from(container.querySelectorAll('.dialog__buttons button')).map((b) => b.textContent)
    expect(buttons).toEqual(['Keep Links', 'Invalidate All Links'])
  })

  it('Esc cancels; a scrim click does not', () => {
    const onCancel = vi.fn()
    mount(<RegenerateKeyModal isOpen onConfirm={noop} onCancel={onCancel} />)
    act(() => (container.querySelector('.dialog-scrim') as HTMLElement).click())
    expect(onCancel).not.toHaveBeenCalled()
    act(() => {
      document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    })
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('shows a rejected onConfirm inline and stays open', async () => {
    mount(<RegenerateKeyModal isOpen onConfirm={() => Promise.reject(new Error('boom'))} onCancel={() => {}} />)
    const confirm = container.querySelector<HTMLButtonElement>('[data-dialog-primary]')!
    await act(async () => { confirm.click() })
    expect(container.querySelector('.dialog__error')?.textContent).toBe('boom')
    expect(container.querySelector('[role="dialog"]')).not.toBeNull()
  })
})
