import React, { useEffect, useRef, useState } from 'react'
import type { DesktopClient, DesktopCursorMsg } from '../lib/desktopClient'
import {
  FIT,
  normalizedDelta,
  panToKeepPointer,
  pointerToStage,
  scrollDelta,
  videoTransform,
  zoomAround,
  type ZoomState,
} from '../lib/desktopCoords'
import { createPhoneInput, type PhoneInput } from '../lib/desktopPhoneInput'
import { attachTrackpad, type TrackpadIntent } from '../lib/touchTrackpad'
import { videoContentRect } from './DesktopView'

export interface DesktopPhoneLayerProps {
  client: DesktopClient
  video: HTMLVideoElement | null
  stage: HTMLElement | null
  rights: { control: boolean; surface: boolean } | null
  locked: boolean
  reconnecting: boolean
}

/** A tap before the Mac's first cursor report waits this long for it, then is dropped. */
const CURSOR_WAIT_MS = 500
const RIPPLE_MS = 150
const RIPPLE_PX = 24

/**
 * Phase 21-10 (DVWR-03) — the iPhone touch layer on the viewer's stage. Phase 20's controller is not
 * created on a phone, so this is the only thing that sends input from one. It listens on the stage
 * (never window or document for touches) and sends Phase 20 messages. The pointer lives on the Mac:
 * drags are relative moves, clicks use the Mac's last cursor report, and zoom never leaves the phone.
 */
