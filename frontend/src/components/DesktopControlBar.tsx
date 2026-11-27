import React, { useEffect, useRef, useState } from 'react'
import {
  ArrowsPointingInIcon,
  ArrowsPointingOutIcon,
  ArrowsRightLeftIcon,
  CheckIcon,
  ChevronDownIcon,
  ClipboardDocumentIcon,
  ComputerDesktopIcon,
  KeyIcon,
  WindowIcon,
} from '@heroicons/react/24/outline'
import type { DesktopSurfacesMsg, SurfaceKind } from '../lib/desktopClient'
import { ContextMenu, type ContextMenuEntry } from './ContextMenu'
import { DesktopSendKeysMenu } from './DesktopSendKeysMenu'

/** The label returns to the previous surface, with a notice, when no surface message answers a pick. */
export const SWITCH_TIMEOUT_MS = 5000
/** The bar shows for this long after the first frame, and after a lock, control or surface change. */
export const REVEAL_FIRST_MS = 5000
export const REVEAL_CHANGE_MS = 3000
/** It hides this long after the pointer leaves both the bar and the reveal zone. */
export const HIDE_AFTER_MS = 1500
/** Below this viewport width every item except the picker and the Control segments is icon-only (style.css has the same number). */
export const COMPACT_WIDTH = 880

const ZONE_HEIGHT_PX = 16
export const WINDOWS_CAPTION = "Menus and pop-ups don't appear in a window share. Share the display to see them."
const WINDOWS_CAPTION_ID = 'desktop-picker-caption'
export const CLIPBOARD_CAPTION = "Cmd+C and Cmd+V in the picture use the Mac's own clipboard. Use these to move text between the two."
const CLIPBOARD_CAPTION_ID = 'desktop-clipboard-caption'
const CONTROL_TOOLTIP = 'Control: click the picture to type. To give the keyboard back, press Ctrl+Option+Cmd+Return.'

export type ControlMode = 'view' | 'control'
export type BarItem = 'picker' | 'sendKeys' | 'clipboard' | 'swap' | 'control' | 'fullscreen'

export interface BarState {
  locked: boolean
  reconnecting: boolean
  /** The daemon reported that posted input has no effect (Control needs Accessibility). */
  accessNeeded: boolean
  /** A pick is in flight. */
  switching: boolean
  mode: ControlMode
  /** The hello grants surface choice; without it the picker is not offered. */
  canSurface: boolean
  /** document.fullscreenEnabled; without it the button is not offered. */
  fullscreenAvailable: boolean
  /** The hello grants the clipboard right; without it the Clipboard button is not offered. */
  canClipboard: boolean
}

export interface ItemAvailability {
  hidden: boolean
  disabled: boolean
  reason?: string
  /** Only the Control segment is greyed; View Only stays selectable (Control needs Accessibility). */
  controlOnly?: boolean
}

/**
 * The 20-UI-SPEC availability table. Hidden: this viewer can never use it. Greyed: it cannot be
 * used now, and the reason is its tooltip and its description. Everything that sends something to
 * the Mac is greyed while locked or reconnecting; Swap and Full Screen only change this viewer.
 */
export function barAvailability(s: BarState): Record<BarItem, ItemAvailability> {
  const lost = s.locked ? 'Locked' : s.reconnecting ? 'Reconnecting' : undefined
  const grey = (reason: string | undefined): ItemAvailability => ({ hidden: false, disabled: reason !== undefined, reason })
  return {
    picker: {
      ...grey(lost ?? (s.switching ? 'Switching…' : undefined)),
      hidden: !s.canSurface,
    },
    sendKeys: grey(
      lost ?? (s.accessNeeded ? 'Control needs Accessibility' : s.mode === 'view' ? 'Turn on Control first' : undefined),
    ),
    clipboard: { ...grey(lost), hidden: !s.canClipboard },
    swap: grey(undefined),
    control: lost
      ? grey(lost)
      : s.accessNeeded
        ? { hidden: false, disabled: true, reason: 'Control needs Accessibility', controlOnly: true }
        : grey(undefined),
    fullscreen: { ...grey(undefined), hidden: !s.fullscreenAvailable },
  }
}

export interface SurfacePick {
  kind: SurfaceKind
  id: number
  name: string
}

