import React, { useCallback, useEffect, useRef, useState } from 'react'
import {
  GetDesktopPermissions,
  OpenDesktopPermissionPane,
  RunDesktopTestCapture,
} from '../wailsjs/go/main/App'
import { useDialogFocus } from '../lib/useDialogFocus'

interface DesktopSetupSheetProps {
  open: boolean
  onClose: () => void
  /** Where focus goes on close when the sheet opened itself (no opener button). */
  returnFocusTo?: HTMLElement | null
}

// How often the sheet asks the daemon for the live grants while it is open.
// There is deliberately no timeout: the user may take as long as they like.
const PERMISSION_POLL_MS = 2000

type Capture =
  | { phase: 'idle' }
  | { phase: 'capturing' }
  | { phase: 'done'; image: string }
  | { phase: 'failed' }

const NOTE_PICKER =
  'macOS 27.2 and later show a dialog the first time Portty captures the screen, asking whether it may bypass the system private window picker. Choose Allow. It does not come back.'
const NOTE_LOGIN_ITEMS =
  'Portty also adds an entry under Login Items so the desktop stays reachable after a restart. macOS lists it under the name of the developer who signed Portty, not "Portty", and its status text can look out of date. Leave it on.'

/**
 * Set Up Desktop Sharing (19-UI-SPEC Surface 2, DPRM-01): the two permissions
 * with live status, an Open System Settings button each, a Test Capture that
 * shows what viewers will see, and Done. It mounts its content only while open,
 * so state is fresh on every open. Closing never turns anything off.
 */
export function DesktopSetupSheet(props: DesktopSetupSheetProps): React.ReactElement | null {
  if (!props.open) return null
  return <DesktopSetupSheetContent onClose={props.onClose} returnFocusTo={props.returnFocusTo} />
}

