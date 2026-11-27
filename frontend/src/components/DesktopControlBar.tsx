import React, { useRef, useState } from 'react'
import { ChevronDownIcon, KeyIcon } from '@heroicons/react/24/outline'
import { DesktopSendKeysMenu } from './DesktopSendKeysMenu'

export interface DesktopControlBarProps {
  /** Taps a chord on the input channel (desktopInput.sendChord). */
  sendChord: (codes: string[]) => void
  /** Called with true while any bar menu or dialog is open: the view releases everything held. */
  onMenuOpenChange: (open: boolean) => void
  /** Called after a Send Keys item ran and its menu closed: the view returns focus to the stage if it had it. */
  onSendKeysDone: () => void
  /** The picture is a window share: the Send Keys items are greyed. */
  windowShare: boolean
}

/**
 * Phase 20-07 — the viewer's control bar (20-UI-SPEC Surface B): a top-center floating
 * toolbar. Rendered by DesktopView only for a viewer that may control the Mac.
 */
export function DesktopControlBar({
  sendChord, onMenuOpenChange, onSendKeysDone, windowShare,
}: DesktopControlBarProps): React.ReactElement {
  const sendKeysBtn = useRef<HTMLButtonElement>(null)
  const [sendKeys, setSendKeys] = useState<{ x: number; y: number } | null>(null)
  const chose = useRef(false)

  function openSendKeys() {
    const r = sendKeysBtn.current!.getBoundingClientRect()
    chose.current = false
    setSendKeys({ x: r.left, y: r.bottom })
    onMenuOpenChange(true)
  }

  return (
    <div className="desktop-bar" role="toolbar" aria-label="Desktop controls">
      <button
        ref={sendKeysBtn}
        type="button"
        className="desktop-bar__item"
        aria-label="Send Keys"
        title="Send Keys"
        aria-haspopup="menu"
        aria-expanded={sendKeys !== null}
        onClick={() => { if (!sendKeys) openSendKeys() }}
      >
        <KeyIcon className="desktop-bar__icon" aria-hidden="true" />
        <span className="desktop-bar__label">Send Keys</span>
        <ChevronDownIcon className="desktop-bar__icon" aria-hidden="true" />
      </button>
      {sendKeys && (
        <DesktopSendKeysMenu
          anchor={sendKeys}
          invoker={sendKeysBtn.current}
          windowShare={windowShare}
          sendChord={(codes) => {
            chose.current = true
            sendChord(codes)
          }}
          onClose={() => {
            setSendKeys(null)
            onMenuOpenChange(false)
            if (chose.current) onSendKeysDone()
          }}
        />
      )}
    </div>
  )
}
