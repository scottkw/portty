import React, { useCallback, useEffect, useImperativeHandle, useLayoutEffect, useRef, useState } from 'react'
import {
  ArrowPathIcon,
  ClipboardDocumentCheckIcon,
  ClipboardIcon,
  ClockIcon,
  ArrowsPointingOutIcon,
  ArrowsRightLeftIcon,
  ComputerDesktopIcon,
  CursorArrowRaysIcon,
  ExclamationTriangleIcon,
  EyeSlashIcon,
  HandRaisedIcon,
  InformationCircleIcon,
  LockClosedIcon,
  MoonIcon,
  PlayIcon,
  SignalSlashIcon,
  WindowIcon,
} from '@heroicons/react/24/outline'
import {
  CLIP_MAX_BYTES,
  clipByteLength,
  createDesktopClient,
  type ClientStatus,
  type ClipOutcome,
  type DesktopClientOptions,
  type DesktopControlMsg,
  type DesktopClient,
  type DesktopStateMsg,
  type DesktopSurfaceMsg,
} from '../lib/desktopClient'
import { HINT_KEY, createDesktopInput, type DesktopInput, type ReleaseReason } from '../lib/desktopInput'
import { isPhone } from '../lib/phoneQuery'
import { UserRefused } from './UserRefused'
import { DesktopCopyPopover, DesktopPastePopover } from './DesktopClipboardPopover'
import { DesktopControlBar, SWITCH_TIMEOUT_MS, type ControlMode, type SurfacePick } from './DesktopControlBar'

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
export type CardKey =
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

/** What the host shell may do to a viewer in control mode (Task 3 fills the handle). */
export interface DesktopControlHandle {
  releaseAll(): void
  releaseKeyboard(): void
  notify(text: string, kind: 'status' | 'alert'): void
}

/** What a host may replace on a card: its words, and the label of its one action. */
export interface CardOverride {
  heading: string
  body: string
  action?: string
}

/** A card the host shell shows over every client card (the app's Local Network card). Its actions sit outside the live region. */
export interface HostCard {
  heading: string
  body: string
  actions: { label: string; onClick: () => void }[]
}

/** The page's own button on a card, when the host gives none. Only the unreachable card has one. */
const DEFAULT_ACTIONS: Partial<Record<CardKey, string>> = { unreachable: 'Reload Page' }

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
/** A transfer that takes longer than this shows Copying from the Mac… or Sending to the Mac… until it ends. */
export const CLIP_SLOW_MS = 400

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
  /** Stays until a newer notice replaces it (a transfer in flight); dwell is ignored. */
  persistent?: boolean
}

// Clipboard notices are 20-UI-SPEC Surface B and C word for word. Every string is fixed here.
type NoticeSpec = Omit<Notice, 'id'>
const COPY_NOTICES: Partial<Record<ClipOutcome, NoticeSpec>> = {
  ok: { Icon: ClipboardDocumentCheckIcon, text: 'Copied from Mac', dwell: 3000 },
  concealed: {
    Icon: EyeSlashIcon,
    text: "The Mac's clipboard holds a hidden item (password manager); not copied",
    dwell: 5000,
  },
  empty: { Icon: ClipboardIcon, text: "The Mac's clipboard has no text", dwell: 3000 },
  waiting_approval: {
    Icon: ClockIcon,
    text: "Waiting for approval at the Mac. macOS is asking before Portty reads its clipboard.",
    dwell: 5000,
  },
  too_large: { Icon: ExclamationTriangleIcon, text: "The Mac's clipboard text is too large to copy (limit 1 MB)", dwell: 5000 },
}
const COPY_FAILED: NoticeSpec = { Icon: ExclamationTriangleIcon, text: "Couldn't copy from the Mac. Try again.", dwell: 5000, alert: true }
const PASTE_NOTICES: Partial<Record<ClipOutcome, NoticeSpec>> = {
  ok: { Icon: ClipboardDocumentCheckIcon, text: "Sent to the Mac's clipboard; press ⌘V", dwell: 5000 },
  empty: { Icon: ClipboardIcon, text: 'Your clipboard has no text', dwell: 3000 },
  too_large: { Icon: ExclamationTriangleIcon, text: 'Your clipboard text is too large to send (limit 1 MB)', dwell: 5000 },
}
const PASTE_FAILED: NoticeSpec = { Icon: ExclamationTriangleIcon, text: "Couldn't send to the Mac. Try again.", dwell: 5000, alert: true }

