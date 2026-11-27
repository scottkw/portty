import React, { useRef, useState } from 'react'
import {
  ArrowDownIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  ArrowUpIcon,
  ChevronUpIcon,
  KeyIcon,
} from '@heroicons/react/24/outline'
import type { DesktopClient, DesktopSurfacesMsg, SurfaceKind } from '../lib/desktopClient'
import { ContextMenu, type ContextMenuEntry } from './ContextMenu'
import { WINDOWS_CAPTION, type SurfacePick } from './DesktopControlBar'
import { DesktopSendKeysMenu } from './DesktopSendKeysMenu'

export type PhoneModifier = 'Control' | 'Alt' | 'Meta'

export interface DesktopPhoneBarsProps {
  keyboardUp: boolean
  /** The picture's zoom, 1 to 5. */
  zoom: number
  onFit(): void
  /** Runs inside the button's click, where iOS allows the keyboard to open. */
  onShowKeyboard(): void
  /** A key of the key bar: a KeyboardEvent.code. */
  onKey(code: string): void
  armed: Set<PhoneModifier>
  onToggleArm(m: PhoneModifier): void
  sendChord(codes: string[]): void
  surfaceName: string | null
  /** The shared surface, so the picker can check it. */
  surface?: { kind: SurfaceKind; id: number } | null
  canControl: boolean
  canPickSurface: boolean
  disabledReason: 'Locked' | 'Reconnecting' | null
  client: DesktopClient
  /** The view's pick handler: it tracks the switch and reports a refusal. */
  onSelectSurface(pick: SurfacePick): void
}

const CAPTION_ID = 'desktop-phone-picker-caption'
type PickerList = { state: 'loading' } | { state: 'failed' } | { state: 'ready'; list: DesktopSurfacesMsg }
const noop = () => {}

/**
 * Phase 21-10 (DVWR-03, 21-UI-SPEC Surface 4) — the one 44 px slot at the bottom of the phone viewer:
 * the bottom bar while the keyboard is down, the key bar while it is up. Firing rule (Phase 14): a key
 * fires on pointerdown with preventDefault (so the hidden input keeps focus) and on a click with
 * detail 0 (VoiceOver); a touch tap's own click has detail 1 and is ignored. Show Keyboard is the one
 * exception, it acts in its click, because iOS only opens the keyboard from there.
 */
