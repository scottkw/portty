import React, { useEffect, useId, useRef, useState } from 'react'
import { ClockIcon, ExclamationTriangleIcon } from '@heroicons/react/24/outline'
import type { DesktopClient, DesktopUnlockMsg } from '../lib/desktopClient'
import { encodeUnlock, zeroBytes } from '../lib/unlockSecret'

// Copy is 23-UI-SPEC's Copywriting Contract, word for word. The daemon picks a state by enum; it never supplies text.
export const UNLOCK_WAITING = "Portty never saves it. Wrong tries count toward the Mac's own limit."
const UNLOCKING = 'Unlocking…'
const RETRYING = 'Trying once more…'
const WRONG = "Unlock didn't work. Check the password and try again."
const WARN_PAUSE = 'If the next try fails, Unlock pauses for 15 minutes.'
const CAPS_LOCK = 'Caps Lock is on at the Mac. Turn it off there, then try again.'
const UNMAPPABLE = "This password has a character Portty can't type. Unlock it at the Mac instead."
const ACCESSIBILITY = 'Unlock needs Accessibility. On the Mac, allow Portty in Privacy & Security.'
const COULD_NOT_RUN = "Couldn't unlock the Mac right now. Try again."
const RECONNECTING = 'Reconnecting…'
const RATE_LIMIT_HINT = 'You can try again in a few seconds'

/** The daemon answers a submit at once; if nothing comes back in this long the channel is treated as failed. */
const NO_ANSWER_MS = 8000

type Icon = React.ComponentType<React.SVGProps<SVGSVGElement>>
interface Line {
  Icon?: Icon
  text: string
  /** quiet: secondary text; error: the failed-unlock and refusal lines; plain: the default text color. */
  tone: 'quiet' | 'error' | 'plain'
  role?: 'alert'
  /** The ticking part is aria-live off; a hidden polite line says it once. */
  countdown?: boolean
}

const pauseWords = (ms: number): string => {
  if (ms < 60000) return 'in under 1 min'
  const m = Math.floor(ms / 60000)
  return m === 1 ? 'in 1 min' : `in ${m} min`
}

export interface DesktopUnlockPanelProps {
  client: DesktopClient
  /** The daemon's last unlock message for this lock episode, or null. The viewer keeps no counters of its own. */
  unlock: DesktopUnlockMsg | null
  reconnecting: boolean
  needsAccessibility: boolean
  phone: boolean
}

/**
 * The Unlock This Mac panel (UNLK-01, 23-UI-SPEC Surface 2). The password input is uncontrolled: the
 * text lives in the DOM value between keystroke and submit, never in React state or props. The only
 * thing held about it is whether the field is empty.
 */
