import { useRef, useState } from 'react'
import { OpenDirectoryDialog } from '../wailsjs/go/main/App'
import { useDialogFocus } from '../lib/useDialogFocus'

const LAST_DIR_KEY = 'agenthub:lastWorkDir'

export interface NewSessionModalProps {
  isOpen: boolean
  /** Called with the chosen working directory ("" = home directory). A rejection keeps the dialog open with the error. */
  onConfirm: (workDir: string) => Promise<void>
  onClose: () => void
}

export function NewSessionModal({ isOpen, onConfirm, onClose }: NewSessionModalProps) {
  const [selectedDir, setSelectedDir] = useState(() => localStorage.getItem(LAST_DIR_KEY) ?? '')
  const [browseLoading, setBrowseLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  // The folder picker is a button, so Create Session takes initial focus.
  useDialogFocus(dialogRef, { onCancel: onClose, initialFocus: 'primary', active: isOpen })

  if (!isOpen) return null

  async function handleBrowse() {
    setError(null)
    setBrowseLoading(true)
    try {
      const path = await OpenDirectoryDialog(selectedDir)
      if (path !== '') {
        setSelectedDir(path)
        localStorage.setItem(LAST_DIR_KEY, path)
      }
    } catch (err) {
      const detail = err instanceof Error ? err.message : String(err)
      setError(`Couldn't open the folder picker: ${detail}.`)
    } finally {
      setBrowseLoading(false)
    }
  }

  async function handleConfirm() {
    setError(null)
    setCreating(true)
    try {
      await onConfirm(selectedDir)
    } catch (err) {
      const detail = err instanceof Error ? err.message : String(err)
      setError(`Couldn't create the session: ${detail}. Check that the folder exists, then try again.`)
      setCreating(false)
    }
  }

  return (
    <div className="dialog-scrim">
      <div
        ref={dialogRef}
        className="dialog new-session-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="new-session-title"
      >
        <h2 className="dialog__title" id="new-session-title">New Session</h2>
        <div className="dialog__body">
          <label className="new-session-modal__section-label">Working Directory</label>
          <div className="new-session-modal__folder-row">
            <div className={`new-session-modal__folder-display${selectedDir ? ' new-session-modal__folder-display--active' : ''}`}>
              {selectedDir || 'Home directory (default)'}
            </div>
            <button
              className="button button--secondary"
              onClick={() => void handleBrowse()}
              disabled={browseLoading}
            >
              {browseLoading ? 'Browsing…' : 'Browse…'}
            </button>
          </div>
          {error && <p className="dialog__error" role="alert">{error}</p>}
        </div>
        <div className="dialog__buttons">
          <button className="button button--secondary" data-dialog-cancel="" onClick={onClose}>Cancel</button>
          <button
            className="button button--primary"
            data-dialog-primary=""
            onClick={() => void handleConfirm()}
            disabled={creating}
          >
            {creating ? 'Creating…' : 'Create Session'}
          </button>
        </div>
      </div>
    </div>
  )
}