function DesktopSetupSheetContent({ onClose, returnFocusTo }: Omit<DesktopSetupSheetProps, 'open'>): React.ReactElement {
  // null: not read yet (or no answer yet). A failed read never becomes false.
  const [sr, setSr] = useState<boolean | null>(null)
  const [ax, setAx] = useState<boolean | null>(null)
  const [capture, setCapture] = useState<Capture>({ phase: 'idle' })
  const dialogRef = useRef<HTMLDivElement>(null)
  const askSeq = useRef(0)
  const alive = useRef(true)

  useDialogFocus(dialogRef, { onCancel: onClose, initialFocus: 'primary' })

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      returnFocusTo?.focus?.()
    }
  }, [returnFocusTo])

  const refresh = useCallback(async () => {
    const seq = ++askSeq.current
    const r = await GetDesktopPermissions()
    // The latest answer wins; an unknown one keeps what the rows already show.
    if (!alive.current || seq !== askSeq.current || r.errorCode) return
    setSr(r.sr)
    setAx(r.ax)
  }, [])

  useEffect(() => {
    void refresh()
    const timer = setInterval(() => void refresh(), PERMISSION_POLL_MS)
    // Back from System Settings: read at once rather than at the next tick.
    const onFocus = (): void => void refresh()
    window.addEventListener('focus', onFocus)
    return () => {
      clearInterval(timer)
      window.removeEventListener('focus', onFocus)
    }
  }, [refresh])

  // Test Capture needs only Screen Recording, and only when it is known to be off.
  const blocked = sr === false
  const capturing = capture.phase === 'capturing'

  async function runCapture(): Promise<void> {
    if (blocked || capturing) return
    setCapture({ phase: 'capturing' })
    const r = await RunDesktopTestCapture()
    if (!alive.current) return
    setCapture(r.errorCode || !r.imageDataURL ? { phase: 'failed' } : { phase: 'done', image: r.imageDataURL })
  }

  function openPane(kind: 'screen' | 'accessibility'): void {
    // A refusal only means no pane opened; the row stays as it was.
    OpenDesktopPermissionPane(kind).catch(() => {})
  }

  return (
    <div className="dialog-scrim">
      <div
        ref={dialogRef}
        className="dialog dialog--setup"
        role="dialog"
        aria-modal="true"
        aria-labelledby="desktop-setup-title"
      >
        <h2 className="dialog__title" id="desktop-setup-title">Set Up Desktop Sharing</h2>
        <div className="dialog__body">
          <p className="settings-row__description">
            Portty needs two permissions to show this Mac&apos;s screen. Grant them here, once, while you are at the Mac.
          </p>

          <div className="settings-group">
            <PermissionRow
              label="Screen Recording"
              granted={sr}
              howTo="Open System Settings, go to Privacy & Security, then Screen & System Audio Recording, and turn on Portty."
              onOpen={() => openPane('screen')}
            />
            <PermissionRow
              label="Accessibility"
              granted={ax}
              howTo="Open System Settings, go to Privacy & Security, then Accessibility, and turn on Portty. Portty uses it to tell a still screen from an unapproved one. Asked now so a later update does not need another trip to this Mac."
              onOpen={() => openPane('accessibility')}
            />
          </div>

          <div className="settings-group">
            <div className="settings-row">
              <div className="settings-row__text">
                <span className="settings-row__label">Test Capture</span>
                <p className="settings-row__description">Checks that Portty can see this Mac&apos;s main display.</p>
                {blocked && (
                  <p className="settings-row__description" id="desktop-setup-blocked">Turn on Screen Recording first.</p>
                )}
              </div>
              <div className="settings-row__control">
                <button
                  type="button"
                  className="button button--secondary"
                  aria-disabled={blocked || capturing}
                  aria-describedby={blocked ? 'desktop-setup-blocked' : undefined}
                  onClick={() => void runCapture()}
                >
                  {capturing ? 'Capturing…' : capture.phase === 'idle' ? 'Test Capture' : 'Test Again'}
                </button>
              </div>
            </div>
            {capturing && <div className="settings-row desktop-setup__result"><div className="desktop-setup__placeholder" data-testid="capture-placeholder" /></div>}
            {capture.phase === 'done' && (
              <div className="settings-row desktop-setup__result">
                <img className="desktop-setup__thumb" src={capture.image} alt="Preview of this Mac's main display" />
                <p className="settings-row__description" role="status">Capture works. This is what viewers will see.</p>
              </div>
            )}
            {capture.phase === 'failed' && (
              <div className="settings-row desktop-setup__result">
                <p className="dialog__error" role="alert">
                  Couldn&apos;t capture the screen. Check that Screen Recording is on above. If a macOS dialog is waiting on the screen, choose Allow, then try again.
                </p>
              </div>
            )}
          </div>

          <p className="settings-row__description">{NOTE_PICKER}</p>
          <p className="settings-row__description">{NOTE_LOGIN_ITEMS}</p>
        </div>
        <div className="dialog__buttons">
          <button type="button" className="button button--primary" data-dialog-primary="" onClick={onClose}>
            Done
          </button>
        </div>
      </div>
    </div>
  )
}

function PermissionRow({ label, granted, howTo, onOpen }: {
  label: string
  granted: boolean | null
  howTo: string
  onOpen: () => void
}): React.ReactElement {
  const state = granted === null ? 'checking' : granted ? 'ok' : 'warn'
  return (
    <div className="settings-row">
      <div className="settings-row__text">
        <span className="settings-row__label">{label}</span>
        <span className="ts-status" aria-live="polite">
          <span className={`ts-status__dot${state === 'checking' ? '' : ` ts-status__dot--${state}`}`} aria-hidden="true" />
          <span className={`ts-status__text${granted ? ' ts-status__text--granted' : ''}`}>
            {granted === null ? 'Checking…' : granted ? 'Granted' : 'Not granted'}
          </span>
        </span>
        {granted === false && <p className="settings-row__description">{howTo}</p>}
      </div>
      <div className="settings-row__control">
        <button type="button" className="button button--secondary" onClick={onOpen}>
          Open System Settings
        </button>
      </div>
    </div>
  )
}