export interface DesktopControlBarProps {
  state: BarState
  /** The shared surface as the daemon last said it; null until the first surface message. */
  surface: SurfacePick | null
  /** Asks the daemon for the displays and windows (each time the picker opens). */
  requestSurfaces: () => Promise<DesktopSurfacesMsg>
  onSelectSurface: (pick: SurfacePick) => void
  /** Taps a chord on the input channel (desktopInput.sendChord). */
  sendChord: (codes: string[]) => void
  /** Called with true while any bar menu or dialog is open: the view releases everything held. */
  onMenuOpenChange: (open: boolean) => void
  /** Called after a Send Keys item ran and its menu closed: the view returns focus to the stage if it had it. */
  onSendKeysDone: () => void
  swapOn: boolean
  onSwapChange: (on: boolean) => void
  onModeChange: (mode: ControlMode) => void
  fullscreen: boolean
  onFullscreen: () => void
  /** The Control segment: the release chord moves focus here. */
  controlRef: React.RefObject<HTMLButtonElement | null>
  /** Copy from Mac / Paste to Mac were chosen. Both run inside the click (the browser's clipboard gesture rule). */
  onCopy: () => void
  onPaste: () => void
  /** A transfer is in flight: both items are greyed so two never overlap. */
  clipBusy: boolean
  /** The Clipboard button, so the host can anchor its Copy and Paste popovers under it. */
  clipboardRef?: React.RefObject<HTMLButtonElement | null>
  /** A Copy or Paste popover is open: the bar stays shown, like for an open menu. */
  popoverOpen?: boolean
}

type PickerList = { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: DesktopSurfacesMsg }

const noop = () => {}

/**
 * When the bar shows. Hidden it is transparent and click-through but stays in the tab order;
 * it shows for the top-center zone or the bar's own box, focus inside, an open menu or dialog,
 * and a timed flash. A press on the hidden bar's box reveals it and is not forwarded to the Mac.
 */
function useReveal(barRef: React.RefObject<HTMLDivElement | null>, menuOpen: boolean, flashKey: string): boolean {
  const [hot, setHot] = useState(false)
  const [focusIn, setFocusIn] = useState(false)
  const [flash, setFlash] = useState(false)
  const hotRef = useRef(false)
  const shownRef = useRef(false)
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const flashTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const flashUntil = useRef(0)
  const firstKey = useRef(flashKey)

  function flashFor(ms: number) {
    const until = Date.now() + ms
    if (until <= flashUntil.current) return
    flashUntil.current = until
    setFlash(true)
    if (flashTimer.current) clearTimeout(flashTimer.current)
    flashTimer.current = setTimeout(() => {
      flashTimer.current = null
      setFlash(false)
    }, ms)
  }

  // 5 s after the first frame (the bar mounts when the picture is up), 3 s after a lock, control or surface change.
  useEffect(() => {
    flashFor(REVEAL_FIRST_MS)
    return () => {
      if (flashTimer.current) clearTimeout(flashTimer.current)
    }
  }, [])
  useEffect(() => {
    if (flashKey !== firstKey.current) flashFor(REVEAL_CHANGE_MS)
    firstKey.current = flashKey
  }, [flashKey])

  useEffect(() => {
    // Where the bar sits, hidden (8 px higher, translateY) or shown: wider and taller than the zone with full labels.
    const inBarBox = (x: number, y: number) => {
      const r = barRef.current?.getBoundingClientRect()
      return !!r && r.width > 0 && x >= r.left && x <= r.right && y >= 0 && y <= r.bottom + 8
    }
    const reveal = () => {
      if (hideTimer.current) clearTimeout(hideTimer.current)
      hideTimer.current = null
      hotRef.current = true
      setHot(true)
    }
    const leave = () => {
      if (!hotRef.current || hideTimer.current) return
      hideTimer.current = setTimeout(() => {
        hideTimer.current = null
        hotRef.current = false
        setHot(false)
      }, HIDE_AFTER_MS)
    }
    const onMove = (e: PointerEvent) => {
      const inZone =
        e.clientY < ZONE_HEIGHT_PX && e.clientX >= window.innerWidth / 4 && e.clientX <= (window.innerWidth * 3) / 4
      const overBar = e.target instanceof Node && barRef.current?.contains(e.target) === true
      if (inZone || overBar || inBarBox(e.clientX, e.clientY)) reveal()
      else leave()
    }
    // The hidden bar is click-through: a press where it sits would reach the stage and the Mac. It reveals the bar instead.
    const onDown = (e: PointerEvent) => {
      if (shownRef.current || !inBarBox(e.clientX, e.clientY)) return
      e.preventDefault()
      e.stopPropagation()
      reveal()
    }
    document.addEventListener('pointermove', onMove, { passive: true })
    document.addEventListener('pointerdown', onDown, true)
    document.documentElement.addEventListener('pointerleave', leave)
    return () => {
      document.removeEventListener('pointermove', onMove)
      document.removeEventListener('pointerdown', onDown, true)
      document.documentElement.removeEventListener('pointerleave', leave)
      if (hideTimer.current) clearTimeout(hideTimer.current)
    }
  }, [barRef])

  // Focus is tracked from the bar's own events so a focus inside always shows it.
  useEffect(() => {
    const el = barRef.current
    if (!el) return
    const onIn = () => setFocusIn(true)
    const onOut = (e: FocusEvent) => {
      if (!(e.relatedTarget instanceof Node) || !el.contains(e.relatedTarget)) setFocusIn(false)
    }
    el.addEventListener('focusin', onIn)
    el.addEventListener('focusout', onOut)
    return () => {
      el.removeEventListener('focusin', onIn)
      el.removeEventListener('focusout', onOut)
    }
  }, [barRef])

  const shown = hot || focusIn || menuOpen || flash
  useEffect(() => {
    shownRef.current = shown
  }, [shown])
  return shown
}

