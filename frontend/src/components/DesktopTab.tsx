import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { ConnectPeerDesktop, OpenLocalNetworkSettings, OpenPeerDesktopInBrowser, ReadHomeClipboard } from '../wailsjs/go/main/App'
import * as pd from '../lib/peerDesktop'
import { controlReducer, isPasteChord, pasteDecision, PASTE_NOTICES, type ControlEvent, type HomeClipboard } from '../lib/controlMode'
import type { ClientStatus, DesktopClient, DesktopStateMsg } from '../lib/desktopClient'
import type { DesktopRowState } from '../lib/sidebarPeers'
import { DesktopView, type DesktopControlHandle } from './DesktopView'
import type { Tab } from './TabBar'

/** The window the system reports covered or hidden: App bridges the native occlusion event to this. */
const WINDOW_HIDDEN_EVENT = 'portty:window-hidden'
/** After one of these the client has ended its own loop, so Reconnect starts it again instead of waking it. */
const FINAL_STATUS: ReadonlySet<ClientStatus['kind']> = new Set(['token_rejected', 'desktop_off', 'other_account'])
const MUTED_STATES: ReadonlySet<string> = new Set(['needs_permission', 'awaiting_approval', 'display_asleep', 'capture_paused'])
/** 21-02's KEYROUTE verdict is page: the stage's preventDefault keeps every chord off Portty's menu, so no native menu switch exists. */
const NATIVE_MENU = false

/** The only home-clipboard read in the app (the types-first Go bridge): a rejected call fails closed. */
async function readHome(): Promise<HomeClipboard> {
  try {
    const r = await ReadHomeClipboard()
    return { outcome: r.outcome as HomeClipboard['outcome'], text: r.text }
  } catch {
    return { outcome: 'failed' }
  }
}

/** The sidebar's muted line for the Mac's last state; null for ready, streaming and the rest. */
function rowState(s: DesktopStateMsg | null): DesktopRowState | null {
  if (!s) return null
  if (s.locked || s.state === 'locked') return 'locked'
  return MUTED_STATES.has(s.state) ? (s.state as DesktopRowState) : null
}

/** The tab bar's two-line label (UI-SPEC toolbar label). */
export interface DesktopLabel {
  line1: string
  line2: string
  controlling: boolean
  tooltip: string
}

export interface DesktopTabProps {
  tab: Tab
  active: boolean
  /** The tab's last state for its sidebar row, keyed by the connect host; null clears it. */
  onDesktopState(connectHost: string, s: DesktopRowState | null): void
  /** The toolbar label while this tab is active; null otherwise (Task 3). */
  onLabel?(l: DesktopLabel | null): void
}

/**
 * Phase 21-08: the Portty app's Desktop tab. It mounts the shared DesktopView (the one the /desktop/
 * browser page uses) over the loopback connection 21-05 builds from the ConnectPeerDesktop binding.
 * The tab only ever knows the host, the loopback URL and error codes: the desktop token stays in the daemon.
 */