export function DesktopUnlockPanel({ client, unlock, reconnecting, needsAccessibility, phone }: DesktopUnlockPanelProps): React.ReactElement {
  const headingId = useId()
  const statusId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const composing = useRef(false)
  const [hasText, setHasText] = useState(false)
  // Between a local submit and the daemon's first answer, or after a send that could not go out.
  const [local, setLocal] = useState<'idle' | 'pending' | 'send_failed'>('idle')
  // The wait for a rate limit or a pause counts down from when its message arrived.
  const [clock, setClock] = useState(() => ({ stamp: Date.now(), now: Date.now() }))

  useEffect(() => {
    const t = Date.now()
    setClock({ stamp: t, now: t })
    if (unlock) setLocal('idle')
    // A wrong password puts the user back in the field (the one place focus is moved for them).
    if (unlock?.state === 'failed') inputRef.current?.focus()
  }, [unlock])

  // The field is emptied when the page is hidden and when the panel goes; a plain blur keeps the text.
  useEffect(() => {
    const el = inputRef.current
    const clear = () => {
      if (el) el.value = ''
      setHasText(false)
    }
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') clear()
    }
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pagehide', clear)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('pagehide', clear)
      if (el) el.value = ''
    }
  }, [])

  const waitLeft = unlock ? unlock.waitMs - (clock.now - clock.stamp) : 0
  const rateLimited = unlock?.state === 'rate_limited' && waitLeft > 0
  const paused = unlock?.state === 'paused' && waitLeft > 0
  useEffect(() => {
    if (!rateLimited && !paused) return
    const id = setInterval(() => setClock((c) => ({ ...c, now: Date.now() })), 1000)
    return () => clearInterval(id)
  }, [rateLimited, paused])

  useEffect(() => {
    if (local !== 'pending') return
    const id = setTimeout(() => setLocal('send_failed'), NO_ANSWER_MS)
    return () => clearTimeout(id)
  }, [local])

  // One message at a time (the States table); `inert` is the input and button being aria-disabled.
  const lines: Line[] = []
  let inert = false
  const state = unlock?.state
  if (reconnecting) {
    inert = true
    lines.push({ text: RECONNECTING, tone: 'quiet' })
  } else if (needsAccessibility || state === 'unavailable') {
    inert = true
    lines.push({ Icon: ExclamationTriangleIcon, text: ACCESSIBILITY, tone: 'plain' })
  } else if (local === 'pending' || state === 'unlocking' || state === 'unlocked') {
    inert = true
    lines.push({ Icon: ClockIcon, text: UNLOCKING, tone: 'plain' })
  } else if (state === 'retrying') {
    inert = true
    lines.push({ Icon: ClockIcon, text: RETRYING, tone: 'plain' })
  } else if (local === 'send_failed' || state === 'error') {
    lines.push({ Icon: ExclamationTriangleIcon, text: COULD_NOT_RUN, tone: 'error', role: 'alert' })
  } else if (state === 'failed') {
    lines.push({ Icon: ExclamationTriangleIcon, text: WRONG, tone: 'error', role: 'alert' })
    if (unlock!.failures >= 2) lines.push({ text: WARN_PAUSE, tone: 'plain' })
  } else if (rateLimited) {
    if (unlock!.failures >= 1) lines.push({ Icon: ExclamationTriangleIcon, text: WRONG, tone: 'error' })
    lines.push({ Icon: ClockIcon, text: `Try again in ${Math.max(1, Math.ceil(waitLeft / 1000))} s`, tone: 'plain', countdown: true })
  } else if (paused) {
    inert = true
    lines.push({
      Icon: ExclamationTriangleIcon,
      text: `Unlock is paused after 3 failed tries. Try again ${pauseWords(waitLeft)}, or unlock it at the Mac.`,
      tone: 'error',
      role: 'alert',
      countdown: true,
    })
  } else if (state === 'refused') {
    lines.push({ Icon: ExclamationTriangleIcon, text: unlock!.reason === 'caps_lock' ? CAPS_LOCK : UNMAPPABLE, tone: 'error', role: 'alert' })
  } else {
    lines.push({ text: UNLOCK_WAITING, tone: 'quiet' })
  }
  const blocked = inert || rateLimited || !hasText

  function clearField() {
    if (inputRef.current) inputRef.current.value = ''
    setHasText(false)
  }

  function submit() {
    const el = inputRef.current
    if (!el || el.value === '' || inert || rateLimited || paused) return
    const bytes = encodeUnlock(el.value)
    // Emptied in the same synchronous step as the read; the array is zeroed once it has been sent.
    el.value = ''
    setHasText(false)
    let sent = false
    try {
      sent = client.sendUnlock(bytes)
    } finally {
      zeroBytes(bytes)
    }
    setLocal(sent ? 'pending' : 'send_failed')
  }

  return (
    <div role="group" aria-labelledby={headingId} className={phone ? 'desktop-unlock desktop-unlock--phone' : 'desktop-unlock'}>
      <h2 id={headingId} className="desktop-unlock__heading">
        Unlock This Mac
      </h2>
      <div className="desktop-unlock__row">
        <input
          ref={inputRef}
          className="desktop-unlock__input"
          type="password"
          placeholder="Mac password"
          aria-label="Mac password"
          aria-describedby={statusId}
          aria-disabled={inert}
          readOnly={inert}
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          spellCheck={false}
          enterKeyHint="go"
          maxLength={256}
          data-1p-ignore=""
          data-lpignore="true"
          data-bwignore=""
          onInput={(e) => setHasText(e.currentTarget.value !== '')}
          onCompositionStart={() => {
            composing.current = true
          }}
          onCompositionEnd={() => {
            composing.current = false
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              clearField()
            } else if (e.key === 'Enter' && !composing.current && !e.nativeEvent.isComposing && e.keyCode !== 229) {
              e.preventDefault()
              submit()
            }
          }}
        />
        <button type="button" className="button button--primary desktop-unlock__button" aria-disabled={blocked} onClick={submit}>
          Unlock
        </button>
      </div>
      <div id={statusId} role="status" aria-live="polite" className="desktop-unlock__status">
        {lines.map((l, i) => (
          <div
            key={`${i}:${l.text.slice(0, 12)}`}
            role={l.role}
            className={`desktop-unlock__line desktop-unlock__line--${l.tone}`}
          >
            {l.Icon && <l.Icon className="desktop-unlock__icon" aria-hidden="true" />}
            <span aria-live={l.countdown ? 'off' : undefined}>{l.text}</span>
          </div>
        ))}
        {rateLimited && <span className="visually-hidden">{RATE_LIMIT_HINT}</span>}
      </div>
    </div>
  )
}
