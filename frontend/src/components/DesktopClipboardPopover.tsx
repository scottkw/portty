import React, { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { CLIP_MAX_BYTES, clipByteLength } from '../lib/desktopClient'

const WIDTH = 320
const INSET = 12 // minimum gap to either window edge
const GAP = 4 // under the Clipboard button

interface ShellProps {
  /** The Clipboard button: the popover hangs 4 px under it. */
  anchor: HTMLElement | null
  /** Closes; the shell gives focus back to the anchor first. */
  onClose: () => void
  labelId: string
  descriptionId: string
  children: (close: () => void) => React.ReactNode
}

/**
 * The shared popover shell (20-UI-SPEC Surface D): role dialog, not modal, anchored under the
 * Clipboard button. Esc and a press outside close it; Tab wraps inside it. Portaled into the
 * full-screen element when there is one, or it would be invisible there (as ContextMenu).
 */
function PopoverShell({ anchor, onClose, labelId, descriptionId, children }: ShellProps): React.ReactPortal {
  const ref = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState({ left: 0, top: 0 })
  const closeRef = useRef(onClose)
  closeRef.current = onClose
  const close = () => {
    anchor?.focus()
    closeRef.current()
  }
  const closeNow = useRef(close)
  closeNow.current = close

  useLayoutEffect(() => {
    if (!anchor) return
    const r = anchor.getBoundingClientRect()
    const width = Math.min(WIDTH, window.innerWidth - 48)
    setPos({ left: Math.max(INSET, Math.min(r.left, window.innerWidth - INSET - width)), top: r.bottom + GAP })
  }, [anchor])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.stopPropagation()
      closeNow.current()
    }
    // A press on the anchor is the button's own business (it opens its menu, which closes this).
    const onPointer = (e: PointerEvent) => {
      const t = e.target as Node
      if (ref.current?.contains(t) || anchor?.contains(t)) return
      closeRef.current()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('pointerdown', onPointer, true)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('pointerdown', onPointer, true)
    }
  }, [anchor])

  // Tab wraps between the first and last control inside the popover.
  function onKeyDown(e: React.KeyboardEvent<HTMLDivElement>) {
    if (e.key !== 'Tab' || !ref.current) return
    const items = Array.from(ref.current.querySelectorAll<HTMLElement>('textarea, button'))
    const first = items[0]
    const last = items[items.length - 1]
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }

  return createPortal(
    <div
      ref={ref}
      role="dialog"
      aria-labelledby={labelId}
      aria-describedby={descriptionId}
      className="desktop-popover"
      style={{ left: pos.left, top: pos.top }}
      onKeyDown={onKeyDown}
    >
      {children(close)}
    </div>,
    document.fullscreenElement ?? document.body,
  )
}

export interface DesktopCopyPopoverProps {
  /** The Mac's text. Shown only here, for this action: never stored or logged. */
  text: string
  anchor: HTMLElement | null
  onClose: () => void
}

/**
 * Phase 20-08: the Copy from Mac fallback. Opens only when the Mac's text arrived and the browser
 * refused to write it to the clipboard: the text is selected and read-only, so Cmd+C works at once.
 */
export function DesktopCopyPopover({ text, anchor, onClose }: DesktopCopyPopoverProps): React.ReactElement {
  const id = useId()
  const area = useRef<HTMLTextAreaElement>(null)
  useEffect(() => {
    area.current?.focus()
    area.current?.select()
  }, [])
  return (
    <PopoverShell anchor={anchor} onClose={onClose} labelId={`${id}-title`} descriptionId={`${id}-text`}>
      {(close) => (
        <>
          <h2 id={`${id}-title`} className="desktop-popover__title">Copy from Mac</h2>
          <p id={`${id}-text`} className="desktop-popover__text">
            Your browser didn&apos;t let Portty write to your clipboard. Select the text below and copy it.
          </p>
          <textarea
            ref={area}
            className="desktop-popover__textarea"
            aria-label="Text from the Mac"
            readOnly
            spellCheck={false}
            value={text}
          />
          <div className="desktop-popover__buttons">
            <button type="button" className="button button--primary" onClick={close}>
              Done
            </button>
          </div>
        </>
      )}
    </PopoverShell>
  )
}

export interface DesktopPastePopoverProps {
  anchor: HTMLElement | null
  onSend: (text: string) => void
  onClose: () => void
}

/**
 * Phase 20-08: the Paste to Mac fallback. Opens only when the browser will not let Portty read the
 * clipboard: the user pastes into the box themselves (no prompt), then sends. The pasted text is not
 * guarded or altered; it is capped at the 1 MB the Mac accepts, with the reason in words.
 */
export function DesktopPastePopover({ anchor, onSend, onClose }: DesktopPastePopoverProps): React.ReactElement {
  const id = useId()
  const area = useRef<HTMLTextAreaElement>(null)
  const [value, setValue] = useState('')
  useEffect(() => area.current?.focus(), [])
  const bytes = clipByteLength(value)
  const tooLong = bytes > CLIP_MAX_BYTES
  const blocked = bytes === 0 || tooLong
  return (
    <PopoverShell anchor={anchor} onClose={onClose} labelId={`${id}-title`} descriptionId={`${id}-text`}>
      {(close) => (
        <>
          <h2 id={`${id}-title`} className="desktop-popover__title">Paste to Mac</h2>
          <p id={`${id}-text`} className="desktop-popover__text">
            Your browser didn&apos;t let Portty read your clipboard. Paste your text here, then choose Send to Mac. This
            replaces what&apos;s on the Mac&apos;s clipboard.
          </p>
          <textarea
            ref={area}
            className="desktop-popover__textarea"
            aria-label="Text to send to the Mac"
            aria-invalid={tooLong ? 'true' : undefined}
            aria-describedby={tooLong ? `${id}-error` : undefined}
            placeholder="Paste text here"
            autoCapitalize="off"
            autoComplete="off"
            autoCorrect="off"
            spellCheck={false}
            value={value}
            onChange={(e) => setValue(e.target.value)}
          />
          {tooLong && (
            <p id={`${id}-error`} role="alert" className="desktop-popover__error">
              Too long to send (limit 1 MB).
            </p>
          )}
          <div className="desktop-popover__buttons">
            <button type="button" className="button button--secondary" onClick={close}>
              Discard Text
            </button>
            <button
              type="button"
              className="button button--primary"
              aria-disabled={blocked ? 'true' : undefined}
              onClick={() => {
                if (blocked) return
                onSend(value)
                close()
              }}
            >
              Send to Mac
            </button>
          </div>
        </>
      )}
    </PopoverShell>
  )
}
