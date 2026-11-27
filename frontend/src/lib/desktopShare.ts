import { useCallback, useEffect, useState } from 'react'
import { EventsOn } from '../wailsjs/wailsjs/runtime/runtime'

// The sharer's view of a click-to-share (Phase 22): the GUI's own Go side forwards
// the daemon's events stream as four Wails events (app_desktop_share.go). Names are
// already sanitized in the daemon; the GUI sanitizes again where it renders them.
// The link is deliberately not part of any of these payloads.

/** One connected viewer or waiting guest. Mode is "guest" (admitted) or "own" (own-account device). */
export interface ShareViewer {
  id: string
  mode: string
  displayName: string
  loginName: string
  nodeName: string
  control: boolean
  clipboard: boolean
  /** RFC 3339 time the connection began; the Watching list is oldest first. */
  since: string
}

/** A guest waiting at Admit (same fields; id is the request id). */
export type SharePending = ShareViewer

export interface ShareState {
  active: boolean
  viewers: ShareViewer[]
  pending: SharePending[]
  secureInput: boolean
  anyController: boolean
}

/** One event line (admit_request, join, leave, share_stop); own-account lines carry viewer.mode "own". */
export interface ShareEvent {
  event: string
  viewer?: ShareViewer
  reason?: string
}

export const NO_SHARE: ShareState = { active: false, viewers: [], pending: [], secureInput: false, anyController: false }

/** The Wails runtime's EventsOn; a test passes a fake. */
export type ShareSubscribe = typeof EventsOn

/** A subscribe that listens to nothing: web mode has no Wails runtime. */
export const NO_SHARE_EVENTS: ShareSubscribe = () => () => {}

export interface DesktopShareStore {
  state: ShareState
  /** The latest event line, for the live region and tests; null before any. */
  lastEvent: ShareEvent | null
  sheetOpen: boolean
  openSheet: () => void
  closeSheet: () => void
  /** The daemon or stream went away under an active share (reason "restart"); cleared by openSheet and closeSheet. */
  stoppedNotice: boolean
}

/**
 * Subscribes to desktopshare:state, :event, :stopped and :open-sheet for the life
 * of the caller (App) and keeps the latest of each.
 */
export function useDesktopShare(subscribe: ShareSubscribe = EventsOn): DesktopShareStore {
  const [state, setState] = useState<ShareState>(NO_SHARE)
  const [lastEvent, setLastEvent] = useState<ShareEvent | null>(null)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [stoppedNotice, setStoppedNotice] = useState(false)

  useEffect(() => {
    const offs = [
      subscribe('desktopshare:state', (s: Partial<ShareState> | null) => {
        // Go sends [] never null, but a hand-built payload must not crash the sheet.
        setState({ ...NO_SHARE, ...s, viewers: s?.viewers ?? [], pending: s?.pending ?? [] })
        // A new share started: the earlier "stopped" notice no longer applies.
        if (s?.active) setStoppedNotice(false)
      }),
      subscribe('desktopshare:event', (e: ShareEvent) => setLastEvent(e)),
      subscribe('desktopshare:stopped', () => {
        setState(NO_SHARE)
        setStoppedNotice(true)
      }),
      subscribe('desktopshare:open-sheet', () => {
        setStoppedNotice(false)
        setSheetOpen(true)
      }),
    ]
    return () => offs.forEach((off) => off())
  }, [subscribe])

  const openSheet = useCallback(() => {
    setStoppedNotice(false)
    setSheetOpen(true)
  }, [])
  const closeSheet = useCallback(() => {
    setStoppedNotice(false)
    setSheetOpen(false)
  }, [])

  return { state, lastEvent, sheetOpen, openSheet, closeSheet, stoppedNotice }
}

/** Drop Unicode categories Cc and Cf (bidi overrides, zero-width) and collapse whitespace: the daemon's rule, applied again. */
export const sanitizeName = (s: string): string => s.replace(/[\p{Cc}\p{Cf}]/gu, '').replace(/\s+/g, ' ').trim()

// A thrown Error stringifies as "Error: …" and the detail often ends in its own
// period; strip both so a fixed sentence reads "…: <detail>. Try again."
export const cleanDetail = (d: string): string => d.trim().replace(/^Error:\s*/, '').replace(/\.+$/, '')
