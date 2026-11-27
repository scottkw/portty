import React, { useRef, useState } from 'react'
import { ChevronDownIcon, ComputerDesktopIcon, KeyIcon, WindowIcon } from '@heroicons/react/24/outline'
import type { DesktopSurfacesMsg, SurfaceKind } from '../lib/desktopClient'
import { ContextMenu, type ContextMenuEntry } from './ContextMenu'
import { DesktopSendKeysMenu } from './DesktopSendKeysMenu'

/** The label returns to the previous surface, with a notice, when no surface message answers a pick. */
export const SWITCH_TIMEOUT_MS = 5000

export const WINDOWS_CAPTION = "Menus and pop-ups don't appear in a window share. Share the display to see them."
const WINDOWS_CAPTION_ID = 'desktop-picker-caption'

export interface SurfacePick {
  kind: SurfaceKind
  id: number
  name: string
}

export interface DesktopControlBarProps {
  /** Taps a chord on the input channel (desktopInput.sendChord). */
  sendChord: (codes: string[]) => void
  /** Called with true while any bar menu or dialog is open: the view releases everything held. */
  onMenuOpenChange: (open: boolean) => void
  /** Called after a Send Keys item ran and its menu closed: the view returns focus to the stage if it had it. */
  onSendKeysDone: () => void
  /** The picture is a window share: the Send Keys items are greyed. */
  windowShare: boolean
  /** The shared surface as the daemon last said it; null until the first surface message. */
  surface: SurfacePick | null
  /** A pick is in flight: the picker reads Switching… and is greyed. */
  switching: boolean
  /** Asks the daemon for the displays and windows (each time the picker opens). */
  requestSurfaces: () => Promise<DesktopSurfacesMsg>
  onSelectSurface: (pick: SurfacePick) => void
}

type PickerList = { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: DesktopSurfacesMsg }

const noop = () => {}

/**
 * Phase 20-07 — the viewer's control bar (20-UI-SPEC Surface B): a top-center floating
 * toolbar. Rendered by DesktopView only for a viewer that may control the Mac.
 */
export function DesktopControlBar({
  sendChord, onMenuOpenChange, onSendKeysDone, windowShare, surface, switching, requestSurfaces, onSelectSurface,
}: DesktopControlBarProps): React.ReactElement {
  const sendKeysBtn = useRef<HTMLButtonElement>(null)
  const pickerBtn = useRef<HTMLButtonElement>(null)
  const [sendKeys, setSendKeys] = useState<{ x: number; y: number } | null>(null)
  const [picker, setPicker] = useState<{ x: number; y: number } | null>(null)
  const [pickerList, setPickerList] = useState<PickerList>({ state: 'loading' })
  const chose = useRef(false)
  // A newer open ignores the answer of an older one.
  const listSeq = useRef(0)

  function anchorBelow(btn: HTMLElement) {
    const r = btn.getBoundingClientRect()
    return { x: r.left, y: r.bottom }
  }

  function openSendKeys() {
    chose.current = false
    setSendKeys(anchorBelow(sendKeysBtn.current!))
    onMenuOpenChange(true)
  }

  function openPicker() {
    const seq = ++listSeq.current
    setPickerList({ state: 'loading' })
    setPicker(anchorBelow(pickerBtn.current!))
    onMenuOpenChange(true)
    requestSurfaces().then(
      (list) => { if (seq === listSeq.current) setPickerList({ state: 'ready', list }) },
      () => { if (seq === listSeq.current) setPickerList({ state: 'failed' }) },
    )
  }

  function pickerItems(): ContextMenuEntry[] {
    const choose = (kind: SurfaceKind, id: number, name: string) => () => onSelectSurface({ kind, id, name })
    const current = (kind: SurfaceKind, id: number) => surface?.kind === kind && surface.id === id
    const out: ContextMenuEntry[] = [{ kind: 'label', text: 'Displays' }]
    if (pickerList.state === 'failed') {
      out.push({ label: "Couldn't load the list. Close the menu and try again.", onSelect: noop, disabled: true })
    } else if (pickerList.state === 'loading') {
      out.push({ label: 'Loading…', onSelect: noop, disabled: true })
      out.push({ kind: 'label', text: 'Windows', separatorBefore: true })
    } else {
      const { displays, windows } = pickerList.list
      for (const d of displays) {
        out.push({
          label: d.name,
          hint: `${d.w} x ${d.h}`,
          title: d.name,
          checked: current('display', d.id),
          onSelect: current('display', d.id) ? noop : choose('display', d.id, d.name),
        })
      }
      out.push({ kind: 'label', text: 'Windows', separatorBefore: true })
      if (windows.length === 0) out.push({ label: 'No windows to show', onSelect: noop, disabled: true })
      for (const w of windows) {
        out.push({
          label: w.name,
          title: w.name,
          checked: current('window', w.id),
          onSelect: current('window', w.id) ? noop : choose('window', w.id, w.name),
        })
      }
    }
    out.push({ kind: 'caption', text: WINDOWS_CAPTION, id: WINDOWS_CAPTION_ID })
    return out
  }

  const label = switching ? 'Switching…' : (surface?.name ?? 'Shared screen')
  const PickerIcon = surface?.kind === 'window' ? WindowIcon : ComputerDesktopIcon

  return (
    <div className="desktop-bar" role="toolbar" aria-label="Desktop controls">
      <button
        ref={pickerBtn}
        type="button"
        className="desktop-bar__item desktop-bar__picker"
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={picker !== null}
        aria-disabled={switching ? 'true' : undefined}
        onClick={() => { if (!switching && !picker) openPicker() }}
      >
        <PickerIcon className="desktop-bar__icon" aria-hidden="true" />
        <span className="desktop-bar__name">{label}</span>
        <ChevronDownIcon className="desktop-bar__icon" aria-hidden="true" />
      </button>
      {picker && (
        <ContextMenu
          className="desktop-menu"
          items={pickerItems()}
          x={picker.x}
          y={picker.y}
          invoker={pickerBtn.current}
          onClose={() => {
            listSeq.current++
            setPicker(null)
            onMenuOpenChange(false)
          }}
        />
      )}
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
