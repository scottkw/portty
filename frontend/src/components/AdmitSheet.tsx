import React, { useEffect, useRef, useState } from 'react'
import { AdmitDesktopGuest, DenyDesktopGuest, type DesktopShareActionResult } from '../wailsjs/go/main/App'
import { useDialogFocus } from '../lib/useDialogFocus'
import { sanitizeName, type SharePending } from '../lib/desktopShare'
import * as copy from '../lib/shareDesktopCopy'

/** How long both buttons ignore activation after a person appears (22-UI-SPEC Surface 3, click guard). */
export const ADMIT_GUARD_MS = 600

interface AdmitSheetProps {
  /** Every guest waiting at Admit; the oldest is shown. */
  pending: SharePending[]
}

/** Oldest connection first, then id, the Watching list's order. */
function oldest(pending: SharePending[]): SharePending {
  return pending.reduce((a, b) => {
    const ta = Date.parse(a.since)
    const tb = Date.parse(b.since)
    if (ta !== tb && !Number.isNaN(ta) && !Number.isNaN(tb)) return ta < tb ? a : b
    return a.id <= b.id ? a : b
  })
}

/**
 * Admit (22-UI-SPEC Surface 3): the trust decision of the phase. A modal that names the
 * guest by login name and is answered only by a deliberate click: focus starts on the dialog,
 * Esc does nothing, neither button is the default, and both ignore activation for
 * ADMIT_GUARD_MS whenever a person appears. Render it last so it is the topmost modal.
 */
export function AdmitSheet({ pending }: AdmitSheetProps): React.ReactElement | null {
  if (pending.length === 0) return null
  return <AdmitSheetContent request={oldest(pending)} others={pending.length - 1} />
}

function AdmitSheetContent({ request, others }: { request: SharePending; others: number }): React.ReactElement {
  const dialogRef = useRef<HTMLDivElement>(null)
  const alive = useRef(true)
  // The guard is per person and restarts after a failed answer: `run` bumps on failure.
  const [run, setRun] = useState(0)
  const guardKey = `${request.id}:${run}`
  const [unlocked, setUnlocked] = useState('')
  const [busy, setBusy] = useState(false)
  // The id answered successfully: its buttons stay off until the state line moves on.
  const [answered, setAnswered] = useState('')
  const [failure, setFailure] = useState<{ id: string; text: string } | null>(null)

  // Esc is inert: a Deny is permanent for the share, and Esc is an everyday terminal key.
  useDialogFocus(dialogRef, { onCancel: () => {}, initialFocus: 'dialog' })

  useEffect(() => {
    const t = setTimeout(() => setUnlocked(guardKey), ADMIT_GUARD_MS)
    return () => clearTimeout(t)
  }, [guardKey])
  useEffect(() => {
    alive.current = true
    return () => { alive.current = false }
  }, [])

  const disabled = busy || unlocked !== guardKey || answered === request.id

  async function answer(call: (id: string) => Promise<DesktopShareActionResult>): Promise<void> {
    if (disabled) return
    const id = request.id
    setBusy(true)
    setFailure(null)
    const r = await call(id).catch((): DesktopShareActionResult => ({ errorCode: 'unavailable', detail: '' }))
    if (!alive.current) return
    setBusy(false)
    // no_request: it already ended, and the next state line moves the sheet on.
    if (r?.errorCode && r.errorCode !== 'no_request') {
      setFailure({ id, text: copy.ADMIT_ANSWER_ERROR })
      setRun((n) => n + 1)
    } else {
      setAnswered(id)
    }
  }

  const display = sanitizeName(request.displayName)
  const login = sanitizeName(request.loginName)
  const device = sanitizeName(request.nodeName)
  // An empty or look-alike display name leaves the login name as the bold line.
  const showDisplay = display !== '' && display !== login
  const headline = showDisplay ? display : login || copy.UNNAMED_VIEWER
  const error = failure?.id === request.id ? failure.text : null

  return (
    <div className="dialog-scrim">
      <div
        ref={dialogRef}
        className="dialog admit"
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        aria-labelledby="admit-title"
        aria-describedby="admit-p1 admit-p2"
      >
        <h2 className="dialog__title" id="admit-title">{copy.ADMIT_TITLE}</h2>
        <div className="dialog__body">
          <div className="admit__identity">
            <p className="admit__name"><bdi>{headline}</bdi></p>
            {showDisplay && login !== '' && (
              <div className="admit__fact">
                <span className="admit__fact-label">{copy.ADMIT_LOGIN_LABEL}</span>
                <span className="admit__fact-value"><bdi>{login}</bdi></span>
              </div>
            )}
            {device !== '' && (
              <div className="admit__fact">
                <span className="admit__fact-label">{copy.ADMIT_DEVICE_LABEL}</span>
                <span className="admit__fact-value"><bdi title={device}>{device}</bdi></span>
              </div>
            )}
          </div>
          <p className="admit__paragraph" id="admit-p1">{copy.ADMIT_PARAGRAPH_1}</p>
          <p className="admit__paragraph" id="admit-p2">{copy.ADMIT_PARAGRAPH_2}</p>
          {others > 0 && <p className="admit__paragraph">{copy.morePeopleWaiting(others)}</p>}
          {error && <p className="viewer-row__error" role="alert">{error}</p>}
        </div>
        <div className="dialog__buttons">
          {/* data-dialog-cancel orders Tab only; neither button is primary (an accidental Return must not admit). */}
          <button
            type="button"
            className="button button--secondary"
            data-dialog-cancel=""
            aria-disabled={disabled}
            onClick={() => void answer(DenyDesktopGuest)}
          >
            {copy.DENY}
          </button>
          <button
            type="button"
            className="button button--secondary"
            aria-disabled={disabled}
            onClick={() => void answer(AdmitDesktopGuest)}
          >
            {copy.ADMIT}
          </button>
        </div>
      </div>
    </div>
  )
}
