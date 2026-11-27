import React, { useCallback, useEffect, useRef, useState } from 'react'
import {
  ArrowPathIcon,
  ClockIcon,
  ComputerDesktopIcon,
  CursorArrowRaysIcon,
  ExclamationTriangleIcon,
  HandRaisedIcon,
  InformationCircleIcon,
  LockClosedIcon,
  MoonIcon,
  PlayIcon,
  SignalSlashIcon,
  WindowIcon,
} from '@heroicons/react/24/outline'
import {
  createDesktopClient,
  type ClientStatus,
  type DesktopControlMsg,
  type DesktopClient,
  type DesktopStateMsg,
  type DesktopSurfaceMsg,
} from '../lib/desktopClient'
import { HINT_KEY, createDesktopInput, type DesktopInput, type ReleaseReason } from '../lib/desktopInput'
import { isPhone } from '../lib/phoneQuery'
import { UserRefused } from './UserRefused'
import { DesktopControlBar, SWITCH_TIMEOUT_MS, type SurfacePick } from './DesktopControlBar'

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

const HINT_ID = 'desktop-stage-hint'
const SURFACE_FALLBACKS: Record<string, string | undefined> = {
  window_closed: 'The shared window closed; showing the main display',
  display_gone: 'The shared display disconnected; showing the main display',
}
const CHIP_SHOW_MS = 3000
const CHIP_FADE_MS = 150

// Notice copy is 20-UI-SPEC Surface C word for word. Every string is fixed here; the wire only picks one.
const RELEASE_CHORD_HINT = 'Typing goes to the Mac. To give the keyboard back, press Ctrl+Option+Cmd+Return.'
const WINDOW_CHIPS = {
  raise_failed: { Icon: ExclamationTriangleIcon, text: "Couldn't bring the window to the front. Click the picture to try again." },
  minimized: { Icon: WindowIcon, text: 'The shared window is minimized. Click the picture to bring it back.' },
} as const
const ACCESSIBILITY_CHIP = {
  Icon: ExclamationTriangleIcon,
  text: 'Control needs Accessibility. On the Mac, allow Portty in Privacy & Security.',
}
/** Releases the viewer did not choose and that the user can be told about (a user's own blur says nothing). */
const HELD_NOTICE: ReadonlySet<ReleaseReason> = new Set<ReleaseReason>(['lock', 'surface', 'disconnect', 'channel'])

interface Notice {
  id: number
  Icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  text: string
  dwell: number
  alert?: boolean
}

export type ControlMode = 'view' | 'control'

