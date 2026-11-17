import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Switch } from './Switch'
import { GetSessionShare, HasCTDisclosure, SetSessionShared } from '../wailsjs/go/main/App'
import { ClipboardSetText } from '../wailsjs/wailsjs/runtime/runtime'
import { TAILSCALE_OFF_REASON } from '../lib/tailscale'
import {
  COPIED,
  COPY,
  COPY_FAILED,
  SHARE_DESCRIPTION,
  SHARE_STARTING,
  SHARE_SWITCH_LABEL,
  SHARE_WARNING,
  shareCtLine,
  shareLinkLoadError,
  shareStartError,
  shareStopError,
} from '../lib/shareCopy'

export interface SharePopoverProps {
  sessionId: string
  sessionName: string
  /** The session's share state from the sessions poll; a change refetches the link. */
  shared: boolean
  /** The toolbar Share button: the popover hangs 8px under it with the arrow centred on it. */
  anchor: HTMLElement | null
  popoverId: string
  tailscaleOff: boolean
  /** The Mac's tailnet certificate name (e.g. office-air.tail1234.ts.net) for the CT line. */
  tailscaleDomain: string
  onClose: () => void
  onSharedChange: (sessionId: string, shared: boolean) => void
}

const WIDTH = 320
const INSET = 12 // minimum gap to either window edge
const ARROW = 16

type Failure = { kind: 'start' | 'stop' | 'copy'; text: string }

/**
 * Portty 04-05 (UI-SPEC "Share popover"): the one Share switch for a session,
 * in a non-modal popover under the toolbar Share button. On shows the full
 * link with Copy and the locked warning line. A pending start or stop keeps
 * running after close; its result reaches the toolbar and sidebar through
 * onSharedChange and the sessions poll.
 */
