import { useEffect, useState } from 'react'
import { GetShellPath, OpenDirectoryDialog } from '../wailsjs/go/main/App'

const LAST_DIR_KEY = 'agenthub:lastWorkDir'
const SHELL_FALLBACK = 'Default login shell'

export interface NewSessionModalProps {
  isOpen: boolean
  /** Called with the chosen working directory ("" = home directory). */
  onConfirm: (workDir: string) => void
  onClose: () => void
}

export function NewSessionModal({ isOpen, onConfirm, onClose }: NewSessionModalProps) {
  const [selectedDir, setSelectedDir] = useState(() => localStorage.getItem(LAST_DIR_KEY) ?? '')
  const [browseLoading, setBrowseLoading] = useState(false)
  const [creating, setCreating] = useState(false)

  // Resolved shell path, re-fetched on every open so Settings → Paths changes
  // show immediately. "" while pending, on error, or on an empty result.
  const [resolvedShellPath, setResolvedShellPath] = useState('')

  useEffect(() => {
    if (!isOpen) return
    setResolvedShellPath('')
    GetShellPath()
      .then((p) => setResolvedShellPath(p ?? ''))
      .catch(() => setResolvedShellPath(''))
  }, [isOpen])

  if (!isOpen) return null

  async function handleBrowse() {
    setBrowseLoading(true)
    try {
      const path = await OpenDirectoryDialog(selectedDir)
      if (path !== '') {
        setSelectedDir(path)
        localStorage.setItem(LAST_DIR_KEY, path)
      }
    } finally {
      setBrowseLoading(false)
    }
  }

  function handleConfirm() {
    setCreating(true)
    onConfirm(selectedDir)
  }

  return (
    <div className="new-session-overlay" onClick={onClose}>
      <div className="new-session-modal" onClick={(e) => e.stopPropagation()}>
        <div className="new-session-modal__header">
          <h2>New Session</h2>
          <button className="new-session-modal__close" aria-label="Close" onClick={onClose}>&times;</button>
        </div>
        <div className="new-session-modal__body">
          <div className="new-session-modal__section">
            <label className="new-session-modal__section-label">Working Directory</label>
            <div className="new-session-modal__folder-row">
              <div className={`new-session-modal__folder-display${selectedDir ? ' new-session-modal__folder-display--active' : ''}`}>
                {selectedDir || 'Home directory (default)'}
              </div>
              <button
                className="new-session-modal__browse-btn"
                onClick={() => void handleBrowse()}
                disabled={browseLoading}
              >
                {browseLoading ? 'Browsing…' : 'Browse…'}
              </button>
            </div>
          </div>
          <div className="new-session-modal__section">
            <label className="new-session-modal__section-label">Shell</label>
            {/* Read-only, non-focusable: the shell is changed in Settings → Paths. */}
            <div className="new-session-modal__folder-display new-session-modal__folder-display--mono">
              {resolvedShellPath || SHELL_FALLBACK}
            </div>
            <p className="new-session-modal__helper">Starts as a login shell. Change it in Settings → Paths.</p>
          </div>
        </div>
        <div className="new-session-modal__footer">
          <button className="new-session-modal__btn--close" onClick={onClose}>Close</button>
          <button
            className="new-session-modal__btn--create"
            onClick={handleConfirm}
            disabled={creating}
          >
            {creating ? 'Creating…' : 'Create Session'}
          </button>
        </div>
      </div>
    </div>
  )
}
