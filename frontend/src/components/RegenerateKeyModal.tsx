import React, { useEffect, useRef, useState } from 'react'
import { useDialogFocus } from '../lib/useDialogFocus'

interface RegenerateKeyModalProps {
  isOpen: boolean
  onConfirm: () => Promise<void>
  onCancel: () => void
  /**
   * Optional copy overrides so this confirm modal can be reused for actions
   * other than signing-key rotation (the End Session confirm). All default
   * to the signing-key copy, so call sites that omit them are unchanged.
   */
  title?: string
  /** Rendered inside a <p>: inline content only (End Session adds a block-styled span). */
  body?: React.ReactNode
  confirmLabel?: string
  /** Label shown while onConfirm is in flight (defaults to "Invalidating…"). */
  actingLabel?: string
  cancelLabel?: string
  titleId?: string
  /** Portty 02-08: the confirm button uses the destructive fill (End Session). */
  destructive?: boolean
}

/**
 * Confirmation modal — originally the signing-key rotation prompt (Phase 87
 * UI-SPEC Surface 2, D-16), generalized in Phase 150 to accept copy overrides
 * so other destructive/guarded actions can reuse the same dialog shell.
 *
 * Portty 02-09: the shared sheet style (.dialog*); useDialogFocus focuses
 * Cancel first, traps Tab, and maps Esc to onCancel.
 * Defaults preserve the original copy: "Invalidate All Links" is the
 * destructive confirm — rotating the signing key immediately invalidates every
 * outstanding capability across every session (the v3.1 panic button).
 */
function RegenerateKeyModal({
  isOpen,
  onConfirm,
  onCancel,
  title = 'Regenerate Signing Key?',
  body = 'This immediately invalidates ALL shared links across ALL sessions. Anyone currently using a shared terminal will be disconnected. You will need to re-share sessions to give access again.',
  confirmLabel = 'Invalidate All Links',
  actingLabel = 'Invalidating…',
  cancelLabel = 'Keep Links',
  titleId = 'regen-key-title',
  destructive = false,
}: RegenerateKeyModalProps): React.ReactElement | null {
  const [acting, setActing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  // Cancel first: the safer default for a destructive action. Esc is ignored
  // while onConfirm is in flight, matching the disabled Cancel button.
  useDialogFocus(dialogRef, {
    onCancel: () => { if (!acting) onCancel() },
    initialFocus: 'cancel',
    active: isOpen,
  })

  // Reset transient state when the modal opens or closes, so an error from a
  // previous attempt never shows on the next open.
  useEffect(() => {
    setError(null)
    setActing(false)
  }, [isOpen])

  if (!isOpen) return null

  async function handleConfirm(): Promise<void> {
    setError(null)
    setActing(true)
    try {
      await onConfirm()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setActing(false)
      return
    }
    setActing(false)
  }

  return (
    <div className="dialog-scrim">
      <div
        ref={dialogRef}
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
      >
        <h2 className="dialog__title" id={titleId}>{title}</h2>
        <div className="dialog__body">
          <p>{body}</p>
          {error && <p className="dialog__error" role="alert">{error}</p>}
        </div>
        <div className="dialog__buttons">
          <button
            className="button button--secondary"
            data-dialog-cancel=""
            disabled={acting}
            onClick={onCancel}
          >
            {cancelLabel}
          </button>
          <button
            className={`button ${destructive ? 'button--destructive' : 'button--primary'}`}
            data-dialog-primary=""
            disabled={acting}
            onClick={() => void handleConfirm()}
          >
            {acting ? actingLabel : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

export { RegenerateKeyModal }
