import React, { useCallback, useEffect, useRef, useState } from 'react'
import {
  ArrowPathIcon,
  ClockIcon,
  ComputerDesktopIcon,
  ExclamationTriangleIcon,
  LockClosedIcon,
  MoonIcon,
  PlayIcon,
  SignalSlashIcon,
} from '@heroicons/react/24/outline'
import { createDesktopClient, type ClientStatus, type DesktopStateMsg } from '../lib/desktopClient'
import { createDesktopInput, type DesktopInput } from '../lib/desktopInput'
import { isPhone } from '../lib/phoneQuery'
import { UserRefused } from './UserRefused'

const TITLE = 'Portty Desktop'

export interface VideoRect {
  x: number
  y: number
  width: number
  height: number
}

/**
 * Where the picture sits inside the video element under object-fit: contain,
 * in CSS pixels. Phase 20 normalises pointer coordinates against this rect so
 * a click on the black letterbox is not a click on the desktop. Zero when
 * either size is unknown.
 */
export function videoContentRect(elementWidth: number, elementHeight: number, videoWidth: number, videoHeight: number): VideoRect {
  if (elementWidth <= 0 || elementHeight <= 0 || videoWidth <= 0 || videoHeight <= 0) {
    return { x: 0, y: 0, width: 0, height: 0 }
  }
  const scale = Math.min(elementWidth / videoWidth, elementHeight / videoHeight)
  const width = videoWidth * scale
  const height = videoHeight * scale
  return { x: (elementWidth - width) / 2, y: (elementHeight - height) / 2, width, height }
}

type Icon = React.ComponentType<React.SVGProps<SVGSVGElement>>
type CardKey =
  | 'connecting' | 'needs_permission' | 'awaiting_approval' | 'display_asleep' | 'capture_paused'
  | 'tap' | 'unreachable' | 'token_rejected' | 'desktop_off'

// Copy is 19-UI-SPEC's Copywriting Contract, word for word. Nothing here is
// rendered from the wire: the server picks a key by enum, never supplies text.
const CARDS: Record<CardKey, { Icon: Icon; heading: string; body: string }> = {
  connecting: {
    Icon: ComputerDesktopIcon,
    heading: 'Connecting…',
    body: 'Waiting for the first picture from your Mac. If your browser asks to find devices nearby, choose Allow.',
  },
  needs_permission: {
    Icon: ExclamationTriangleIcon,
    heading: 'Needs permission',
    body: "Portty doesn't have permission to show this Mac's screen. On the Mac, open Portty, go to Settings, then Sharing, and choose Set Up Permissions.",
  },
  awaiting_approval: {
    Icon: ClockIcon,
    heading: 'Waiting for approval at the Mac',
    body: "macOS is asking someone at the Mac to approve Portty's screen access. The picture comes back once it is approved.",
  },
  display_asleep: {
    Icon: MoonIcon,
    heading: 'Display asleep',
    body: "The Mac's display is off. The picture comes back when it wakes.",
  },
  capture_paused: {
    Icon: ExclamationTriangleIcon,
    heading: 'Desktop paused',
    body: "Portty stopped showing this Mac's screen after repeated errors. On the Mac, open Portty, go to Settings, then Sharing, and run Test Capture.",
  },
  tap: {
    Icon: PlayIcon,
    heading: 'Tap to start',
    body: 'Your browser needs a tap to start the picture.',
  },
  unreachable: {
    Icon: SignalSlashIcon,
    heading: "Can't reach this Mac",
    body: "Check that it's awake and on your tailnet. This page keeps trying.",
  },
  token_rejected: {
    Icon: ExclamationTriangleIcon,
    heading: 'This desktop link no longer works',
    body: 'The link is incomplete or has been replaced. On the Mac, open Portty, go to Settings, then Sharing, and use Copy Link under Desktop.',
  },
  desktop_off: {
    Icon: ComputerDesktopIcon,
    heading: 'Desktop sharing is off on this Mac',
    body: "Turn on Always-available desktop in Portty's Settings, under Sharing, then copy the link from there.",
  },
}