export function DesktopPhoneLayer({ client, video, stage, rights, locked, reconnecting }: DesktopPhoneLayerProps): React.ReactElement {
  const canControl = rights?.control === true
  const paused = locked || reconnecting
  const live = useRef({ canControl, paused })
  live.current = { canControl, paused }
  const input = useRef<PhoneInput | null>(null)
  const rippleRef = useRef<HTMLDivElement>(null)
  const [zoom, setZoom] = useState(1)

  useEffect(() => {
    if (!stage) return
    const pi = createPhoneInput({
      sendInput: (msg, droppable) => client.sendInput(msg, droppable),
      epoch: () => client.currentEpoch(),
      now: () => performance.now(),
      setInterval: (fn, ms) => window.setInterval(fn, ms),
      clearInterval: (id) => window.clearInterval(id as number),
      win: window,
      doc: document,
    })
    input.current = pi

    let z: ZoomState = FIT
    let pinchBase: number | null = null // the zoom when the current pinch began
    let cursor: DesktopCursorMsg | null = null
    let inputOpen = false
    let dragDropped = false // a drag whose start had no cursor to press at: its moves go nowhere
    const waiters = new Set<{ run: (c: DesktopCursorMsg) => void; timer: number }>()
    let rippleTimer: number | null = null

    const rect1x = () =>
      video ? videoContentRect(video.clientWidth, video.clientHeight, video.videoWidth, video.videoHeight) : { x: 0, y: 0, width: 0, height: 0 }

    const applyZoom = (next: ZoomState) => {
      z = next
      if (video) {
        video.style.transformOrigin = next.scale > 1 ? '0 0' : ''
        video.style.transform = videoTransform(next)
      }
      setZoom(next.scale)
    }

    // The Mac's last report, only while it is for the surface the Mac shows now.
    const cursorNow = () => (cursor !== null && cursor.e === client.currentEpoch() ? cursor : null)
    // A zero move posts nothing visible; the Mac answers every relative move with where its pointer is.
    const requestCursor = () => pi.rmove(0, 0)

    function withCursor(run: (c: DesktopCursorMsg) => void) {
      const c = cursorNow()
      if (c) return run(c)
      requestCursor()
      const w = { run, timer: window.setTimeout(() => waiters.delete(w), CURSOR_WAIT_MS) }
      waiters.add(w)
    }

    function showRipple(c: DesktopCursorMsg) {
      const el = rippleRef.current
      if (!el) return
      const p = pointerToStage(c.x, c.y, rect1x())
      el.style.transform = `translate(${z.scale * p.x + z.tx - RIPPLE_PX / 2}px, ${z.scale * p.y + z.ty - RIPPLE_PX / 2}px)`
      el.hidden = false
      if (rippleTimer !== null) window.clearTimeout(rippleTimer)
      rippleTimer = window.setTimeout(() => {
        rippleTimer = null
        el.hidden = true
      }, RIPPLE_MS)
    }

    function endDrag(at: number) {
      if (dragDropped) {
        dragDropped = false
        return
      }
      if (pi.held() === 0) return
      const c = cursorNow()
      if (c) pi.release(c.x, c.y, 0, at)
      else pi.releaseAll()
    }

    const onIntent = (i: TrackpadIntent) => {
      // Zoom is local: it works for a viewer without the control right and while the Mac is locked.
      if (i.kind === 'pinch') {
        if (pinchBase === null) {
          pinchBase = z.scale
          if (video) video.style.willChange = 'transform'
        }
        const r = stage.getBoundingClientRect()
        applyZoom(zoomAround(z, pinchBase * i.scale, i.cx - r.left, i.cy - r.top, stage.clientWidth, stage.clientHeight))
        return
      }
      if (i.kind === 'pinchEnd') {
        pinchBase = null
        if (video) video.style.willChange = ''
        return
      }
      if (i.kind === 'resetZoom') {
        applyZoom(FIT)
        return
      }
      // A button held by a drag is always let go of, whatever the state has become since.
      if (i.kind === 'dragEnd') return endDrag(i.at)
      if (!live.current.canControl || live.current.paused) return
      switch (i.kind) {
        case 'move':
        case 'dragMove': {
          if (i.kind === 'dragMove' && dragDropped) return
          const r = rect1x()
          const d = normalizedDelta(i.dx, i.dy, z.scale, r.width, r.height)
          if (d.ndx !== 0 || d.ndy !== 0) pi.rmove(d.ndx, d.ndy)
          return
        }
        case 'tap':
          return withCursor((c) => pi.click(c.x, c.y, 0, i.at))
        case 'doubleTap':
          return withCursor((c) => pi.doubleClick(c.x, c.y, i.at))
        case 'longPress':
          return withCursor((c) => {
            pi.click(c.x, c.y, 2, i.at)
            showRipple(c)
          })
        case 'dragStart': {
          const c = cursorNow()
          dragDropped = c === null
          if (!c) return requestCursor()
          pi.press(c.x, c.y, 0, i.at)
          showRipple(c)
          return
        }
        case 'scroll': {
          const d = scrollDelta(i.dx, i.dy, z.scale)
          pi.wheel(d.dx, d.dy)
          return
        }
      }
    }
    const detach = attachTrackpad(stage, onIntent)

    // While zoomed the view follows the Mac's pointer so it stays 32 px inside the edge.
    const onCursor = (c: DesktopCursorMsg) => {
      cursor = c
      if (cursorNow() === null) return
      for (const w of [...waiters]) {
        window.clearTimeout(w.timer)
        waiters.delete(w)
        w.run(c)
      }
      if (z.scale > 1) {
        const p = pointerToStage(c.x, c.y, rect1x())
        const next = panToKeepPointer(z, p.x, p.y, stage.clientWidth, stage.clientHeight)
        if (next !== z) applyZoom(next)
      }
    }

    let lastEpoch: number | null = null
    const offs = [
      client.on('cursor', onCursor),
      client.on('input', ({ open }) => {
        inputOpen = open
        if (open) return requestCursor()
        // A closed channel strands whatever was held; the next report is for a new connection.
        cursor = null
        pi.releaseAll()
      }),
      client.on('surface', (sf) => {
        if (lastEpoch !== null && sf.e !== lastEpoch) {
          cursor = null
          if (pi.held() > 0) pi.releaseAll()
        }
        lastEpoch = sf.e
        if (inputOpen) requestCursor()
      }),
    ]

    // The coordinates change under a zoom when the phone turns, so it goes back to 1x.
    const portrait = () => window.innerHeight >= window.innerWidth
    let wasPortrait = portrait()
    const onResize = () => {
      const now = portrait()
      if (now !== wasPortrait && z.scale > 1) applyZoom(FIT)
      wasPortrait = now
    }
    window.addEventListener('resize', onResize)

    return () => {
      window.removeEventListener('resize', onResize)
      for (const off of offs) off()
      detach()
      for (const w of waiters) window.clearTimeout(w.timer)
      waiters.clear()
      if (rippleTimer !== null) window.clearTimeout(rippleTimer)
      pi.destroy()
      input.current = null
      if (video) {
        video.style.transform = ''
        video.style.transformOrigin = ''
        video.style.willChange = ''
      }
    }
  }, [client, stage, video])

  // The Mac locked or the connection dropped under a held button: let go of it.
  useEffect(() => {
    if (paused && (input.current?.held() ?? 0) > 0) input.current?.releaseAll()
  }, [paused])

  return <div ref={rippleRef} className="desktop-ripple" aria-hidden="true" hidden data-zoom={zoom} />
}