interface BarButtonProps {
  label: string
  avail: ItemAvailability
  /** Icon and visible text; the text is hidden below COMPACT_WIDTH, the label stays. */
  children: React.ReactNode
  onActivate: () => void
  ref?: React.Ref<HTMLButtonElement>
  className?: string
  extra?: React.ButtonHTMLAttributes<HTMLButtonElement>
}

/** A bar button: greyed means aria-disabled (still focusable, so its reason is read), never disabled. */
function BarButton({ label, avail, children, onActivate, ref, className, extra }: BarButtonProps) {
  return (
    <button
      ref={ref}
      type="button"
      className={className ? `desktop-bar__item ${className}` : 'desktop-bar__item'}
      aria-label={label}
      title={avail.reason ?? label}
      aria-disabled={avail.disabled ? 'true' : undefined}
      aria-describedby={avail.reason ? `desktop-bar-reason-${label.replace(/\W+/g, '-')}` : undefined}
      onClick={() => {
        if (!avail.disabled) onActivate()
      }}
      {...extra}
    >
      {children}
    </button>
  )
}

/**
 * Phase 20-07 — the viewer's control bar (20-UI-SPEC Surface B): a top-center floating
 * toolbar. Rendered by DesktopView only for a viewer that may control the Mac, while a
 * picture is up.
 */