export function SharePopover({
  sessionId,
  sessionName,
  shared: sharedProp,
  anchor,
  popoverId,
  tailscaleOff,
  tailscaleDomain,
  onClose,
  onSharedChange,
}: SharePopoverProps): React.ReactPortal {
  const ref = useRef<HTMLDivElement>(null)
  const linkRef = useRef<HTMLInputElement>(null)
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const busyRef = useRef(false)
  const [shared, setShared] = useState(sharedProp)
  const [url, setUrl] = useState('')
  const [loadError, setLoadError] = useState('') // GetSessionShare's failure detail
  const [busy, setBusy] = useState<'starting' | 'stopping' | null>(null)
  const [error, setError] = useState<Failure | null>(null)
  const [ctAcked, setCtAcked] = useState(true) // hidden until HasCTDisclosure answers
  const [copied, setCopied] = useState(false)
  const [pos, setPos] = useState({ left: 0, top: 0, width: WIDTH, arrowLeft: (WIDTH - ARROW) / 2 })

  const switchId = `${popoverId}-switch`
  const reasonId = `${popoverId}-reason`

  // Read the daemon's state on open, and again when the poll says another
  // entry point (sidebar menu, `portty share`) flipped it.
  useEffect(() => {
    if (busyRef.current) return
    let cancelled = false
    GetSessionShare(sessionId)
      .then((r) => {
        if (cancelled || busyRef.current) return
        // On failure keep the poll's state; a shared session shows the error instead of a link.
        if (r.errorCode) { setLoadError(r.errorDetail); return }
        setLoadError('')
        setShared(r.shared)
        setUrl(r.url)
      })
      .catch((err) => { if (!cancelled && !busyRef.current) setLoadError(String(err)) })
    return () => { cancelled = true }
  }, [sessionId, sharedProp])

  useEffect(() => {
    HasCTDisclosure().then(setCtAcked).catch(() => setCtAcked(false))
  }, [])

  useEffect(() => () => { if (copiedTimer.current) clearTimeout(copiedTimer.current) }, [])

  // An error from before Tailscale changed state is stale; don't show it beside the reason line.
  useEffect(() => { setError(null) }, [tailscaleOff])

  // Show the end of the link (session path and token), the part that differs between links.
  const showLink = !busy && !tailscaleOff && shared && !!url
  useLayoutEffect(() => {
    const el = linkRef.current
    if (showLink && el) el.scrollLeft = el.scrollWidth
  }, [showLink, url])

  const place = useCallback(() => {
    if (!anchor) return
    const r = anchor.getBoundingClientRect()
    const width = Math.min(WIDTH, window.innerWidth - 2 * INSET)
    // Right edge on the button's right edge, at least INSET from both window edges.
    const left = Math.max(INSET, Math.min(r.right - width, window.innerWidth - INSET - width))
    const centre = r.left + r.width / 2 - left
    const arrowLeft = Math.max(INSET, Math.min(centre - ARROW / 2, width - INSET - ARROW))
    setPos({ left, top: r.bottom + 8, width, arrowLeft })
  }, [anchor])

  useLayoutEffect(() => {
    place()
    window.addEventListener('resize', place)
    return () => window.removeEventListener('resize', place)
  }, [place])

  // Focus starts on the switch.
  useEffect(() => {
    document.getElementById(switchId)?.focus()
  }, [switchId])

  // Esc closes and returns focus to the button; a press outside the popover
  // and the button closes without moving focus; so does the window losing key status.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.stopPropagation()
      anchor?.focus()
      onClose()
    }
    const onPointer = (e: PointerEvent) => {
      const t = e.target as Node
      if (ref.current?.contains(t) || anchor?.contains(t)) return
      onClose()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onPointer, true)
    window.addEventListener('blur', onClose)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onPointer, true)
      window.removeEventListener('blur', onClose)
    }
  }, [anchor, onClose])

  // Tab and Shift+Tab wrap among the switch, the link field and Copy.
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key !== 'Tab' || !ref.current) return
    const items = Array.from(ref.current.querySelectorAll<HTMLElement>('button:not([disabled]), input'))
    if (items.length === 0) return
    const i = items.indexOf(document.activeElement as HTMLElement)
    const next = items[(i + (e.shiftKey ? items.length - 1 : 1) + items.length) % items.length]
    e.preventDefault()
    next.focus()
  }

  async function toggle() {
    if (busyRef.current) return
    const next = !shared
    busyRef.current = true
    setBusy(next ? 'starting' : 'stopping')
    setShared(next)
    setError(null)
    setCopied(false)
    let r: { shared: boolean; url: string; errorCode: string; errorDetail: string }
    try {
      r = await SetSessionShared(sessionId, next)
    } catch (err) {
      r = { shared: !next, url, errorCode: next ? 'start_failed' : 'stop_failed', errorDetail: String(err) }
    }
    busyRef.current = false
    setBusy(null)
    if (r.errorCode) {
      setShared(!next)
      setError(next
        ? { kind: 'start', text: shareStartError(r.errorCode, r.errorDetail) }
        : { kind: 'stop', text: shareStopError(r.errorDetail) })
      return
    }
    setShared(r.shared)
    setUrl(r.url)
    if (r.shared) setCtAcked(true) // the daemon records the disclosure on the first share
    onSharedChange(sessionId, r.shared)
  }

  async function copy() {
    let ok = false
    try {
      ok = await ClipboardSetText(url)
    } catch {
      ok = false
    }
    if (copiedTimer.current) clearTimeout(copiedTimer.current)
    if (ok) {
      setError((e) => (e?.kind === 'copy' ? null : e))
      setCopied(true)
      copiedTimer.current = setTimeout(() => setCopied(false), 2000)
    } else {
      setCopied(false)
      setError({ kind: 'copy', text: COPY_FAILED })
      linkRef.current?.select()
    }
  }

  const line = (text: string, id?: string) => <p className="share-popover__line" id={id}>{text}</p>
  const errorLine = (kinds: Failure['kind'][]) =>
    error && kinds.includes(error.kind) ? <p className="share-popover__line share-popover__error">{error.text}</p> : null

  let body: React.ReactNode = null
  if (busy === 'starting') {
    body = line(SHARE_STARTING)
  } else if (busy === 'stopping') {
    body = null
  } else if (tailscaleOff) {
    body = <>{errorLine(['start'])}{line(TAILSCALE_OFF_REASON, reasonId)}{errorLine(['stop'])}</>
  } else if (shared) {
    body = showLink ? (
      <>
        <div className="share-popover__link-row">
          <input
            ref={linkRef}
            className="share-popover__link"
            type="text"
            readOnly
            value={url}
            aria-label="Share link"
            spellCheck={false}
            onFocus={(e) => e.currentTarget.select()}
            onClick={(e) => e.currentTarget.select()}
          />
          <button type="button" className="button button--secondary share-popover__copy" onClick={() => void copy()}>
            {copied ? COPIED : COPY}
          </button>
        </div>
        {line(SHARE_WARNING)}
        {errorLine(['stop', 'copy'])}
      </>
    ) : loadError ? (
      <>
        <p className="share-popover__line share-popover__error">{shareLinkLoadError(loadError)}</p>
        {errorLine(['stop'])}
      </>
    ) : (
      line(SHARE_STARTING)
    )
  } else {
    body = (
      <>
        {errorLine(['start'])}
        {line(SHARE_DESCRIPTION)}
        {!ctAcked && tailscaleDomain && line(shareCtLine(tailscaleDomain))}
      </>
    )
  }

  const blocked = tailscaleOff && !shared && !busy

  return createPortal(
    <div
      ref={ref}
      id={popoverId}
      className="share-popover"
      role="dialog"
      aria-label={`Share “${sessionName}”`}
      style={{ left: pos.left, top: pos.top, width: pos.width }}
      onKeyDown={onKeyDown}
    >
      <svg className="share-popover__arrow" style={{ left: pos.arrowLeft }} width="16" height="8" viewBox="0 0 16 8" aria-hidden="true">
        <polygon className="share-popover__arrow-fill" points="0,8 8,0 16,8" />
        <path className="share-popover__arrow-stroke" d="M0.5 8 L8 0.5 L15.5 8" />
      </svg>
      <div className="share-popover__row">
        <label htmlFor={switchId}>{SHARE_SWITCH_LABEL}</label>
        <Switch
          id={switchId}
          checked={shared}
          onChange={() => void toggle()}
          saving={busy !== null}
          blocked={blocked}
          describedBy={blocked ? reasonId : undefined}
        />
      </div>
      <div className={`share-popover__state${showLink ? ' share-popover__state--link' : ''}`} aria-live="polite">{body}</div>
    </div>,
    document.body,
  )
}
