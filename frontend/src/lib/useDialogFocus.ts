import { useEffect, useRef, type RefObject } from 'react'

export type DialogInitialFocus = 'first-input' | 'primary' | 'cancel'

export interface DialogFocusOptions {
  onCancel: () => void
  /** Default 'first-input'. Each mode falls back to the next sensible target. */
  initialFocus?: DialogInitialFocus
  /** Default true. Dialogs that render null while closed pass their isOpen. */
  active?: boolean
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]'

function focusables(root: HTMLElement): HTMLElement[] {
  // tabindex=-1 covers the roving segments of ShareSegmentedControl.
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => el.getAttribute('tabindex') !== '-1',
  )
}

function initialTarget(root: HTMLElement, mode: DialogInitialFocus): HTMLElement | null {
  const input = root.querySelector<HTMLElement>(
    'input:not([disabled]), select:not([disabled]), textarea:not([disabled])',
  )
  const primary = root.querySelector<HTMLElement>('[data-dialog-primary]')
  const cancel = root.querySelector<HTMLElement>('[data-dialog-cancel]')
  const order =
    mode === 'first-input' ? [input, primary, cancel] : mode === 'primary' ? [primary, cancel] : [cancel, primary]
  return order.find((el) => el != null) ?? focusables(root)[0] ?? null
}

/**
 * Modal focus behaviour for every dialog (UI-SPEC "Dialogs and sheets"):
 * initial focus, Tab / Shift+Tab wrap inside the dialog, Esc cancels, and
 * focus returns to the invoker when the dialog closes or unmounts.
 */
export function useDialogFocus(ref: RefObject<HTMLElement | null>, options: DialogFocusOptions): void {
  const { initialFocus = 'first-input', active = true } = options
  // Callers pass inline arrows; read the latest one without re-running the effect.
  const onCancelRef = useRef(options.onCancel)
  onCancelRef.current = options.onCancel

  useEffect(() => {
    const dialog = ref.current
    if (!active || !dialog) return
    const invoker = document.activeElement as HTMLElement | null
    initialTarget(dialog, initialFocus)?.focus()

    function onKeyDown(e: KeyboardEvent): void {
      if (!dialog) return
      // Only the topmost modal handles keys (a confirm opened from a sheet sits later in the DOM).
      const modals = document.querySelectorAll('[aria-modal="true"]')
      if (modals.length > 0 && modals[modals.length - 1] !== dialog) return
      if (e.key === 'Escape') {
        e.preventDefault()
        onCancelRef.current()
        return
      }
      if (e.key !== 'Tab') return
      const items = focusables(dialog)
      if (items.length === 0) {
        e.preventDefault()
        return
      }
      const first = items[0]
      const last = items[items.length - 1]
      const current = document.activeElement as HTMLElement | null
      const inside = current !== null && dialog.contains(current)
      if (e.shiftKey && (!inside || current === first)) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && (!inside || current === last)) {
        e.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      invoker?.focus?.()
    }
  }, [active, initialFocus, ref])
}
