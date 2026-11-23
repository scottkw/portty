import React from 'react'
import { ArrowDownIcon, ArrowLeftIcon, ArrowRightIcon, ArrowUpIcon } from '@heroicons/react/24/outline'
import { arrowSeq, ESC, PIPE, TAB, type ArrowDir } from '../lib/ctrlMap'

interface KeyBarProps {
  /** Send bytes to the shell as typing does (TerminalPanel's term.input path). */
  send: (seq: string) => void
  /** True while the terminal is in application cursor mode (arrows send ESC O x). */
  applicationCursor: () => boolean
  ctrlArmed: boolean
  onCtrlChange: (armed: boolean) => void
  /** 14-11: the on-screen keyboard covers the home indicator, so the bar drops its bottom safe-area inset. */
  keyboardUp?: boolean
}

/**
 * Portty 14-10 (MOB-01): the keys iOS's keyboard lacks, one row under the terminal on a phone.
 * Firing rule: a key fires on pointerdown (preventDefault keeps the terminal focused and the keyboard
 * up) and on a click with detail 0 (VoiceOver, a hardware keyboard); a touch tap's own click has
 * detail >= 1 and is ignored, so one tap sends once. Ctrl is one-shot: it arms, the next key gets it.
 */
export function KeyBar({ send, applicationCursor, ctrlArmed, onCtrlChange, keyboardUp }: KeyBarProps): React.ReactElement {
  // The key being pressed: iOS may not apply :active when pointerdown is prevented, so the class is explicit.
  const [down, setDown] = React.useState<string | null>(null)
  const release = () => setDown(null)
  // Disarm first: the transform on the input path then sees an unarmed Ctrl for these bytes.
  const fire = (seq: () => string) => {
    const bytes = seq()
    onCtrlChange(false)
    send(bytes)
  }
  const arrow = (dir: ArrowDir) => () => arrowSeq(dir, applicationCursor(), ctrlArmed)

  const key = (name: string, label: React.ReactNode, onFire: () => void, extra?: { armed?: boolean; className?: string }) => (
    <button
      type="button"
      className={`keybar__key${extra?.className ? ` ${extra.className}` : ''}${extra?.armed ? ' keybar__key--armed' : ''}${down === name ? ' keybar__key--down' : ''}`}
      aria-label={name === label ? undefined : name}
      aria-pressed={extra?.armed === undefined ? undefined : extra.armed}
      onPointerDown={(e) => { e.preventDefault(); setDown(name); onFire() }}
      onPointerUp={release}
      onPointerCancel={release}
      onPointerLeave={release}
      onClick={(e) => { if (e.detail === 0) onFire() }}
    >
      {label}
    </button>
  )
  const icon = (I: typeof ArrowUpIcon) => <I width={20} height={20} aria-hidden="true" />

  return (
    <div className={`keybar${keyboardUp ? ' keybar--kb-up' : ''}`} role="toolbar" aria-label="Terminal keys" aria-orientation="horizontal">
      {key('Esc', 'Esc', () => fire(() => ESC))}
      {key('Tab', 'Tab', () => fire(() => TAB))}
      {key('Ctrl', 'Ctrl', () => onCtrlChange(!ctrlArmed), { armed: ctrlArmed })}
      {key('Left arrow', icon(ArrowLeftIcon), () => fire(arrow('left')))}
      {key('Up arrow', icon(ArrowUpIcon), () => fire(arrow('up')))}
      {key('Down arrow', icon(ArrowDownIcon), () => fire(arrow('down')))}
      {key('Right arrow', icon(ArrowRightIcon), () => fire(arrow('right')))}
      {key('Vertical bar', PIPE, () => fire(() => PIPE), { className: 'keybar__key--pipe' })}
    </div>
  )
}
