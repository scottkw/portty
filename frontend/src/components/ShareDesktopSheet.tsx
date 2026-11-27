import React, { useEffect, useRef, useState } from 'react'
import {
  AcknowledgeCTDisclosure,
  GetDesktopShare,
  HasCTDisclosure,
  StartDesktopShare,
  StopDesktopShare,
} from '../wailsjs/go/main/App'
import { ClipboardSetText } from '../wailsjs/wailsjs/runtime/runtime'
import { useDialogFocus } from '../lib/useDialogFocus'
import { type TailscaleHealth } from '../lib/tailscale'
import { cleanDetail, type ShareState } from '../lib/desktopShare'
import * as copy from '../lib/shareDesktopCopy'
import { Row } from './SettingsRow'
import { ViewerList } from './ViewerList'

interface ShareDesktopSheetProps {
  open: boolean
  onClose: () => void
  tailscaleHealth: TailscaleHealth | null
  /** The latest desktopshare:state (who is watching). */
  state: ShareState
  /** A desktopshare:stopped arrived: the share ended under the sheet (daemon restart, key regenerated). */
  stoppedNotice: boolean
  /** Where focus goes on close when the sheet opened itself (no opener button). */
  returnFocusTo?: HTMLElement | null
}

/** How long Copy Link reads "Copied!". */
const COPIED_MS = 2000

// checking: reading the acknowledgement and any running share; starting: a start is in flight;
// sharing: the link row and the Watching list; idle: Not sharing.
type Phase = 'checking' | 'starting' | 'sharing' | 'idle'

/**
 * Share Desktop (22-UI-SPEC Surface 2): one click starts a share when the Certificate
 * Transparency line has been acknowledged, then Copy Link, who is watching, Stop Sharing.
 * It mounts its content only while open, so state is fresh on every open. Closing never
 * stops the share. The link lives in a ref: it is copied, never rendered or logged.
 */
export function ShareDesktopSheet(props: ShareDesktopSheetProps): React.ReactElement | null {
  if (!props.open) return null
  return <ShareDesktopSheetContent {...props} />
}

