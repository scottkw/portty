import { describe, it, expect, afterEach, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { act } from 'react-dom/test-utils'
import raw from '../QuitConfirmModal.tsx?raw'
import appRaw from '../../App.tsx?raw'
import { QuitConfirmModal } from '../QuitConfirmModal'

describe('QuitConfirmModal source inspection', () => {
  describe('APP-01: modal structure', () => {
    it('uses the shared sheet shell and the dialog focus hook (Portty 02-09)', () => {
      expect(raw).toContain('dialog-scrim')
      expect(raw).toContain('useDialogFocus(')
    })
    it('sets role dialog and aria-modal', () => {
      expect(raw).toContain('role="dialog"')
      expect(raw).toContain('aria-modal="true"')
    })
    it('sets aria-labelledby pointing to title', () => {
      expect(raw).toContain('aria-labelledby="quit-modal-title"')
    })
    it('has no header close button (Portty 02-09)', () => {
      expect(raw).not.toContain('aria-label="Close"')
      expect(raw).not.toContain('&times;')
    })
    it('displays modal title "Quit Portty?"', () => {
      expect(raw).toContain('Quit Portty?')
    })
  })

  describe('APP-02: exit mode buttons', () => {
    it('renders Keep Running button', () => {
      expect(raw).toContain('Keep Running')
    })
    it('renders Quit GUI Only button', () => {
      expect(raw).toContain('Quit GUI Only')
    })
    it('renders Quit Everything button', () => {
      expect(raw).toContain('Quit Everything')
    })
    it('disables buttons after click (acting state)', () => {
      expect(raw).toContain('acting')
      expect(raw).toContain('disabled={acting}')
    })
  })

  describe('APP-03: session list display', () => {
    it('accepts sessions prop', () => {
      expect(raw).toContain('sessions')
    })
    it('renders session name', () => {
      expect(raw).toContain('quit-modal__session-name')
    })
    it('shows no session status dot or text (CONTEXT Amendment 2)', () => {
      expect(raw).not.toContain('quit-modal__session-status')
      expect(raw).not.toContain('quit-modal__session-dot')
      expect(raw).not.toContain('dotColor')
    })
    it('shows no-sessions text when empty (D-02)', () => {
      expect(raw).toContain('No active sessions')
    })
    it('truncates session list at 5 entries', () => {
      expect(raw).toContain('slice(0, 5)')
    })
    it('shows overflow count', () => {
      expect(raw).toContain('quit-modal__overflow')
    })
  })
})

describe('App.tsx quit-modal wiring (Phase 85)', () => {
  it('subscribes to app:quit-requested event', () => {
    expect(appRaw).toContain("'app:quit-requested'")
  })
  it('defines showQuitModal state', () => {
    expect(appRaw).toContain('showQuitModal')
  })
  it('imports QuitConfirmModal component', () => {
    expect(appRaw).toContain('QuitConfirmModal')
  })
  it('renders QuitConfirmModal in JSX', () => {
    expect(appRaw).toContain('<QuitConfirmModal')
  })
  it('unsubscribes via offQuit in cleanup', () => {
    expect(appRaw).toContain('offQuit')
  })
  it('imports QuitGUIOnly bound method', () => {
    expect(appRaw).toContain('QuitGUIOnly')
  })
  it('imports QuitAll bound method', () => {
    expect(appRaw).toContain('QuitAll')
  })
})

describe('QuitConfirmModal behaviour (Portty 02-09, LOOK-01)', () => {
  let container: HTMLElement
  let root: ReturnType<typeof createRoot>
  afterEach(() => {
    act(() => root?.unmount())
    container?.remove()
  })
  function mount(onCancel = vi.fn()) {
    container = document.createElement('div')
    document.body.appendChild(container)
    root = createRoot(container)
    act(() => {
      root.render(
        <QuitConfirmModal isOpen sessions={[{ id: 'a', name: 'shell 1' }]} onQuitGUI={vi.fn()} onQuitAll={vi.fn()} onCancel={onCancel} />,
      )
    })
    return onCancel
  }

  it('is a labelled modal dialog with Keep Running focused and the buttons in order', () => {
    mount()
    const dialog = container.querySelector('[role="dialog"]')!
    expect(dialog.getAttribute('aria-modal')).toBe('true')
    expect(document.getElementById(dialog.getAttribute('aria-labelledby')!)?.textContent).toBe('Quit Portty?')
    expect(document.activeElement?.textContent).toBe('Keep Running')
    const buttons = Array.from(container.querySelectorAll('.dialog__buttons button'))
    expect(buttons.map((b) => b.textContent)).toEqual(['Keep Running', 'Quit GUI Only', 'Quit Everything'])
    expect(buttons[2].classList.contains('button--primary')).toBe(true)
  })

  it('Esc cancels; a scrim click does not', () => {
    const onCancel = mount()
    act(() => (container.querySelector('.dialog-scrim') as HTMLElement).click())
    expect(onCancel).not.toHaveBeenCalled()
    act(() => {
      document.activeElement!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    })
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  describe('share warning (22-11)', () => {
    const SENTENCE = 'Quitting also stops sharing your desktop and disconnects everyone watching.'
    function mountSharing(sharing: boolean | undefined) {
      container = document.createElement('div')
      document.body.appendChild(container)
      root = createRoot(container)
      act(() => {
        root.render(
          <QuitConfirmModal isOpen sessions={[{ id: 'a', name: 'shell 1' }]} sharing={sharing} onQuitGUI={vi.fn()} onQuitAll={vi.fn()} onCancel={vi.fn()} />,
        )
      })
    }

    it('with sharing true the sentence sits right under the session count and all three choices remain', () => {
      mountSharing(true)
      const paragraphs = Array.from(container.querySelectorAll('.dialog__body > p')).map((p) => p.textContent)
      expect(paragraphs.slice(0, 2)).toEqual(['You have 1 active session running.', SENTENCE])
      expect(Array.from(container.querySelectorAll('.dialog__buttons button')).map((b) => b.textContent)).toEqual([
        'Keep Running', 'Quit GUI Only', 'Quit Everything',
      ])
    })

    it('with sharing false or absent the sentence is not shown', () => {
      mountSharing(false)
      expect(container.textContent).not.toContain('stops sharing')
      act(() => root.unmount())
      container.remove()
      mountSharing(undefined)
      expect(container.textContent).not.toContain('stops sharing')
    })
  })
})
