import React, { useEffect, useId, useRef, useState } from 'react'
import { SpeakerWaveIcon, SpeakerXMarkIcon } from '@heroicons/react/24/outline'

/** The one-time hint stays this long (23-UI-SPEC Surface 1). */
export const SOUND_HINT_MS = 5000

// Copy is 23-UI-SPEC's Copywriting Contract, word for word.
export const SOUND_HINT = 'Sound is off. Click Unmute in the bar to hear this Mac.'
export const SOUND_HINT_PHONE = 'Sound is off. Tap Unmute to hear this Mac.'
export const SOUND_UNAVAILABLE = "Sound isn't available from this Mac right now."
export const SOUND_UNAVAILABLE_NOTICE = `${SOUND_UNAVAILABLE} The picture is not affected.`
export const SOUND_PLAY_REJECTED = "Couldn't turn sound on. Choose Unmute to try again."
export const SOUND_PLAY_REJECTED_PHONE = "Couldn't turn sound on. Tap Unmute to try again."

export type SoundNotice = 'unavailable' | 'play-rejected'

export interface DesktopSoundButtonProps {
  /** The one <video> that plays the Mac's picture and sound. Its `muted` is the source of truth. */
  video: HTMLVideoElement | null
  /** The daemon reported that audio failed: the item is greyed and says why. */
  failed: boolean
  /** The 44px icon-only form of the phone's bottom bar. */
  phone: boolean
  /** Icon only (the accessible name and tooltip stay). */
  compact: boolean
  onNotice(kind: SoundNotice): void
  /** The sound is on or off; also called once at mount with the current state. */
  onChange(on: boolean): void
}

/**
 * The Sound item (DCTL-07): Unmute while muted, Mute while on. It fires on click only, because a
 * completed tap is the user activation that lets a browser play audio. The choice lives in the video
 * element and in memory, so every page load starts muted.
 */
export function DesktopSoundButton({ video, failed, phone, compact, onNotice, onChange }: DesktopSoundButtonProps): React.ReactElement {
  const [on, setOn] = useState(() => video !== null && !video.muted)
  const [announce, setAnnounce] = useState('')
  const pending = useRef(false)
  const reasonId = useId()
  const notice = useRef(onNotice)
  notice.current = onNotice
  const changed = useRef(onChange)
  changed.current = onChange

  useEffect(() => {
    changed.current(on)
  }, [on])

  // Something else muted the element (a reconnect that could not play with sound): the item says so.
  useEffect(() => {
    if (!video) return
    const sync = () => {
      if (video.muted) setOn(false)
    }
    video.addEventListener('volumechange', sync)
    return () => video.removeEventListener('volumechange', sync)
  }, [video])

  async function toggle() {
    if (failed) return notice.current('unavailable')
    if (!video || pending.current) return
    if (on) {
      video.muted = true
      setOn(false)
      setAnnounce('Sound off')
      return
    }
    // muted is cleared and play() runs inside this click: that is the activation the browser needs.
    pending.current = true
    video.muted = false
    try {
      await video.play()
      setOn(true)
      setAnnounce('Sound on')
    } catch {
      video.muted = true
      notice.current('play-rejected')
    } finally {
      pending.current = false
    }
  }

  const label = on ? 'Mute' : 'Unmute'
  const Icon = on ? SpeakerWaveIcon : SpeakerXMarkIcon
  const className = ['desktop-bar__item', 'desktop-sound', phone && 'desktop-sound--phone'].filter(Boolean).join(' ')
  return (
    <>
      <button
        type="button"
        className={className}
        aria-label={label}
        title={failed ? SOUND_UNAVAILABLE : on ? 'Mute' : "Unmute: hear this Mac's sound"}
        aria-disabled={failed ? 'true' : undefined}
        aria-describedby={failed ? reasonId : undefined}
        onClick={() => void toggle()}
      >
        <Icon className="desktop-bar__icon" aria-hidden="true" />
        {!compact && (
          <span className="desktop-bar__label desktop-sound__label">
            <span>{label}</span>
            {/* Always holds the wider word, so toggling never changes the item's width. */}
            <span className="desktop-sound__ghost" aria-hidden="true">Unmute</span>
          </span>
        )}
      </button>
      {failed && (
        <span id={reasonId} className="visually-hidden">
          {SOUND_UNAVAILABLE}
        </span>
      )}
      <span role="status" className="visually-hidden">
        {announce}
      </span>
    </>
  )
}