function ShareDesktopSheetContent({ onClose, tailscaleHealth, state, returnFocusTo }: ShareDesktopSheetProps): React.ReactElement {
  const [phase, setPhase] = useState<Phase>('checking')
  // null: not read yet.
  const [acknowledged, setAcknowledged] = useState<boolean | null>(null)
  const [startError, setStartError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [stopping, setStopping] = useState(false)
  const dialogRef = useRef<HTMLDivElement>(null)
  const urlRef = useRef('')
  const alive = useRef(true)
  const opened = useRef(false)
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useDialogFocus(dialogRef, { onCancel: onClose, initialFocus: 'primary' })

  const tailscaleOff = !!tailscaleHealth && !tailscaleHealth.connected

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      if (copiedTimer.current) clearTimeout(copiedTimer.current)
      returnFocusTo?.focus?.()
    }
  }, [returnFocusTo])

  // The primary button changes with the phase; keep focus inside the dialog when the one that had it goes.
  useEffect(() => {
    const dialog = dialogRef.current
    if (dialog && !dialog.contains(document.activeElement)) {
      dialog.querySelector<HTMLElement>('[data-dialog-primary]')?.focus()
    }
  }, [phase])

  async function begin(): Promise<void> {
    setPhase('starting')
    setStartError(null)
    setActionError(null)
    const r = await StartDesktopShare()
    if (!alive.current) return
    if (r.errorCode) {
      setStartError(copy.shareDesktopStartError(cleanDetail(r.detail)))
      setPhase('idle')
      return
    }
    urlRef.current = r.url
    setPhase('sharing')
  }

  // On open: a running share shows Sharing with its link; otherwise one click starts it
  // once the certificate line has been acknowledged and Tailscale is up.
  useEffect(() => {
    if (opened.current) return
    opened.current = true
    void (async () => {
      const ack = await HasCTDisclosure().catch(() => false)
      const info = await GetDesktopShare().catch(() => null)
      if (!alive.current) return
      setAcknowledged(ack)
      if (info?.active && info.url) {
        urlRef.current = info.url
        setPhase('sharing')
      } else if (ack && !tailscaleOff) {
        await begin()
      } else {
        setPhase('idle')
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once per open, by design
  }, [])

  async function startSharing(): Promise<void> {
    if (phase !== 'idle') return
    // The line was shown before this click; the click is the consent.
    if (!acknowledged) {
      await AcknowledgeCTDisclosure().catch(() => {})
      if (!alive.current) return
      setAcknowledged(true)
    }
    await begin()
  }

  async function copyLink(): Promise<void> {
    if (phase !== 'sharing') return
    setActionError(null)
    let ok = false
    try {
      ok = (await ClipboardSetText(urlRef.current)) !== false
    } catch {
      ok = false
    }
    if (!alive.current) return
    if (!ok) {
      setActionError(copy.COPY_LINK_FAILED)
      return
    }
    setCopied(true)
    if (copiedTimer.current) clearTimeout(copiedTimer.current)
    copiedTimer.current = setTimeout(() => setCopied(false), COPIED_MS)
  }

  async function stopSharing(): Promise<void> {
    if (stopping) return
    setStopping(true)
    setActionError(null)
    const r = await StopDesktopShare()
    if (!alive.current) return
    setStopping(false)
    if (r.errorCode) {
      setActionError(copy.shareDesktopStopError(cleanDetail(r.detail)))
      return
    }
    // The sharer's own choice: stay open in Not sharing and do not start again.
    urlRef.current = ''
    setCopied(false)
    setPhase('idle')
  }

  const sharing = phase === 'sharing'
  const inert = phase === 'checking' || phase === 'starting'
  const host = (tailscaleHealth?.domain ?? '').replace(/\.ts\.net$/, '')
  const showCT = acknowledged === false && !!tailscaleHealth?.domain

  return (
    <div className="dialog-scrim">
      <div
        ref={dialogRef}
        className="dialog dialog--setup dialog--share"
        role="dialog"
        aria-modal="true"
        aria-labelledby="share-desktop-title"
      >
        <h2 className="dialog__title" id="share-desktop-title">{copy.SHARE_SHEET_TITLE}</h2>
        <div className="dialog__body">
          {sharing ? (
            <>
              <div className="settings-group">
                <Row label={copy.SHARE_LINK_LABEL} description={copy.SHARE_LINK_DESCRIPTION}>
                  <button
                    type="button"
                    className="button button--secondary button--copy"
                    aria-disabled={tailscaleOff}
                    onClick={() => { if (!tailscaleOff) void copyLink() }}
                  >
                    {copied ? copy.COPIED : copy.COPY_LINK}
                  </button>
                </Row>
              </div>
              <ViewerList viewers={state.viewers} />
              <p className="settings-sheet__footnote">{copy.SHARE_FOOTNOTE}</p>
              {actionError && <p className="settings-sheet__error" role="alert">{actionError}</p>}
            </>
          ) : (
            <>
              {phase === 'starting' && <p className="settings-row__description">{copy.SHARE_STARTING}</p>}
              {phase === 'idle' && <p className="settings-row__description">{copy.SHARE_NOT_SHARING_DESCRIPTION}</p>}
              {phase === 'idle' && showCT && (
                <p className="settings-row__description share-desktop__note">{copy.SHARE_CT_LINE(host)}</p>
              )}
              {startError && <p className="dialog__error" role="alert">{startError}</p>}
            </>
          )}
        </div>
        <div className="dialog__buttons share-desktop__footer">
          {sharing ? (
            <>
              <button
                type="button"
                className="button button--secondary share-desktop__stop"
                aria-disabled={stopping}
                onClick={() => void stopSharing()}
              >
                {copy.STOP_SHARING}
              </button>
              <button type="button" className="button button--primary" data-dialog-primary="" onClick={onClose}>
                {copy.DONE}
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                className="button button--secondary"
                aria-disabled={inert}
                onClick={() => { if (!inert) onClose() }}
              >
                {copy.NOT_NOW}
              </button>
              <button
                type="button"
                className="button button--primary"
                data-dialog-primary=""
                aria-disabled={inert}
                onClick={() => void startSharing()}
              >
                {copy.START_SHARING}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