export function DesktopTab({ tab, active, onDesktopState, onLabel }: DesktopTabProps): React.ReactElement {
  const connectHost = tab.connectHost ?? tab.peerHost ?? ''
  const [localNetwork, setLocalNetwork] = useState(false)
  // The client was stopped because the window stayed hidden; the Reconnecting chip shows until a picture or status returns.
  const [paused, setPaused] = useState(false)
  const clientRef = useRef<DesktopClient | null>(null)
  const offTrack = useRef<() => void>(() => {})
  const statusRef = useRef<ClientStatus['kind']>('connecting')
  const stateCb = useRef(onDesktopState)
  stateCb.current = onDesktopState
  // What the toolbar label is made from; each piece is reported by the viewer.
  const [view, setView] = useState<Omit<pd.DesktopTabLabelInput, 'host'>>({
    status: 'connecting',
    hasPicture: false,
    captured: false,
    locked: false,
    mode: 'control',
    controlAllowed: false,
  })
  const patch = useCallback((p: Partial<typeof view>) => setView((v) => ({ ...v, ...p })), [])

  // Control mode (21-09) is the viewer's "keyboard captured" state; the reducer says what each event releases.
  const handleRef = useRef<DesktopControlHandle | null>(null)
  const setHandle = useCallback((h: DesktopControlHandle | null) => {
    handleRef.current = h
  }, [])
  const controlling = useRef(false)
  const control = useCallback((ev: ControlEvent) => {
    const next = controlReducer(controlling.current, ev, NATIVE_MENU)
    controlling.current = next.controlling
    for (const a of next.actions) {
      if (a === 'releaseAll') handleRef.current?.releaseAll()
      else if (a === 'releaseKeyboard') handleRef.current?.releaseKeyboard()
    }
  }, [])
  // Another tab came forward: the keyboard goes back to Portty.
  useEffect(() => {
    if (!active) control('tab-inactive')
  }, [active, control])
  // Portty stopped being the active app, or the window was hidden. The stage input is blurred, so coming
  // back does not turn control mode on again; one click does.
  useEffect(() => {
    const away = () => control('window-blur')
    const hidden = () => { if (document.visibilityState === 'hidden') away() }
    window.addEventListener('blur', away)
    document.addEventListener('visibilitychange', hidden)
    return () => {
      window.removeEventListener('blur', away)
      document.removeEventListener('visibilitychange', hidden)
    }
  }, [control])
  // A layout cleanup, so the viewer's handle still exists to send the release before the client is torn down.
  useLayoutEffect(() => () => control('tab-close'), [control])

  // Cmd+V in control mode (CONTEXT, accepted): the home clipboard's text goes to the Mac with Phase 20's
  // paste message, then the key. This is the only place the home clipboard is read. The listener sits on
  // this tab's own wrapper in the capture phase, so it runs before the stage's key listener and never
  // touches window or document.
  const wrapRef = useRef<HTMLDivElement>(null)
  const lastSent = useRef<string | null>(null) // memory only, gone with the tab
  const pasting = useRef(false)
  const replaying = useRef(false)
  useEffect(() => {
    const wrap = wrapRef.current
    if (!wrap) return
    const notice = (text: string) => handleRef.current?.notify(text, text === PASTE_NOTICES.failed ? 'alert' : 'status', 5000)
    async function paste(e: KeyboardEvent) {
      const target = e.target as HTMLElement
      const replay = {
        code: e.code, key: e.key, metaKey: true, modifierCapsLock: e.getModifierState('CapsLock'), bubbles: true, cancelable: true,
      }
      const d = pasteDecision(await readHome(), lastSent.current)
      if (d.notice) return notice(d.notice)
      if (!controlling.current) return // control ended while the read was in flight: nothing goes to the Mac
      if (d.send !== null) {
        let outcome: string | undefined
        try {
          outcome = (await clientRef.current?.pasteToMac(d.send))?.outcome
        } catch {
          // Reported below as a failed send.
        }
        if (outcome !== 'ok') return notice(outcome === 'too_large' ? PASTE_NOTICES.too_large : PASTE_NOTICES.failed)
        lastSent.current = d.send
      }
      // The user may have left control mode while this was in flight: then the key stays home.
      if (!d.key || !controlling.current) return
      // Phase 20's own handler sends the key (its Cmd-chord tap, the Swap setting, the modifier mask).
      replaying.current = true
      try {
        target.dispatchEvent(new KeyboardEvent('keydown', replay))
      } finally {
        replaying.current = false
      }
    }
    const onKey = (e: KeyboardEvent) => {
      if (replaying.current || !controlling.current || !isPasteChord(e)) return
      // Stopped even when ignored, so a Cmd+V never reaches the Mac as a plain key behind the paste.
      e.preventDefault()
      e.stopPropagation()
      if (e.repeat || pasting.current) return
      pasting.current = true
      void paste(e).finally(() => { pasting.current = false })
    }
    wrap.addEventListener('keydown', onKey, { capture: true })
    return () => wrap.removeEventListener('keydown', onKey, { capture: true })
  }, [])
  const labelCb = useRef(onLabel)
  labelCb.current = onLabel
  // Closing the tab takes its sidebar line and toolbar label with it; the peer may keep announcing its desktop.
  useEffect(() => () => {
    stateCb.current(connectHost, null)
    labelCb.current?.(null)
  }, [connectHost])
  const label = pd.desktopTabLabel({ host: connectHost, ...view })
  useEffect(() => {
    // Only the active tab owns the toolbar label (like the On <host> chip).
    labelCb.current?.(active ? { ...label, tooltip: pd.LABEL_TOOLTIP(connectHost) } : null)
  }, [active, connectHost, label.line1, label.line2, label.controlling])

  // Built once per mount: the options own the first connect, so a re-render must not make another.
  const built = useRef<ReturnType<typeof pd.createPeerDesktopOptions> | null>(null)
  if (built.current === null) {
    built.current = pd.createPeerDesktopOptions({ host: connectHost, connect: ConnectPeerDesktop, onLocalNetwork: setLocalNetwork })
  }
  const peer = built.current

  // StrictMode (dev) runs this effect's cleanup and then the effect again on the same instance; dispose
  // is final, so it waits a tick and the second run cancels it.
  const disposeTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (disposeTimer.current !== null) clearTimeout(disposeTimer.current)
    disposeTimer.current = null
    return () => {
      disposeTimer.current = setTimeout(() => peer.dispose(), 0)
    }
  }, [peer])

  // A fresh loopback url before the client dials again: a restarted daemon may have moved the relay port.
  // A failed connect is the client's own business (it classifies and retries), so it is not surfaced here.
  const refreshUrl = useCallback(async () => {
    try {
      await peer.options.beforeRedial?.()
    } catch {
      // The client's next attempt reports the failure.
    }
  }, [peer])

  const reconnect = useCallback(async () => {
    await refreshUrl()
    const c = clientRef.current
    if (!c) return
    if (FINAL_STATUS.has(statusRef.current)) c.start()
    else c.wake('external')
  }, [refreshUrl])

  // The Wails runtime's wake event (system wake, network change), re-dispatched on window by App.
  useEffect(() => {
    const onWake = () => clientRef.current?.wake('external')
    window.addEventListener('portty:wake', onWake)
    return () => window.removeEventListener('portty:wake', onWake)
  }, [])

  // A window hidden about 2 minutes lets the office display sleep (T-21-26): stop the client, which makes the
  // office drop this viewer, and start it again when the window is shown. The document says hidden for a
  // closed, minimized or hidden window; the native occlusion event covers a window that is only covered.
  useEffect(() => {
    let occluded = false
    // Registered before the pause's own listener, so it has already updated when the pause reads it.
    const track = (e: Event) => { occluded = (e as CustomEvent<boolean>).detail === true }
    window.addEventListener(WINDOW_HIDDEN_EVENT, track)
    const doc = {
      get visibilityState() { return document.visibilityState === 'hidden' || occluded ? 'hidden' : 'visible' },
      addEventListener: (_t: string, l: EventListener) => {
        document.addEventListener('visibilitychange', l)
        window.addEventListener(WINDOW_HIDDEN_EVENT, l)
      },
      removeEventListener: (_t: string, l: EventListener) => {
        document.removeEventListener('visibilitychange', l)
        window.removeEventListener(WINDOW_HIDDEN_EVENT, l)
      },
    } as unknown as Document
    const off = pd.createHiddenPause({
      doc,
      clock: pd.realClock,
      onPause: () => {
        control('not-connected')
        clientRef.current?.stop()
        setPaused(true)
      },
      onResume: () => {
        void refreshUrl().then(() => clientRef.current?.start())
      },
    })
    return () => {
      off()
      window.removeEventListener(WINDOW_HIDDEN_EVENT, track)
    }
  }, [refreshUrl, control])

  const onClient = useCallback((c: DesktopClient | null) => {
    clientRef.current = c
    offTrack.current()
    offTrack.current = c ? c.on('track', () => setPaused(false)) : () => {}
  }, [])

  return (
    // display: contents keeps the wrapper out of layout; it exists only to carry the paste listener.
    <div ref={wrapRef} style={{ display: 'contents' }}>
      <DesktopView
        embedded
        clientOptions={peer.options}
        cards={pd.DESKTOP_TAB_CARDS}
        hostCard={localNetwork ? {
          heading: pd.LOCAL_NETWORK_CARD.heading,
          body: pd.LOCAL_NETWORK_CARD.body,
          actions: [
            { label: pd.LOCAL_NETWORK_CARD.openSettings, onClick: () => void OpenLocalNetworkSettings().catch(() => {}) },
            { label: pd.LOCAL_NETWORK_CARD.openBrowser, onClick: () => void OpenPeerDesktopInBrowser(connectHost).catch(() => {}) },
          ],
        } : null}
        readClipboard={readHome}
        paused={paused}
        onReload={() => void reconnect()}
        onClient={onClient}
        onStatus={(s) => {
          statusRef.current = s.kind
          setPaused(false)
          patch({ status: s.kind })
          // Not connected: Portty's shortcuts return at once, so Cmd+W closes a dead tab.
          if (s.kind !== 'streaming') control('not-connected')
        }}
        onDesktopState={(s) => {
          stateCb.current(connectHost, rowState(s))
          patch({ locked: s?.locked === true || s?.state === 'locked' })
        }}
        onPicture={(hasPicture) => patch({ hasPicture })}
        onCaptureChange={(captured) => {
          control(captured ? 'capture-on' : 'capture-off')
          patch({ captured })
        }}
        onModeChange={(mode) => patch({ mode })}
        onControlChange={({ available }) => patch({ controlAllowed: available })}
        controlRef={setHandle}
        hideCaptureNotices
        releaseChord={pd.APP_RELEASE_CHORD}
      />
    </div>
  )
}
