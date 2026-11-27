import React, { useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { ContextMenu, type ContextMenuEntry } from './ContextMenu'
import { RegenerateKeyModal } from './RegenerateKeyModal'

/** Ordinary key codes: the Mac sees a normal tap, the swap never applies (these name Cmd explicitly). */
export const CMD_TAB = ['MetaLeft', 'Tab']
export const CMD_SPACE = ['MetaLeft', 'Space']
export const LOCK_SCREEN = ['ControlLeft', 'MetaLeft', 'KeyQ']

export const WINDOW_SHARE_CAPTION = 'Not available in a window share. Share the display to use these.'
const CAPTION_ID = 'desktop-sendkeys-caption'

export interface DesktopSendKeysMenuProps {
  anchor: { x: number; y: number }
  /** Gets focus back when the menu or its confirm closes. */
  invoker: HTMLElement | null
  onClose: () => void
  /** Taps the chord: every code down in order, then every code up in reverse, as one burst. */
  sendChord: (codes: string[]) => void
  /** A window share: all three items are greyed and the confirm cannot open. */
  windowShare: boolean
  /** Why everything is greyed, when it is (for example Locked); the items are inert. */
  disabledReason?: string
  /** Phase 21-10: open upward from the anchor (the iPhone bars sit at the bottom of the screen). */
  above?: boolean
  /** A guest page (22-07): Lock Screen… is not offered, only Cmd+Tab and Cmd+Space. */
  guest?: boolean
}

/**
 * Phase 20-07 — the Send Keys menu (DCTL-03), standalone so the Phase 21 Desktop tab and
 * iPhone key bar place it unchanged. It owns the Lock This Mac confirm: only the confirm
 * sends Ctrl+Cmd+Q. Mounted while open; it calls onClose once everything it opened is gone.
 */
export function DesktopSendKeysMenu({
  anchor, invoker, onClose, sendChord, windowShare, disabledReason, above, guest = false,
}: DesktopSendKeysMenuProps): React.ReactElement {
  const [menuOpen, setMenuOpen] = useState(true)
  const [confirm, setConfirm] = useState(false)
  // The menu closes itself after a pick; when the pick opened the confirm, the confirm owns what follows.
  const confirming = useRef(false)
  function closeConfirm() {
    setConfirm(false)
    onClose()
  }
  const inert = windowShare || disabledReason !== undefined
  const title = disabledReason
  const describedBy = windowShare ? CAPTION_ID : undefined

  const items: ContextMenuEntry[] = [
    { label: 'Cmd+Tab', onSelect: () => sendChord(CMD_TAB), disabled: inert, describedBy, title },
    { label: 'Cmd+Space', onSelect: () => sendChord(CMD_SPACE), disabled: inert, describedBy, title },
  ]
  if (!guest) {
    items.push({
      label: 'Lock Screen…',
      hint: 'Ctrl+Cmd+Q',
      separatorBefore: true,
      disabled: inert,
      describedBy,
      title,
      onSelect: () => {
        confirming.current = true
        setConfirm(true)
      },
    })
  }
  if (windowShare) items.push({ kind: 'caption', text: WINDOW_SHARE_CAPTION, id: CAPTION_ID })

  return (
    <>
      {menuOpen && (
        <ContextMenu
          className="desktop-menu"
          items={items}
          x={anchor.x}
          y={anchor.y}
          above={above}
          invoker={invoker}
          onClose={() => {
            setMenuOpen(false)
            if (!confirming.current) onClose()
          }}
        />
      )}
      {confirm &&
        createPortal(
          <RegenerateKeyModal
            isOpen
            title="Lock This Mac"
            body="The Mac will lock. To unlock it from here you will need the Mac's password."
            confirmLabel="Lock Screen"
            actingLabel="Lock Screen"
            cancelLabel="Don't Lock"
            titleId="desktop-lock-title"
            destructive={false}
            onCancel={closeConfirm}
            onConfirm={async () => {
              // A window share must not grant more than a window: the chord is never sent there.
              if (!windowShare) sendChord(LOCK_SCREEN)
              closeConfirm()
            }}
          />,
          // As ContextMenu: inside the full-screen element when there is one, or the dialog would be invisible.
          document.fullscreenElement ?? document.body,
        )}
    </>
  )
}
