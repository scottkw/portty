import React, { useId, useRef, useState } from 'react'
import type { DesktopClient, DesktopUnlockMsg } from '../lib/desktopClient'
import { encodeUnlock, zeroBytes } from '../lib/unlockSecret'

export const UNLOCK_WAITING = "Portty never saves it. Wrong tries count toward the Mac's own limit."

export interface DesktopUnlockPanelProps {
  client: DesktopClient
  unlock: DesktopUnlockMsg | null
  reconnecting: boolean
  needsAccessibility: boolean
  phone: boolean
}

/**
 * The Unlock This Mac panel (UNLK-01, 23-UI-SPEC Surface 2). The password input is uncontrolled: the
 * text lives in the DOM value between keystroke and submit, never in React state or props.
 */
export function DesktopUnlockPanel({ client, phone }: DesktopUnlockPanelProps): React.ReactElement {
  const headingId = useId()
  const statusId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [hasText, setHasText] = useState(false)

  function submit() {
    const el = inputRef.current
    if (!el || el.value === '') return
    const bytes = encodeUnlock(el.value)
    // Emptied in the same synchronous step as the read; the array is zeroed once it has been sent.
    el.value = ''
    setHasText(false)
    try {
      client.sendUnlock(bytes)
    } finally {
      zeroBytes(bytes)
    }
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
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.nativeEvent.isComposing && e.keyCode !== 229) {
              e.preventDefault()
              submit()
            }
          }}
        />
        <button type="button" className="button button--primary desktop-unlock__button" aria-disabled={!hasText} onClick={submit}>
          Unlock
        </button>
      </div>
      <div id={statusId} role="status" aria-live="polite" className="desktop-unlock__status">
        <span className="desktop-unlock__line desktop-unlock__line--quiet">{UNLOCK_WAITING}</span>
      </div>
    </div>
  )
}