export type { ControlMode }

/** Safari with storage blocked throws on the property read itself; the setting then reads as off. */
function safeStorage(): Storage | null {
  try {
    return window.localStorage
  } catch {
    return null
  }
}

interface KeyboardApi {
  lock?: (codes?: string[]) => Promise<void>
  unlock?: () => void
}
/** navigator.keyboard (Chrome, Edge); absent in Safari and Firefox. */
function keyboardApi(): KeyboardApi | undefined {
  return (navigator as Navigator & { keyboard?: KeyboardApi }).keyboard
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
  /**
   * Phase 21-08, the Portty app's Desktop tab embeds this view. Every prop below is optional and its
   * default leaves the /desktop/ page as Phase 20 built it.
   * Connection options for createDesktopClient (the loopback transport and its hooks); read once at mount.
   */
  clientOptions?: DesktopClientOptions
  /** Inside a tab: no page title, no splash removal, and the root is contained by the tab, not fixed to the window. */
  embedded?: boolean
  /** The client, once created, and null when it is torn down. */
  onClient?: (c: DesktopClient | null) => void
  /** Every status the client reports. */
  onStatus?: (s: ClientStatus) => void
  /** The Mac's last state message; null when the connection that sent it is gone. */
  onDesktopState?: (s: DesktopStateMsg | null) => void
  /** Per-card words and action label, merged over the page's copy (the app's wording has no link, page or browser). */
  cards?: Partial<Record<CardKey, CardOverride>>
  /** A card that wins over every client card. */
  hostCard?: HostCard | null
  /** The host stopped the client (a hidden window): shows the Reconnecting chip until the host clears it. */
  paused?: boolean
  /** True while a picture shows and no card covers it. */
  onPicture?: (has: boolean) => void
  /** True when the stage has the keyboard (the user clicked or tabbed onto it), false when it goes. */
  onCaptureChange?: (captured: boolean) => void
  /** The bar's View Only or Control choice, reported at mount and on every change. */
  onModeChange?: (mode: ControlMode) => void
  /** What the host shell may do in control mode: release keys, give the keyboard back, show a notice. */
  controlRef?: React.Ref<DesktopControlHandle>
  /** The host shows who owns the keyboard itself, so the two capture notices (Typing goes to the Mac, Typing stays on this device) never render. */
  hideCaptureNotices?: boolean
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
  clientOptions,
  embedded = false,
  onClient,
  onStatus,
  onDesktopState,
  cards,
  hostCard = null,
  paused: hostPaused = false,
  onPicture,
  onCaptureChange,
  onModeChange,
  controlRef,
  hideCaptureNotices = false,
}: DesktopViewProps = {}): React.ReactElement {
  const rootRef = useRef<HTMLDivElement>(null)
  const controlSegRef = useRef<HTMLButtonElement>(null)
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
  const optionsRef = useRef(clientOptions)
  const clientCb = useRef(onClient)
  clientCb.current = onClient
  const statusCb = useRef(onStatus)
  statusCb.current = onStatus
  const stateCb = useRef(onDesktopState)
  stateCb.current = onDesktopState
  const pictureCb = useRef(onPicture)
  pictureCb.current = onPicture
  const captureCb = useRef(onCaptureChange)
  captureCb.current = onCaptureChange
  const modeCb = useRef(onModeChange)
  modeCb.current = onModeChange
  const hideCaptureRef = useRef(hideCaptureNotices)
  hideCaptureRef.current = hideCaptureNotices
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
  // The bar owns View Only or Control; the controlMode prop sets it, and a change of the prop wins.
  const [mode, setMode] = useState<ControlMode>(controlMode)
  useEffect(() => setMode(controlMode), [controlMode])
  const [swapOn, setSwapOn] = useState(false)
  const [canSurface, setCanSurface] = useState(false)
  const [canClipboard, setCanClipboard] = useState(false)
  // A clipboard transfer is in flight: both Clipboard items are greyed so two never overlap.
  const [clipBusy, setClipBusy] = useState(false)
  const clipBusyRef = useRef(false)
  // The one open popover. The Copy popover holds the Mac's text for as long as it is open: memory only, never logged.
  const [popover, setPopover] = useState<{ kind: 'copy'; text: string } | { kind: 'paste' } | null>(null)
  const clipboardBtnRef = useRef<HTMLButtonElement>(null)
  const chipRef = useRef<HTMLDivElement>(null)
  const [chipH, setChipH] = useState(0)
  const showBarRef = useRef(false)
  const barMenuOpenRef = useRef(false)
  const [fullscreen, setFullscreen] = useState(false)
  // Whether the stage had the keyboard when the pointer went down on the bar (the press itself blurs it).
  const stageHadFocus = useRef(false)

  const pushNotice = useCallback((n: Omit<Notice, 'id'>) => {
    noticeSeq.current++
    setNoticeFading(false)
    setNotice({ ...n, id: noticeSeq.current })
  }, [])
  const typingStays = useCallback(() => {
    if (hideCaptureRef.current) return
    pushNotice({ Icon: CursorArrowRaysIcon, text: 'Typing stays on this device', dwell: 3000 })
  }, [pushNotice])

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
    if (!notice || notice.persistent) return
    const fade = setTimeout(() => setNoticeFading(true), notice.dwell)
    const gone = setTimeout(() => setNotice(null), notice.dwell + CHIP_FADE_MS)
    return () => {
      clearTimeout(fade)
      clearTimeout(gone)
    }
  }, [notice])

  useEffect(() => {
    // Inside the app, the window's title and splash are App's, not this view's.
    if (!embedded) {
      document.title = TITLE
      let meta = document.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-title"]')
      if (!meta) {
        meta = document.createElement('meta')
        meta.name = 'apple-mobile-web-app-title'
        document.head.appendChild(meta)
      }
      meta.content = TITLE
      // index.html's static splash belongs to the Wails window (App hides it on mount). The viewer never
      // mounts App, so left alone it sits opaque over the picture and takes every pointer event (20-09, E1).
      document.getElementById('splash-static')?.remove()
    }
    // React sets the muted property but not the attribute; Safari's autoplay policy reads the attribute.
    videoRef.current?.setAttribute('muted', '')
  }, [embedded])

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
    const client = createDesktopClient(optionsRef.current)
    clientRef.current = client
    clientCb.current?.(client)
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
              if (!c || hideCaptureRef.current) return
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
              // The keyboard is the viewer's own again: the bar shows and focus waits on its Control segment.
              controlSegRef.current?.focus()
            },
            onReleased: (reason) => {
              if (HELD_NOTICE.has(reason)) pushNotice({ Icon: HandRaisedIcon, text: 'Held keys were released', dwell: 3000 })
            },
          },
        )
        ctl.setControl(true)
        inputCtl.current = ctl
        setSwapOn(ctl.swap())
        setCanSurface(r.surface)
        setCanClipboard(r.clipboard)
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
        stateCb.current?.(s)
      }),
      client.on('size', updateRect),
      client.on('status', (s) => {
        setStatus(s)
        // A state card describes the connection that is gone; the next state message replaces it.
        if (s.kind !== 'streaming' && s.kind !== 'connecting') {
          setDesk(null)
          stateCb.current?.(null)
        }
        statusCb.current?.(s)
      }),
    ]
    client.start()
    return () => {
      for (const off of offs) off()
      inputCtl.current?.destroy()
      inputCtl.current = null
      client.stop()
      clientRef.current = null
      clientCb.current?.(null)
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
  const active = canControl && mode === 'control'
  const inputPaused = hostPaused || locked || status.kind === 'reconnecting' || status.kind === 'unreachable' || accessNeeded

  useEffect(() => {
    const ctl = inputCtl.current
    if (!ctl) return
    // The user choosing View Only while the keyboard was captured gives it back with a word.
    if (!active && capturedRef.current) typingStays()
    ctl.setControl(active)
  }, [active, typingStays])

  useEffect(() => {
    inputCtl.current?.setPaused(inputPaused)
  }, [inputPaused, canControl])

  // Held input is let go of when the Mac locks or the picture is lost; the user keeps the keyboard.
  useEffect(() => {
    if (locked) inputCtl.current?.releaseAll('lock')
  }, [locked, canControl])
  useEffect(() => {
    if (hostPaused || status.kind === 'reconnecting' || status.kind === 'unreachable') inputCtl.current?.releaseAll('disconnect')
  }, [hostPaused, status.kind, canControl])

  // Opening a bar menu or dialog is a user action: release, hand the keyboard back, say so.
  const anyMenuOpen = menuOpen || barMenuOpen || popover !== null
  // At most one menu, popover or dialog: opening a bar menu closes a popover.
  useEffect(() => {
    if (barMenuOpen) setPopover(null)
  }, [barMenuOpen])

  useEffect(() => {
    const ctl = inputCtl.current
    if (!anyMenuOpen || !ctl) return
    if (capturedRef.current) typingStays()
    ctl.releaseAll('menu')
  }, [anyMenuOpen, canControl, typingStays])

  useEffect(() => {
    changeCb.current?.({ available: canControl, captured })
  }, [canControl, captured])
  useEffect(() => {
    captureCb.current?.(captured)
  }, [captured])
  useEffect(() => {
    modeCb.current?.(mode)
  }, [mode])

  useImperativeHandle(
    controlRef,
    () => ({
      // The user's own doing (a tab switch, a click away), so no Held keys notice.
      releaseAll: () => inputCtl.current?.releaseAll('blur'),
      releaseKeyboard: () => inputRef.current?.blur(),
      notify: (text, kind) => pushNotice({ Icon: InformationCircleIcon, text, dwell: 3000, alert: kind === 'alert' }),
    }),
    [pushNotice],
  )

  // Leaving full screen (the button, a long Esc, the browser) lets go of the keyboard lock and every held key.
  useEffect(() => {
    const onChange = () => {
      const on = document.fullscreenElement != null
      setFullscreen(on)
      if (on) return
      keyboardApi()?.unlock?.()
      inputCtl.current?.releaseAll('blur')
    }
    document.addEventListener('fullscreenchange', onChange)
    return () => document.removeEventListener('fullscreenchange', onChange)
  }, [])

  async function toggleFullscreen() {
    if (document.fullscreenElement) {
      await document.exitFullscreen().catch(() => {})
      return
    }
    const root = rootRef.current
    if (!root) return
    try {
      await root.requestFullscreen()
    } catch {
      return // The browser refused full screen itself; nothing changed, nothing to explain.
    }
    // The click on the button blurred the stage, and Keyboard Lock only helps a focused page.
    stageRef.current?.focus({ preventScroll: true })
    let granted = false
    try {
      await keyboardApi()?.lock?.()
      granted = keyboardApi()?.lock !== undefined
    } catch {
      // A refused lock is never an error: the browser keeps its shortcuts.
    }
    pushNotice({
      Icon: ArrowsPointingOutIcon,
      text: granted
        ? 'Cmd and Esc shortcuts now go to the Mac. Hold Esc to leave full screen.'
        : 'Full screen is on. This browser keeps Cmd shortcuts; use Send Keys for Cmd+Tab and Cmd+Space.',
      dwell: 5000,
    })
  }

  function toggleSwap(on: boolean) {
    inputCtl.current?.setSwap(on)
    setSwapOn(on)
    pushNotice({
      Icon: ArrowsRightLeftIcon,
      text: on ? 'Ctrl and Cmd are swapped' : 'Ctrl and Cmd are back to normal',
      dwell: 3000,
    })
  }

  /** Marks a transfer in flight; the slow notice shows only if it outlasts CLIP_SLOW_MS. Returns the end of the transfer. */
  function beginTransfer(slowText: string): () => void {
    clipBusyRef.current = true
    setClipBusy(true)
    const slow = setTimeout(() => pushNotice({ Icon: ClipboardIcon, text: slowText, dwell: 0, persistent: true }), CLIP_SLOW_MS)
    return () => {
      clearTimeout(slow)
      clipBusyRef.current = false
      setClipBusy(false)
    }
  }

  /** Opens a popover unless something else is in the way (a bar menu, or no bar). The popover is the feedback, so a Copying… notice goes. */
  function openPopover(p: NonNullable<typeof popover>): boolean {
    if (!showBarRef.current || barMenuOpenRef.current) return false
    setNotice((n) => (n?.persistent ? null : n))
    setPopover(p)
    return true
  }

  /** Copy from Mac. Runs inside the click: the browser write starts now, with a promise for the text (Safari's gesture rule). */
  function copyFromMac() {
    const client = clientRef.current
    if (!client || clipBusyRef.current) return
    const end = beginTransfer('Copying from the Mac…')
    const result = client.copyFromMac()
    const clip = navigator.clipboard
    let written: Promise<boolean>
    if (typeof ClipboardItem !== 'undefined' && clip?.write) {
      // Rejects on any outcome but ok, so a refused copy writes nothing.
      const blob = result.then((r) => {
        if (r.outcome !== 'ok' || !r.text) throw new Error(r.outcome)
        return new Blob([r.text], { type: 'text/plain' })
      })
      blob.catch(() => {})
      try {
        written = clip.write([new ClipboardItem({ 'text/plain': blob })]).then(() => true, () => false)
      } catch {
        written = Promise.resolve(false)
      }
    } else {
      written = result.then(async (r) => {
        if (r.outcome !== 'ok' || !r.text || !clip?.writeText) return false
        try {
          await clip.writeText(r.text)
          return true
        } catch {
          return false
        }
      })
    }
    void (async () => {
      const r = await result
      const ok = await written
      end()
      if (r.outcome === 'ok' && r.text) {
        if (ok) pushNotice(COPY_NOTICES.ok!)
        else if (!openPopover({ kind: 'copy', text: r.text })) pushNotice(COPY_FAILED)
      } else {
        pushNotice(r.outcome === 'ok' ? COPY_NOTICES.empty! : (COPY_NOTICES[r.outcome] ?? COPY_FAILED))
      }
    })()
  }

  async function sendToMac(text: string) {
    const client = clientRef.current
    if (!client) return
    const n = clipByteLength(text)
    if (n === 0) return pushNotice(PASTE_NOTICES.empty!)
    if (n > CLIP_MAX_BYTES) return pushNotice(PASTE_NOTICES.too_large!)
    if (clipBusyRef.current) return
    const end = beginTransfer('Sending to the Mac…')
    const { outcome } = await client.pasteToMac(text)
    end()
    pushNotice(PASTE_NOTICES[outcome] ?? PASTE_FAILED)
  }

  /** Paste to Mac. The read of the viewer's clipboard starts inside the click; it is sent at once. */
  async function pasteToMac() {
    if (!clientRef.current || clipBusyRef.current) return
    const clip = navigator.clipboard
    if (!clip?.readText) {
      // Still inside the menu item's own handler, where the Clipboard menu counts as open until it closes just after.
      await Promise.resolve()
      if (!openPopover({ kind: 'paste' })) pushNotice(PASTE_FAILED)
      return
    }
    clipBusyRef.current = true
    setClipBusy(true)
    let text: string
    try {
      text = await clip.readText()
    } catch {
      clipBusyRef.current = false
      setClipBusy(false)
      if (!openPopover({ kind: 'paste' })) pushNotice(PASTE_FAILED)
      return
    }
    clipBusyRef.current = false
    setClipBusy(false)
    await sendToMac(text)
  }

  // One card or one chip at a time; the first that applies wins.
  const stateCard = desk ? STATE_CARDS[desk.state] : undefined
  let cardKey: CardKey | null = null
  let chip: { Icon?: Icon; text: string; fading?: boolean } | null = null
  // Embedded, the other-account case is the Refused card (the page's Refused screen talks about a link).
  const clientStatus = embedded && status.kind === 'other_account' ? 'token_rejected' : status.kind
  if (hostCard) cardKey = null
  else if (hostPaused) chip = { Icon: ArrowPathIcon, text: 'Reconnecting…' }
  else if (clientStatus === 'token_rejected' || clientStatus === 'desktop_off' || clientStatus === 'unreachable') cardKey = clientStatus
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
  const showBar = canControl && cardKey === null && !hostCard && !hostPaused && (status.kind === 'streaming' || status.kind === 'reconnecting')
  showBarRef.current = showBar
  const picture = hasFrame && cardKey === null && !hostCard && !hostPaused
  useEffect(() => {
    pictureCb.current?.(picture)
  }, [picture])
  // A popover belongs to the bar: when the bar goes (a card covers the picture), so does it.
  // A menu open when the bar went would otherwise block the copy/paste fallback popover
  // (the barMenuOpenRef guard) after the bar returns.
  useEffect(() => {
    if (!showBar) {
      setPopover(null)
      barMenuOpenRef.current = false
      setBarMenuOpen(false)
    }
  }, [showBar])

  // The notice chip sits below the state chip, so it needs the state chip's height once that wraps.
  const measureChip = useCallback(() => {
    const h = chipRef.current?.offsetHeight ?? 0
    setChipH((prev) => (prev === h ? prev : h))
  }, [])
  useLayoutEffect(measureChip)
  useEffect(() => {
    window.addEventListener('resize', measureChip)
    return () => window.removeEventListener('resize', measureChip)
  }, [measureChip])

  if (status.kind === 'other_account' && !embedded) return <UserRefused />
  const scrim = hostCard !== null || (cardKey !== null && cardKey !== 'connecting') || chip?.Icon === ArrowPathIcon
  const card = hostCard
    ? { Icon: ExclamationTriangleIcon, heading: hostCard.heading, body: hostCard.body }
    : cardKey
      ? { ...CARDS[cardKey], ...cards?.[cardKey] }
      : null
  const cardAction = cardKey ? (cards?.[cardKey]?.action ?? DEFAULT_ACTIONS[cardKey]) : undefined

  return (
    <div
      ref={rootRef}
      className={`desktop-view${canControl ? ' desktop-view--bar' : ''}${embedded ? ' desktop-view--embedded' : ''}`}
    >
      <div
        className={active && !inputPaused ? 'desktop-stage desktop-stage--live' : 'desktop-stage'}
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
      <div
        className="desktop-status"
        style={chipH > 32 ? ({ '--desktop-chip-h': `${chipH}px` } as React.CSSProperties) : undefined}
      >
        {/* The wrapper and the live region stay mounted, so every change is announced.
            The region holds text only; the card's buttons sit beside it, so an update
            never re-reads a control and focus is never inside the region. */}
        <div className={card ? 'desktop-card' : undefined}>
          <div role="status" aria-live="polite" className="desktop-status__text">
            {chip && (
              <div ref={chipRef} className={chip.fading ? 'desktop-chip desktop-chip--fading' : 'desktop-chip'}>
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
              {cards?.tap?.action ?? 'Tap to Start'}
            </button>
          )}
          {(cardKey === 'unreachable' || cardKey === 'token_rejected') && cardAction && (
            <button
              type="button"
              className="button button--secondary desktop-card__action"
              onClick={onReload ?? (() => window.location.reload())}
            >
              {cardAction}
            </button>
          )}
          {hostCard?.actions.map((a) => (
            <button key={a.label} type="button" className="button button--secondary desktop-card__action" onClick={a.onClick}>
              {a.label}
            </button>
          ))}
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
        <div
          className="desktop-overlay"
          onPointerDownCapture={() => {
            // The press itself takes the keyboard off the stage, so read it now.
            stageHadFocus.current = capturedRef.current
            if (capturedRef.current) typingStays()
          }}
        >
          <DesktopControlBar
            state={{
              locked,
              reconnecting: status.kind === 'reconnecting',
              accessNeeded,
              switching: switching !== null,
              mode,
              canSurface,
              fullscreenAvailable: !embedded && document.fullscreenEnabled === true,
              canClipboard,
            }}
            surface={surf ? { kind: surf.kind, id: surf.id, name: surf.name } : null}
            requestSurfaces={() => clientRef.current?.requestSurfaces() ?? Promise.reject(new Error('not connected'))}
            onSelectSurface={(pick) => {
              if (switchingRef.current) return
              const sw = { ...pick, previous: surfRef.current?.name ?? 'the main display' }
              switchingRef.current = sw
              setSwitching(sw)
              clientRef.current?.selectSurface(pick.kind, pick.id)
            }}
            sendChord={(codes) => inputCtl.current?.sendChord(codes)}
            onMenuOpenChange={(open) => {
                barMenuOpenRef.current = open // eagerly, for a popover that opens before the next render
                setBarMenuOpen(open)
              }}
            onSendKeysDone={() => { if (stageHadFocus.current) inputRef.current?.focus({ preventScroll: true }) }}
            swapOn={swapOn}
            onSwapChange={toggleSwap}
            onModeChange={setMode}
            fullscreen={fullscreen}
            onFullscreen={() => void toggleFullscreen()}
            controlRef={controlSegRef}
            onCopy={copyFromMac}
            onPaste={() => void pasteToMac()}
            clipBusy={clipBusy}
            clipboardRef={clipboardBtnRef}
            popoverOpen={popover !== null}
          />
        </div>
      )}
      {showBar && popover?.kind === 'copy' && (
        <DesktopCopyPopover text={popover.text} anchor={clipboardBtnRef.current} onClose={() => setPopover(null)} />
      )}
      {showBar && popover?.kind === 'paste' && (
        <DesktopPastePopover
          anchor={clipboardBtnRef.current}
          onSend={(text) => void sendToMac(text)}
          onClose={() => setPopover(null)}
        />
      )}
    </div>
  )
}
