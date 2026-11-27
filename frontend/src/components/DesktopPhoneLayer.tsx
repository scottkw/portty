import React, { useEffect, useRef } from 'react'
import type { DesktopClient } from '../lib/desktopClient'
import { normalizedDelta } from '../lib/desktopCoords'
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

/**
 * Phase 21-10 (DVWR-03) — the iPhone touch layer on the viewer's stage. Phase 20's controller is not
 * created on a phone, so this is the only thing that sends input from one. Renders nothing itself;
 * it listens on the stage (never window or document for touches) and sends Phase 20 messages.
 */
export function DesktopPhoneLayer({ client, video, stage, rights, locked, reconnecting }: DesktopPhoneLayerProps): React.ReactElement | null {
  const canControl = rights?.control === true
  const paused = locked || reconnecting
  const live = useRef({ canControl, paused })
  live.current = { canControl, paused }
  const input = useRef<PhoneInput | null>(null)

  useEffect(() => {
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
    return () => {
      input.current = null
    }
  }, [client])

  useEffect(() => {
    if (!stage) return
    const onIntent = (i: TrackpadIntent) => {
      if (!live.current.canControl || live.current.paused) return
      if (i.kind === 'move') {
        const r = video ? videoContentRect(video.clientWidth, video.clientHeight, video.videoWidth, video.videoHeight) : null
        if (!r) return
        const d = normalizedDelta(i.dx, i.dy, 1, r.width, r.height)
        if (d.ndx !== 0 || d.ndy !== 0) input.current?.rmove(d.ndx, d.ndy)
      }
    }
    return attachTrackpad(stage, onIntent)
  }, [stage, video])

  return null
}