export function DesktopPhoneBars({
  keyboardUp, zoom, onFit, onShowKeyboard, onKey, armed, onToggleArm, sendChord,
  surfaceName, surface, canControl, canPickSurface, disabledReason, client, onSelectSurface,
}: DesktopPhoneBarsProps): React.ReactElement {
  const [down, setDown] = useState<string | null>(null)
  const [sendKeys, setSendKeys] = useState<{ x: number; y: number } | null>(null)
  const [picker, setPicker] = useState<{ x: number; y: number } | null>(null)
  const [pickerList, setPickerList] = useState<PickerList>({ state: 'loading' })
  const listSeq = useRef(0)
  const pickerBtn = useRef<HTMLButtonElement>(null)
  const sendKeysBtn = useRef<HTMLButtonElement>(null)
  const greyed = disabledReason !== null

  const anchorAbove = (btn: HTMLElement | null) => {
    const r = btn?.getBoundingClientRect()
    return { x: r?.left ?? 0, y: r?.top ?? 0 }
  }
  const openSendKeys = () => setSendKeys(anchorAbove(sendKeysBtn.current))
  const openPicker = () => {
    const seq = ++listSeq.current
    setPickerList({ state: 'loading' })
    setPicker(anchorAbove(pickerBtn.current))
    client.requestSurfaces().then(
      (list) => { if (seq === listSeq.current) setPickerList({ state: 'ready', list }) },
      () => { if (seq === listSeq.current) setPickerList({ state: 'failed' }) },
    )
  }

  function pickerItems(): ContextMenuEntry[] {
    const pick = (kind: SurfaceKind, id: number, name: string) => () => onSelectSurface({ kind, id, name })
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
          onSelect: current('display', d.id) ? noop : pick('display', d.id, d.name),
        })
      }
      out.push({ kind: 'label', text: 'Windows', separatorBefore: true })
      if (windows.length === 0) out.push({ label: 'No windows to show', onSelect: noop, disabled: true })
      for (const w of windows) {
        out.push({
          label: w.name,
          title: w.name,
          checked: current('window', w.id),
          onSelect: current('window', w.id) ? noop : pick('window', w.id, w.name),
        })
      }
    }
    out.push({ kind: 'caption', text: WINDOWS_CAPTION, id: CAPTION_ID })
    return out
  }

  // The Phase 14 firing rule. A greyed key is aria-disabled and does nothing.
  const key = (
    name: string,
    label: React.ReactNode,
    onFire: () => void,
    extra: { armed?: boolean; ref?: React.Ref<HTMLButtonElement>; className?: string; menu?: boolean; disabled?: boolean; plainLabel?: boolean } = {},
  ) => {
    const off = extra.disabled ?? greyed
    return (
      <button
        key={name}
        ref={extra.ref}
        type="button"
        className={`keybar__key desktop-phone__key${extra.className ? ` ${extra.className}` : ''}${extra.armed ? ' keybar__key--armed' : ''}${down === name ? ' keybar__key--down' : ''}`}
        aria-label={extra.plainLabel ? undefined : name}
        aria-pressed={extra.armed === undefined ? undefined : extra.armed}
        aria-haspopup={extra.menu ? 'menu' : undefined}
        aria-disabled={off ? true : undefined}
        onPointerDown={(e) => { e.preventDefault(); setDown(name); if (!off) onFire() }}
        onPointerUp={() => setDown(null)}
        onPointerCancel={() => setDown(null)}
        onPointerLeave={() => setDown(null)}
        onClick={(e) => { if (e.detail === 0 && !off) onFire() }}
      >
        {label}
      </button>
    )
  }
  const icon = (I: typeof ArrowUpIcon) => <I width={20} height={20} aria-hidden="true" />
  const modifier = (name: string, label: string, m: PhoneModifier) =>
    key(name, label, () => onToggleArm(m), { armed: armed.has(m), plainLabel: true })

  const pickerLabel = surfaceName ?? 'Shared screen'
  const zoomText = `${Number(zoom.toFixed(1))}x`

  const bottomBar = (
    <div className="desktop-phone__bar" role="toolbar" aria-label="Desktop controls" aria-orientation="horizontal">
      {canPickSurface &&
        key(
          pickerLabel,
          <>
            <span className="desktop-phone__name">{pickerLabel}</span>
            <ChevronUpIcon width={16} height={16} aria-hidden="true" />
          </>,
          openPicker,
          { ref: pickerBtn, menu: true },
        )}
      {canControl &&
        key(
          'Send Keys',
          <>
            <span>Send Keys</span>
            <ChevronUpIcon width={16} height={16} aria-hidden="true" />
          </>,
          openSendKeys,
          { ref: sendKeysBtn, menu: true, plainLabel: true },
        )}
      {canControl && (
        <button
          type="button"
          className="keybar__key desktop-phone__key"
          onPointerDown={(e) => e.preventDefault()}
          onClick={onShowKeyboard}
        >
          Show Keyboard
        </button>
      )}
      {zoom > 1 &&
        key(`Fit to screen, zoom ${zoomText}`, 'Fit to Screen', onFit, { disabled: false })}
    </div>
  )

  const keyBar = (
    <div className="desktop-phone__bar" role="toolbar" aria-label="Desktop keys" aria-orientation="horizontal">
      {key('Esc', 'Esc', () => onKey('Escape'), { plainLabel: true })}
      {key('Tab', 'Tab', () => onKey('Tab'), { plainLabel: true })}
      {modifier('Ctrl', 'Ctrl', 'Control')}
      {modifier('Opt', 'Opt', 'Alt')}
      {modifier('Cmd', 'Cmd', 'Meta')}
      {key('Left arrow', icon(ArrowLeftIcon), () => onKey('ArrowLeft'))}
      {key('Up arrow', icon(ArrowUpIcon), () => onKey('ArrowUp'))}
      {key('Down arrow', icon(ArrowDownIcon), () => onKey('ArrowDown'))}
      {key('Right arrow', icon(ArrowRightIcon), () => onKey('ArrowRight'))}
      {key('Send Keys', icon(KeyIcon), openSendKeys, { ref: sendKeysBtn, menu: true })}
    </div>
  )

  return (
    <div className={`keybar desktop-phone__slot${keyboardUp ? ' keybar--kb-up' : ''}`}>
      {keyboardUp ? keyBar : bottomBar}
      {sendKeys && (
        <DesktopSendKeysMenu
          anchor={sendKeys}
          invoker={sendKeysBtn.current}
          onClose={() => setSendKeys(null)}
          sendChord={sendChord}
          windowShare={surface?.kind === 'window'}
          disabledReason={disabledReason ?? undefined}
          above
        />
      )}
      {picker && (
        <ContextMenu
          className="desktop-menu"
          items={pickerItems()}
          x={picker.x}
          y={picker.y}
          above
          invoker={pickerBtn.current}
          onClose={() => setPicker(null)}
        />
      )}
    </div>
  )
}