export function DesktopControlBar({
  state, surface, requestSurfaces, onSelectSurface, sendChord, onMenuOpenChange, onSendKeysDone,
  swapOn, onSwapChange, onModeChange, fullscreen, onFullscreen, controlRef, onCopy, onPaste, clipBusy, clipboardRef, popoverOpen,
}: DesktopControlBarProps): React.ReactElement {
  const barRef = useRef<HTMLDivElement>(null)
  const sendKeysBtn = useRef<HTMLButtonElement>(null)
  const pickerBtn = useRef<HTMLButtonElement>(null)
  const ownClipBtn = useRef<HTMLButtonElement>(null)
  const clipBtn = clipboardRef ?? ownClipBtn
  const viewOnlyBtn = useRef<HTMLButtonElement>(null)
  const [sendKeys, setSendKeys] = useState<{ x: number; y: number } | null>(null)
  const [picker, setPicker] = useState<{ x: number; y: number } | null>(null)
  const [clipMenu, setClipMenu] = useState<{ x: number; y: number } | null>(null)
  const [pickerList, setPickerList] = useState<PickerList>({ state: 'loading' })
  const chose = useRef(false)
  // A newer open ignores the answer of an older one.
  const listSeq = useRef(0)
  // A press on a trigger closes its open menu (the menu's outside-press close); the click must not reopen it.
  const pressedWhileOpen = useRef(false)

  const avail = barAvailability(state)
  const windowShare = surface?.kind === 'window'
  const menuOpen = sendKeys !== null || picker !== null || clipMenu !== null || popoverOpen === true
  const revealed = useReveal(barRef, menuOpen, `${state.locked}|${state.accessNeeded}|${surface?.kind}:${surface?.id}`)

  function anchorBelow(btn: HTMLElement) {
    const r = btn.getBoundingClientRect()
    return { x: r.left, y: r.bottom }
  }

  function openSendKeys() {
    chose.current = false
    setSendKeys(anchorBelow(sendKeysBtn.current!))
    onMenuOpenChange(true)
  }

  function openClipMenu() {
    setClipMenu(anchorBelow(clipBtn.current!))
    onMenuOpenChange(true)
  }

  function clipItems(): ContextMenuEntry[] {
    return [
      { label: 'Copy from Mac', onSelect: onCopy, disabled: clipBusy },
      { label: 'Paste to Mac', onSelect: onPaste, disabled: clipBusy },
      { kind: 'caption', text: CLIPBOARD_CAPTION, id: CLIPBOARD_CAPTION_ID, separatorBefore: true },
    ]
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

  const pickerLabel = state.switching ? 'Switching…' : (surface?.name ?? 'Shared screen')
  const PickerIcon = surface?.kind === 'window' ? WindowIcon : ComputerDesktopIcon
  const trigger = (open: boolean, openIt: () => void) => ({
    onMouseDown: () => { pressedWhileOpen.current = open },
    onActivate: () => {
      if (pressedWhileOpen.current) {
        pressedWhileOpen.current = false
        return
      }
      if (!open) openIt()
    },
  })
  const pickerTrigger = trigger(picker !== null, openPicker)
  const sendKeysTrigger = trigger(sendKeys !== null, openSendKeys)
  const clipTrigger = trigger(clipMenu !== null, openClipMenu)

  function onSegmentKeyDown(e: React.KeyboardEvent) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const to: ControlMode = state.mode === 'view' ? 'control' : 'view'
    if (avail.control.disabled && (!avail.control.controlOnly || to === 'control')) return
    ;(to === 'control' ? controlRef : viewOnlyBtn).current?.focus()
    onModeChange(to)
  }

  const controlGreyed = avail.control.disabled
  const viewOnlyGreyed = avail.control.disabled && !avail.control.controlOnly
  const segment = (mode: ControlMode, greyed: boolean) => ({
    role: 'radio' as const,
    'aria-checked': state.mode === mode,
    'aria-disabled': greyed ? ('true' as const) : undefined,
    'aria-describedby': greyed ? 'desktop-bar-reason-control' : undefined,
    tabIndex: state.mode === mode ? 0 : -1,
    className: state.mode === mode ? 'share-seg is-active' : 'share-seg',
    onClick: () => { if (!greyed) onModeChange(mode) },
  })

  const groups: React.ReactNode[] = []
  if (!avail.picker.hidden) {
    groups.push(
      <BarButton
        key="picker"
        ref={pickerBtn}
        label={pickerLabel}
        avail={avail.picker}
        className="desktop-bar__picker"
        onActivate={pickerTrigger.onActivate}
        extra={{ 'aria-haspopup': 'menu', 'aria-expanded': picker !== null, onMouseDown: pickerTrigger.onMouseDown }}
      >
        <PickerIcon className="desktop-bar__icon" aria-hidden="true" />
        <span className="desktop-bar__name">{pickerLabel}</span>
        <ChevronDownIcon className="desktop-bar__icon" aria-hidden="true" />
      </BarButton>,
    )
  }
  // Send Keys and Clipboard are one group: what the viewer sends to the Mac.
  const sendGroup: React.ReactNode[] = [
    <BarButton
      key="sendKeys"
      ref={sendKeysBtn}
      label="Send Keys"
      avail={avail.sendKeys}
      onActivate={sendKeysTrigger.onActivate}
      extra={{ 'aria-haspopup': 'menu', 'aria-expanded': sendKeys !== null, onMouseDown: sendKeysTrigger.onMouseDown }}
    >
      <KeyIcon className="desktop-bar__icon" aria-hidden="true" />
      <span className="desktop-bar__label">Send Keys</span>
      <ChevronDownIcon className="desktop-bar__icon" aria-hidden="true" />
    </BarButton>,
  ]
  if (!avail.clipboard.hidden) {
    sendGroup.push(
      <BarButton
        key="clipboard"
        ref={clipBtn}
        label="Clipboard"
        avail={avail.clipboard}
        onActivate={clipTrigger.onActivate}
        extra={{ 'aria-haspopup': 'menu', 'aria-expanded': clipMenu !== null, onMouseDown: clipTrigger.onMouseDown }}
      >
        <ClipboardDocumentIcon className="desktop-bar__icon" aria-hidden="true" />
        <span className="desktop-bar__label">Clipboard</span>
        <ChevronDownIcon className="desktop-bar__icon" aria-hidden="true" />
      </BarButton>,
    )
  }
  groups.push(<React.Fragment key="send">{sendGroup}</React.Fragment>)
  groups.push(
    <React.Fragment key="mapping">
      <BarButton
        label="Swap Ctrl and Cmd"
        avail={avail.swap}
        className={swapOn ? 'desktop-bar__toggle is-on' : 'desktop-bar__toggle'}
        onActivate={() => onSwapChange(!swapOn)}
        extra={{ 'aria-pressed': swapOn }}
      >
        {swapOn ? (
          <CheckIcon className="desktop-bar__icon" aria-hidden="true" />
        ) : (
          <ArrowsRightLeftIcon className="desktop-bar__icon" aria-hidden="true" />
        )}
        <span className="desktop-bar__label">Swap Ctrl and Cmd</span>
      </BarButton>
      <div className="share-segbar desktop-bar__segbar" role="radiogroup" aria-label="Control mode" onKeyDown={onSegmentKeyDown}>
        <button
          ref={viewOnlyBtn}
          type="button"
          title={viewOnlyGreyed ? avail.control.reason : undefined}
          {...segment('view', viewOnlyGreyed)}
        >
          View Only
        </button>
        <button
          ref={controlRef}
          type="button"
          title={controlGreyed ? avail.control.reason : CONTROL_TOOLTIP}
          {...segment('control', controlGreyed)}
        >
          Control
        </button>
      </div>
    </React.Fragment>,
  )
  if (!avail.fullscreen.hidden) {
    groups.push(
      <BarButton
        key="fullscreen"
        label={fullscreen ? 'Exit Full Screen' : 'Full Screen'}
        avail={avail.fullscreen}
        onActivate={onFullscreen}
      >
        {fullscreen ? (
          <ArrowsPointingInIcon className="desktop-bar__icon" aria-hidden="true" />
        ) : (
          <ArrowsPointingOutIcon className="desktop-bar__icon" aria-hidden="true" />
        )}
        <span className="desktop-bar__label">{fullscreen ? 'Exit Full Screen' : 'Full Screen'}</span>
      </BarButton>,
    )
  }

  // The reasons for greyed items as real text, linked by aria-describedby; a tooltip alone is not enough.
  const reasons: [string, string][] = []
  if (avail.picker.reason && !avail.picker.hidden) reasons.push([pickerLabel, avail.picker.reason])
  if (avail.sendKeys.reason) reasons.push(['Send Keys', avail.sendKeys.reason])
  if (avail.clipboard.reason && !avail.clipboard.hidden) reasons.push(['Clipboard', avail.clipboard.reason])
  if (avail.control.reason) reasons.push(['control', avail.control.reason])

  return (
    <div
      ref={barRef}
      className={revealed ? 'desktop-bar' : 'desktop-bar desktop-bar--hidden'}
      role="toolbar"
      aria-label="Desktop controls"
    >
      {groups.map((g, n) => (
        <React.Fragment key={n}>
          {n > 0 && <div className="desktop-bar__divider" role="separator" aria-orientation="vertical" />}
          {g}
        </React.Fragment>
      ))}
      {reasons.map(([name, why]) => (
        <span key={name} id={`desktop-bar-reason-${name.replace(/\W+/g, '-')}`} className="visually-hidden">
          {why}
        </span>
      ))}
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
      {clipMenu && (
        <ContextMenu
          className="desktop-menu"
          items={clipItems()}
          x={clipMenu.x}
          y={clipMenu.y}
          invoker={clipBtn.current}
          onClose={() => {
            setClipMenu(null)
            onMenuOpenChange(false)
          }}
        />
      )}
      {sendKeys && (
        <DesktopSendKeysMenu
          anchor={sendKeys}
          invoker={sendKeysBtn.current}
          windowShare={windowShare}
          disabledReason={avail.sendKeys.reason}
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