/** Safari with storage blocked throws on the property read itself; the setting then reads as off. */
function safeStorage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export interface DesktopViewProps {
  /** Phase 20 normalises pointer coordinates against this rect. */
  onVideoRect?: (r: VideoRect) => void
  /** View Only or Control; the bar (20-07) owns the choice. Control is the default, and View Only is not remembered. */
  controlMode?: ControlMode
  /** A bar menu, popover or dialog is open: everything held is released and the keyboard is given back. */
  menuOpen?: boolean
  /** Told when the viewer's ability to control, or its keyboard capture, changes. */
  onControlChange?: (c: { available: boolean; captured: boolean }) => void
  /** Ctrl+Option+Cmd+Return was pressed: the host moves focus to its Control segment. */
  onReleaseChord?: () => void
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
export function DesktopView({
  onVideoRect,
  onReload,
  controlMode = 'control',
  menuOpen = false,
  onControlChange,
  onReleaseChord,
}: DesktopViewProps = {}): React.ReactElement {
  const videoRef = useRef<HTMLVideoElement>(null)
  const stageRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const dotRef = useRef<HTMLDivElement>(null)
  const inputCtl = useRef<DesktopInput | null>(null)
  const rectCb = useRef(onVideoRect)
  rectCb.current = onVideoRect
  const chordCb = useRef(onReleaseChord)
  chordCb.current = onReleaseChord
  const changeCb = useRef(onControlChange)
  changeCb.current = onControlChange
  const capturedRef = useRef(false)
  const noticeSeq = useRef(0)

  const [status, setStatus] = useState<ClientStatus>({ kind: 'connecting' })
  const [desk, setDesk] = useState<DesktopStateMsg | null>(null)
  const [viewers, setViewers] = useState<number | null>(null)
  const [hasFrame, setHasFrame] = useState(false)
  const [needsTap, setNeedsTap] = useState(false)
  const [chipPhase, setChipPhase] = useState<'gone' | 'shown' | 'fading'>('gone')
  // The viewer may control the Mac (the hello grants it and this is not a phone). The keyboard is not
  // captured until a click gives the stage focus.
  const [canControl, setCanControl] = useState(false)
  const [captured, setCaptured] = useState(false)
  const [ctlState, setCtlState] = useState<DesktopControlMsg>({ input: 'ok', window: '' })
  const [notice, setNotice] = useState<Notice | null>(null)
  const [noticeFading, setNoticeFading] = useState(false)
  // The shared surface as the daemon last said it, and whether a bar menu or dialog is open.
  const [surf, setSurf] = useState<DesktopSurfaceMsg | null>(null)
  // A pick is in flight until its surface message arrives, the daemon refuses, or SWITCH_TIMEOUT_MS passes.
  const [switching, setSwitching] = useState<(SurfacePick & { previous: string }) | null>(null)
  const clientRef = useRef<DesktopClient | null>(null)
  const surfRef = useRef<DesktopSurfaceMsg | null>(null)
  const switchingRef = useRef<(SurfacePick & { previous: string }) | null>(null)
  const [barMenuOpen, setBarMenuOpen] = useState(false)
  // Whether the stage had the keyboard when the pointer went down on the bar (the press itself blurs it).
  const stageHadFocus = useRef(false)

  const pushNotice = useCallback((n: Omit<Notice, 'id'>) => {
    noticeSeq.current++
    setNoticeFading(false)
    setNotice({ ...n, id: noticeSeq.current })
  }, [])
  const typingStays = useCallback(
    () => pushNotice({ Icon: CursorArrowRaysIcon, text: 'Typing stays on this device', dwell: 3000 }),
    [pushNotice],
  )

  // The label returns to the previous surface and the user is told, in words, that it did not happen.
  const switchFailed = useCallback(() => {
    const sw = switchingRef.current
    if (!sw) return
    switchingRef.current = null
    setSwitching(null)
    pushNotice({
      Icon: ExclamationTriangleIcon,
      text: `Couldn't switch to ${sw.name}. Still showing ${sw.previous}.`,
      dwell: 5000,
      alert: true,
    })
  }, [pushNotice])

  const switchFailedRef = useRef(switchFailed)
  switchFailedRef.current = switchFailed
  useEffect(() => {
    if (!switching) return
    const id = setTimeout(switchFailed, SWITCH_TIMEOUT_MS)
    return () => clearTimeout(id)
  }, [switching, switchFailed])

  // A notice dwells, then fades, then goes; a newer one replaces it and restarts the clock.
  useEffect(() => {
    if (!notice) return
    const fade = setTimeout(() => setNoticeFading(true), notice.dwell)
    const gone = setTimeout(() => setNotice(null), notice.dwell + CHIP_FADE_MS)
    return () => {
      clearTimeout(fade)
      clearTimeout(gone)
    }
  }, [notice])

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
    clientRef.current = client
    const phone = isPhone()
    let lastEpoch: number | null = null
    const offs = [
      client.on('rights', (r) => {
        const stage = stageRef.current
        const input = inputRef.current
        if (!r.control || phone || inputCtl.current || !stage || !input) return
        const ctl = createDesktopInput(
          stage,
          input,
          {
            send: (msg, droppable = false) => client.sendInput(msg, droppable),
            getEpoch: () => client.currentEpoch(),
            getRect: () => {
              const v = videoRef.current
              return v ? videoContentRect(v.clientWidth, v.clientHeight, v.videoWidth, v.videoHeight) : { x: 0, y: 0, width: 0, height: 0 }
            },
            storage: safeStorage(),
          },
          {
            // Straight to the element, on the animation frame of the move: no React render between.
            onDot: (p) => {
              const d = dotRef.current
              if (!d) return
              d.hidden = p === null
              if (p) d.style.transform = `translate(${p.x - 4}px, ${p.y - 4}px)`
            },
            onCapture: (c) => {
              capturedRef.current = c
              setCaptured(c)
              if (!c) return
              let seen = false
              try {
                seen = window.localStorage.getItem(HINT_KEY) === '1'
                if (!seen) window.localStorage.setItem(HINT_KEY, '1')
              } catch {
                // Storage blocked: the long hint shows each time, which is harmless.
              }
              pushNotice(
                seen
                  ? { Icon: CursorArrowRaysIcon, text: 'Typing goes to the Mac', dwell: 3000 }
                  : { Icon: CursorArrowRaysIcon, text: RELEASE_CHORD_HINT, dwell: 5000 },
              )
            },
            onReleaseChord: () => {
              typingStays()
              chordCb.current?.()
            },
            onReleased: (reason) => {
              if (HELD_NOTICE.has(reason)) pushNotice({ Icon: HandRaisedIcon, text: 'Held keys were released', dwell: 3000 })
            },
          },
        )
        ctl.setControl(true)
        inputCtl.current = ctl
        setCanControl(true)
      }),
      client.on('input', ({ open }) => {
        if (open) return
        inputCtl.current?.releaseAll('channel')
        setCtlState({ input: 'ok', window: '' })
      }),
      client.on('control', setCtlState),
      client.on('surface', (sf) => {
        // Every viewer, a phone too, hears what the Mac now shows. The notice goes first so that, when
        // something was held, "Held keys were released" (sent by the release below) is the one that stays.
        const prev = surfRef.current
        surfRef.current = sf
        setSurf(sf)
        const sw = switchingRef.current
        if (sw && (sf.reason !== '' || (sf.kind === sw.kind && sf.id === sw.id))) {
          switchingRef.current = null
          setSwitching(null)
        }
        const surfaceNotice = SURFACE_FALLBACKS[sf.reason]
        if (surfaceNotice) pushNotice({ Icon: InformationCircleIcon, text: surfaceNotice, dwell: 5000 })
        else if (prev && (prev.kind !== sf.kind || prev.id !== sf.id)) {
          pushNotice({
            Icon: sf.kind === 'window' ? WindowIcon : ComputerDesktopIcon,
            text: sf.kind === 'display' && sf.name === 'Main Display' ? 'Showing the main display' : `Showing ${sf.name}`,
            dwell: 3000,
          })
        }
        if (lastEpoch !== null && sf.e !== lastEpoch) inputCtl.current?.releaseAll('surface')
        lastEpoch = sf.e
      }),
      client.on('selectFailed', () => switchFailedRef.current()),
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
      clientRef.current = null
    }
  }, [startPlay, updateRect, pushNotice, typingStays])

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

  const locked = desk?.locked === true || desk?.state === 'locked'
  const accessNeeded = ctlState.input === 'needs_accessibility'
  const active = canControl && controlMode === 'control'
  const paused = locked || status.kind === 'reconnecting' || status.kind === 'unreachable' || accessNeeded

  useEffect(() => {
    const ctl = inputCtl.current
    if (!ctl) return
    // The user choosing View Only while the keyboard was captured gives it back with a word.
    if (!active && capturedRef.current) typingStays()
    ctl.setControl(active)
  }, [active, typingStays])

  useEffect(() => {
    inputCtl.current?.setPaused(paused)
  }, [paused, canControl])

  // Held input is let go of when the Mac locks or the picture is lost; the user keeps the keyboard.
  useEffect(() => {
    if (locked) inputCtl.current?.releaseAll('lock')
  }, [locked, canControl])
  useEffect(() => {
    if (status.kind === 'reconnecting' || status.kind === 'unreachable') inputCtl.current?.releaseAll('disconnect')
  }, [status.kind, canControl])

  // Opening a bar menu or dialog is a user action: release, hand the keyboard back, say so.
  const anyMenuOpen = menuOpen || barMenuOpen
  useEffect(() => {
    const ctl = inputCtl.current
    if (!anyMenuOpen || !ctl) return
    if (capturedRef.current) typingStays()
    ctl.releaseAll('menu')
  }, [anyMenuOpen, canControl, typingStays])

  useEffect(() => {
    changeCb.current?.({ available: canControl, captured })
  }, [canControl, captured])

  if (status.kind === 'other_account') return <UserRefused />

  // One card or one chip at a time; the first that applies wins.
  const stateCard = desk ? STATE_CARDS[desk.state] : undefined
  let cardKey: CardKey | null = null
  let chip: { Icon?: Icon; text: string; fading?: boolean } | null = null
  if (status.kind === 'token_rejected' || status.kind === 'desktop_off' || status.kind === 'unreachable') cardKey = status.kind
  else if (stateCard) cardKey = stateCard
  else if (needsTap) cardKey = 'tap'
  else if (status.kind === 'reconnecting') chip = { Icon: ArrowPathIcon, text: 'Reconnecting…' }
  else if (!hasFrame) cardKey = 'connecting'
  else if (locked) chip = { Icon: LockClosedIcon, text: 'Locked' }
  else if (canControl && ctlState.window !== '') chip = WINDOW_CHIPS[ctlState.window]
  else if (canControl && accessNeeded) chip = ACCESSIBILITY_CHIP
  else if (chipPhase !== 'gone' && viewers !== null && viewers >= 1) {
    chip = { text: `Streaming to ${viewers} ${viewers === 1 ? 'viewer' : 'viewers'}`, fading: chipPhase === 'fading' }
  }
  // The bar is for a viewer that may control the Mac, while a picture is up and no card covers it.
  const showBar = canControl && cardKey === null && (status.kind === 'streaming' || status.kind === 'reconnecting')
  const scrim = (cardKey !== null && cardKey !== 'connecting') || chip?.Icon === ArrowPathIcon
  const card = cardKey ? CARDS[cardKey] : null

  return (
    <div className="desktop-view">
      <div
        className={active && !paused ? 'desktop-stage desktop-stage--live' : 'desktop-stage'}
        ref={stageRef}
        {...(canControl
          ? { role: 'application', 'aria-label': 'Mac desktop', 'aria-describedby': HINT_ID, tabIndex: active ? 0 : -1 }
          : {})}
      >
        {video}
        {canControl && captured && <div className="desktop-ring" aria-hidden="true" />}
        {canControl && <div className="desktop-dot" ref={dotRef} aria-hidden="true" hidden />}
        <span id={HINT_ID} className="visually-hidden">
          While this has focus, your keyboard and pointer control the Mac. To give the keyboard back, press Control, Option, Command and Return together.
        </span>
        <textarea
          ref={inputRef}
          className="desktop-stage__input"
          aria-label="Mac desktop"
          aria-describedby={HINT_ID}
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
        {/* The notice slot: one at a time, the newest replaces the older, directly below the state chip. */}
        <div role="status" aria-live="polite" className="desktop-notice-slot">
          {notice && (
            <div
              key={notice.id}
              role={notice.alert ? 'alert' : undefined}
              className={noticeFading ? 'desktop-chip desktop-chip--notice desktop-chip--fading' : 'desktop-chip desktop-chip--notice'}
            >
              <notice.Icon className="desktop-chip__icon" aria-hidden="true" />
              {notice.text}
            </div>
          )}
        </div>
      </div>
      {showBar && (
        <div className="desktop-overlay" onPointerDownCapture={() => { stageHadFocus.current = capturedRef.current }}>
          <DesktopControlBar
            windowShare={surf?.kind === 'window'}
            surface={surf ? { kind: surf.kind, id: surf.id, name: surf.name } : null}
            switching={switching !== null}
            requestSurfaces={() => clientRef.current?.requestSurfaces() ?? Promise.reject(new Error('not connected'))}
            onSelectSurface={(pick) => {
              if (switchingRef.current) return
              const sw = { ...pick, previous: surfRef.current?.name ?? 'the main display' }
              switchingRef.current = sw
              setSwitching(sw)
              clientRef.current?.selectSurface(pick.kind, pick.id)
            }}
            sendChord={(codes) => inputCtl.current?.sendChord(codes)}
            onMenuOpenChange={setBarMenuOpen}
            onSendKeysDone={() => { if (stageHadFocus.current) inputRef.current?.focus({ preventScroll: true }) }}
          />
        </div>
      )}
    </div>
  )
}