const STATE_CARDS: Partial<Record<DesktopStateMsg['state'], CardKey>> = {
  needs_permission: 'needs_permission',
  awaiting_approval: 'awaiting_approval',
  display_asleep: 'display_asleep',
  capture_paused: 'capture_paused',
}

const CHIP_SHOW_MS = 3000
const CHIP_FADE_MS = 150

export interface DesktopViewProps {
  /** Phase 20 normalises pointer coordinates against this rect. */
  onVideoRect?: (r: VideoRect) => void
  /** Reload Page; a Home Screen web app has no browser reload. Defaults to location.reload. */
  onReload?: () => void
}

/**
 * Phase 19-06 — the /desktop/ viewer root. Owns one DesktopClient and one
 * <video> for the life of the page; no Wails imports. Phase 20-06 wraps the
 * video in the stage, which owns every pointer and key listener of a viewer
 * that may control the Mac; a phone, and a viewer without the control right,
 * stays the view-only page.
 */
export function DesktopView({ onVideoRect, onReload }: DesktopViewProps = {}): React.ReactElement {
  const videoRef = useRef<HTMLVideoElement>(null)
  const stageRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const inputCtl = useRef<DesktopInput | null>(null)
  const rectCb = useRef(onVideoRect)
  rectCb.current = onVideoRect

  const [status, setStatus] = useState<ClientStatus>({ kind: 'connecting' })
  const [desk, setDesk] = useState<DesktopStateMsg | null>(null)
  const [viewers, setViewers] = useState<number | null>(null)
  const [hasFrame, setHasFrame] = useState(false)
  const [needsTap, setNeedsTap] = useState(false)
  const [chipPhase, setChipPhase] = useState<'gone' | 'shown' | 'fading'>('gone')

  useEffect(() => {
    document.title = TITLE
    let meta = document.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-title"]')
    if (!meta) {
      meta = document.createElement('meta')
      meta.name = 'apple-mobile-web-app-title'
      document.head.appendChild(meta)
    }
    meta.content = TITLE
    // React sets the muted property but not the attribute; Safari's autoplay policy reads the attribute.
    videoRef.current?.setAttribute('muted', '')
  }, [])

  const updateRect = useCallback(() => {
    const v = videoRef.current
    if (v && rectCb.current) rectCb.current(videoContentRect(v.clientWidth, v.clientHeight, v.videoWidth, v.videoHeight))
  }, [])

  useEffect(() => {
    const v = videoRef.current
    v?.addEventListener('resize', updateRect)
    window.addEventListener('resize', updateRect)
    return () => {
      v?.removeEventListener('resize', updateRect)
      window.removeEventListener('resize', updateRect)
    }
  }, [updateRect])

  // A rejected play() (autoplay policy, always on iPhone) is the Tap to start card, not an error.
  const startPlay = useCallback(() => {
    const v = videoRef.current
    if (!v) return
    try {
      void Promise.resolve(v.play()).then(() => setNeedsTap(false), () => setNeedsTap(true))
    } catch {
      setNeedsTap(true)
    }
  }, [])

  useEffect(() => {
    const client = createDesktopClient()
    const phone = isPhone()
    const offs = [
      client.on('rights', (r) => {
        const stage = stageRef.current
        const input = inputRef.current
        if (!r.control || phone || inputCtl.current || !stage || !input) return
        const ctl = createDesktopInput(stage, input, {
          send: (msg, droppable = false) => client.sendInput(msg, droppable),
          getEpoch: () => client.currentEpoch(),
          getRect: () => {
            const v = videoRef.current
            return v ? videoContentRect(v.clientWidth, v.clientHeight, v.videoWidth, v.videoHeight) : { x: 0, y: 0, width: 0, height: 0 }
          },
        })
        ctl.setControl(true)
        inputCtl.current = ctl
      }),
      client.on('track', (stream) => {
        const v = videoRef.current
        if (!v) return
        v.srcObject = stream
        startPlay()
      }),
      client.on('state', (s) => {
        setDesk(s)
        setViewers(s.viewers)
      }),
      client.on('size', updateRect),
      client.on('status', (s) => {
        setStatus(s)
        // A state card describes the connection that is gone; the next state message replaces it.
        if (s.kind !== 'streaming' && s.kind !== 'connecting') setDesk(null)
      }),
    ]
    client.start()
    return () => {
      for (const off of offs) off()
      inputCtl.current?.destroy()
      inputCtl.current = null
      client.stop()
    }
  }, [startPlay, updateRect])

  // The count chip: 3 s after the first frame, and again whenever the count changes.
  useEffect(() => {
    if (!hasFrame || viewers === null || viewers < 1) return
    setChipPhase('shown')
    const fade = setTimeout(() => setChipPhase('fading'), CHIP_SHOW_MS)
    const gone = setTimeout(() => setChipPhase('gone'), CHIP_SHOW_MS + CHIP_FADE_MS)
    return () => {
      clearTimeout(fade)
      clearTimeout(gone)
    }
  }, [hasFrame, viewers])

  const video = (
    <video
      ref={videoRef}
      className="desktop-view__video"
      aria-label="Mac desktop"
      muted
      playsInline
      autoPlay
      onPlaying={() => { setHasFrame(true); setNeedsTap(false) }}
      onLoadedData={() => setHasFrame(true)}
    />
  )

  if (status.kind === 'other_account') return <UserRefused />

  // One card or one chip at a time; the first that applies wins.
  const locked = desk?.locked === true || desk?.state === 'locked'
  const stateCard = desk ? STATE_CARDS[desk.state] : undefined
  let cardKey: CardKey | null = null
  let chip: { Icon?: Icon; text: string; fading?: boolean } | null = null
  if (status.kind === 'token_rejected' || status.kind === 'desktop_off' || status.kind === 'unreachable') cardKey = status.kind
  else if (stateCard) cardKey = stateCard
  else if (needsTap) cardKey = 'tap'
  else if (status.kind === 'reconnecting') chip = { Icon: ArrowPathIcon, text: 'Reconnecting…' }
  else if (!hasFrame) cardKey = 'connecting'
  else if (locked) chip = { Icon: LockClosedIcon, text: 'Locked' }
  else if (chipPhase !== 'gone' && viewers !== null && viewers >= 1) {
    chip = { text: `Streaming to ${viewers} ${viewers === 1 ? 'viewer' : 'viewers'}`, fading: chipPhase === 'fading' }
  }
  const scrim = (cardKey !== null && cardKey !== 'connecting') || chip?.Icon === ArrowPathIcon
  const card = cardKey ? CARDS[cardKey] : null

  return (
    <div className="desktop-view">
      <div className="desktop-stage" ref={stageRef}>
        {video}
        <textarea
          ref={inputRef}
          className="desktop-stage__input"
          aria-label="Mac desktop"
          autoCapitalize="off"
          autoComplete="off"
          autoCorrect="off"
          spellCheck={false}
          tabIndex={-1}
        />
      </div>
      {scrim && <div className="desktop-scrim" onClick={needsTap ? startPlay : undefined} />}
      <div className="desktop-status">
        {/* The wrapper and the live region stay mounted, so every change is announced.
            The region holds text only; the card's buttons sit beside it, so an update
            never re-reads a control and focus is never inside the region. */}
        <div className={card ? 'desktop-card' : undefined}>
          <div role="status" aria-live="polite" className="desktop-status__text">
            {chip && (
              <div className={chip.fading ? 'desktop-chip desktop-chip--fading' : 'desktop-chip'}>
                {chip.Icon && <chip.Icon className="desktop-chip__icon" aria-hidden="true" />}
                {chip.text}
              </div>
            )}
            {card && (
              <>
                <card.Icon className="desktop-card__icon" aria-hidden="true" />
                <h1 className="desktop-card__heading">{card.heading}</h1>
                <p className="desktop-card__body">{card.body}</p>
              </>
            )}
            {!chip && !card && status.kind === 'streaming' && <span className="visually-hidden">Streaming</span>}
          </div>
          {cardKey === 'tap' && (
            <button type="button" className="button button--primary desktop-card__action" autoFocus onClick={startPlay}>
              Tap to Start
            </button>
          )}
          {cardKey === 'unreachable' && (
            <button
              type="button"
              className="button button--secondary desktop-card__action"
              onClick={onReload ?? (() => window.location.reload())}
            >
              Reload Page
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
