import React, { useEffect, useRef, useState } from 'react'
import { useDialogFocus } from '../lib/useDialogFocus'

interface QuitConfirmModalProps {
  isOpen: boolean
  sessions: Array<{ id: string; name: string }>
  onQuitGUI: () => void
  onQuitAll: () => void
  onCancel: () => void
}

function QuitConfirmModal({ isOpen, sessions, onQuitGUI, onQuitAll, onCancel }: QuitConfirmModalProps): React.ReactElement | null {
  const [acting, setActing] = useState(false)
  const dialogRef = useRef<HTMLDivElement>(null)
  // "Keep Running" first: both other choices end the GUI.
  useDialogFocus(dialogRef, { onCancel, initialFocus: 'cancel', active: isOpen })

  // Reset acting state when modal reopens
  useEffect(() => {
    if (isOpen) setActing(false)
  }, [isOpen])

  if (!isOpen) return null

  const visible = sessions.slice(0, 5)
  const overflow = sessions.length - 5

  const subtitleText =
    sessions.length === 0
      ? 'No active sessions.'
      : sessions.length === 1
        ? 'You have 1 active session running.'
        : `You have ${sessions.length} active sessions running.`

  return (
    <div className="dialog-scrim">
      <div
        ref={dialogRef}
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="quit-modal-title"
      >
        <h2 className="dialog__title" id="quit-modal-title">Quit Portty?</h2>
        <div className="dialog__body">
          <p>{subtitleText}</p>
          {sessions.length > 0 && (
            <div className="quit-modal__session-list">
              {visible.map((s) => (
                <div className="quit-modal__session-item" key={s.id}>
                  <span className="quit-modal__session-name">{s.name}</span>
                </div>
              ))}
              {overflow > 0 && (
                <span className="quit-modal__overflow dialog__secondary">...and {overflow} more</span>
              )}
            </div>
          )}
          {sessions.length === 0 && (
            <p className="quit-modal__no-sessions dialog__secondary">No active sessions.</p>
          )}
        </div>
        <div className="dialog__buttons">
          <button
            className="button button--secondary"
            data-dialog-cancel=""
            disabled={acting}
            onClick={onCancel}
          >
            Keep Running
          </button>
          <button
            className="button button--secondary"
            disabled={acting}
            onClick={() => { setActing(true); onQuitGUI() }}
          >
            Quit GUI Only
          </button>
          <button
            className="button button--primary"
            data-dialog-primary=""
            disabled={acting}
            onClick={() => { setActing(true); onQuitAll() }}
          >
            Quit Everything
          </button>
        </div>
      </div>
    </div>
  )
}

export { QuitConfirmModal }
