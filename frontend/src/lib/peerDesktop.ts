import {
  webSocketTransport,
  type CheckVerdict,
  type ClientStatus,
  type DesktopClientOptions,
  type DesktopClock,
  type SignalTransport,
} from './desktopClient'
import { shortHost } from './sidebarPeers'

/**
 * Phase 21-05: the Desktop tab's connection logic. Pure TypeScript: the daemon
 * binding is passed in as `connect`, so 21-08 hands it the ConnectPeerDesktop
 * binding and this module stays testable against the real Phase 19 client.
 *
 * The tab only ever handles the loopback URL the binding returns and an error
 * code. The desktop token stays in the daemon: no URL built here carries it.
 */

/** The shape of the ConnectPeerDesktop binding's result. */
export interface PeerDesktopConnectResult {
  host: string
  url: string
  errorCode: string
}

/** Close code the Local Network heal uses to make the client rebuild. */
export const LOCAL_NETWORK_CLOSE = 4000
/** ICE still new this long after the answer is applied: Local Network is the likely cause. */
export const LOCAL_NETWORK_MS = 10000

/** What the client's liveness check makes of a connect result's error code. */
export function connectVerdict(errorCode: string): CheckVerdict {
  switch (errorCode) {
    case '':
      return 'ok'
    case 'forbidden':
      return 'other_account'
    case 'desktop_off':
      return 'off'
    default:
      // unreachable, unknown_host, tailscale_off or a code from a newer daemon
      return 'retry'
  }
}

/**
 * Watches for a blocked Local Network. It arms when the answer is applied
 * (signalingState becomes stable), so a slow answer is not mistaken for a
 * blocked network. addEventListener only: the client owns the connection-state
 * property handler.
 */
export function watchLocalNetwork(
  pc: RTCPeerConnection,
  opts: { clock: DesktopClock; timeoutMs: number; onBlocked(): void; onClear(): void },
): () => void {
  let timer: unknown = null
  let armed = false
  let sawChecking = false
  let cleared = false
  let blocked = false
  const stopTimer = () => {
    if (timer !== null) opts.clock.clearTimeout(timer)
    timer = null
  }
  const block = () => {
    if (blocked) return
    blocked = true
    stopTimer()
    opts.onBlocked()
  }
  const onSignaling = () => {
    if (armed || pc.signalingState !== 'stable') return
    armed = true
    timer = opts.clock.setTimeout(() => {
      timer = null
      if (pc.signalingState !== 'closed' && pc.iceConnectionState === 'new') block()
    }, opts.timeoutMs)
  }
  const onIce = () => {
    if (blocked) return
    const s = pc.iceConnectionState
    if (s === 'checking' || s === 'connected' || s === 'completed') {
      sawChecking = true
      stopTimer()
      if (!cleared) {
        cleared = true
        opts.onClear()
      }
    } else if (s === 'failed' && !sawChecking) {
      block()
    }
  }
  pc.addEventListener('signalingstatechange', onSignaling)
  pc.addEventListener('iceconnectionstatechange', onIce)
  return () => {
    stopTimer()
    pc.removeEventListener('signalingstatechange', onSignaling)
    pc.removeEventListener('iceconnectionstatechange', onIce)
  }
}

export interface PeerDesktopDeps {
  host: string
  connect(host: string): Promise<PeerDesktopConnectResult>
  onLocalNetwork(blocked: boolean): void
  clock?: DesktopClock
  makeTransport?(url: string): SignalTransport
  makePC?(): RTCPeerConnection
  localNetworkMs?: number
}

export const realClock: DesktopClock = {
  now: () => Date.now(),
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (id) => clearTimeout(id as ReturnType<typeof setTimeout>),
}

/** A hidden window this long lets the office Mac's display sleep (DVWR-01). */
export const HIDDEN_PAUSE_MS = 120000

/**
 * Calls onPause once the document has been hidden for delayMs, and onResume on
 * the next visible. Returns a disposer that removes the listener and the timer.
 */
export function createHiddenPause(opts: {
  doc: Document
  clock: DesktopClock
  delayMs?: number
  onPause(): void
  onResume(): void
}): () => void {
  let timer: unknown = null
  let paused = false
  const clear = () => {
    if (timer !== null) opts.clock.clearTimeout(timer)
    timer = null
  }
  const onChange = () => {
    if (opts.doc.visibilityState === 'hidden') {
      if (timer !== null || paused) return
      timer = opts.clock.setTimeout(() => {
        timer = null
        paused = true
        opts.onPause()
      }, opts.delayMs ?? HIDDEN_PAUSE_MS)
      return
    }
    clear()
    if (paused) {
      paused = false
      opts.onResume()
    }
  }
  opts.doc.addEventListener('visibilitychange', onChange)
  onChange()
  return () => {
    clear()
    opts.doc.removeEventListener('visibilitychange', onChange)
  }
}

export type DesktopTabLabelInput = {
  host: string
  status: ClientStatus['kind']
  hasPicture: boolean
  captured: boolean
  locked: boolean
  mode?: 'view' | 'control'
  controlAllowed: boolean
}

/** The app's release chord (CONTEXT, after the 21-02 probe): Ctrl+Option+Cmd+Return is taken by the system in the Portty app. */
export const APP_RELEASE_CHORD = 'Ctrl+Option+Cmd+Shift+Return'

/** The toolbar label's two lines: who owns the keyboard (UI-SPEC toolbar label table). */
export function desktopTabLabel(i: DesktopTabLabelInput): { line1: string; line2: string; controlling: boolean } {
  const host = shortHost(i.host)
  const connected = i.status === 'streaming' && i.hasPicture
  // Control mode is off whenever the stream is not connected (UI-SPEC), so the
  // captured flag only counts while connected.
  if (connected && i.captured) {
    return { line1: `Controlling ${host}`, line2: `Release: ${APP_RELEASE_CHORD}`, controlling: true }
  }
  let line2: string
  if (!connected) line2 = 'Not connected'
  else if (i.locked) line2 = 'Locked'
  else if (i.mode === 'view' || !i.controlAllowed) line2 = 'View only'
  else line2 = 'Click the picture to control'
  return { line1: `On ${host}`, line2, controlling: false }
}

export function LABEL_TOOLTIP(host: string): string {
  return `While you control ${shortHost(host)}, every key goes to that Mac, including Cmd+Q, Cmd+W and Cmd+C. Press ${APP_RELEASE_CHORD} to get Portty's shortcuts back.`
}

/** UI-SPEC Copywriting Contract, word for word. */
export const DESKTOP_TAB_CARDS: Record<
  'connecting' | 'unreachable' | 'desktop_off' | 'token_rejected' | 'tap',
  { heading: string; body: string; action?: string }
> = {
  connecting: {
    heading: 'Connecting…',
    body: 'Waiting for the first picture from the other Mac. If macOS asks to find devices on local networks, choose Allow.',
  },
  unreachable: {
    heading: "Can't reach this Mac",
    body: "Check that it's awake and on your tailnet. Portty keeps trying.",
    action: 'Reconnect',
  },
  desktop_off: {
    heading: 'Desktop sharing is off on this Mac',
    body: "Turn on Always-available desktop in Portty's Settings, under Sharing, on that Mac.",
  },
  token_rejected: {
    heading: "Can't open this desktop",
    body: 'That Mac refused the connection. Check that both Macs are signed in to the same Tailscale account, then try again.',
    action: 'Reconnect',
  },
  tap: {
    heading: 'Click to start',
    body: 'Portty needs a click to start the picture.',
    action: 'Click to Start',
  },
}

export const LOCAL_NETWORK_CARD = {
  heading: 'Portty needs Local Network access to show this desktop',
  body: 'Open System Settings, go to Privacy & Security, then Local Network, and turn on Portty. Or open the desktop in your browser.',
  openSettings: 'Open System Settings',
  openBrowser: 'Open in Browser',
}

export function createPeerDesktopOptions(deps: PeerDesktopDeps): {
  options: DesktopClientOptions
  forceRebuild(): void
  dispose(): void
} {
  const clock = deps.clock ?? realClock
  const open = deps.makeTransport ?? webSocketTransport

  let disposed = false
  let url = ''
  let initialPending = true
  // Phase 19's client never runs beforeRedial before its first connect, so the
  // first transport waits here. A rejected connect leaves the url empty and the
  // transport reports a close, which sends the client to retry.
  const initial = deps.connect(deps.host).then(
    (r) => r.url,
    () => '',
  )
  void initial.then((u) => {
    url = u
    initialPending = false
  })

  let current: SignalTransport | null = null

  /** A transport that opens its WebSocket once the url is known. */
  function lazyTransport(urlOf: Promise<string> | string): SignalTransport {
    let real: SignalTransport | null = null
    let closed = false
    const queue: object[] = []
    let openCb: () => void = () => {}
    let msgCb: (m: unknown) => void = () => {}
    let closeCb: (c: number) => void = () => {}

    const bind = (u: string) => {
      if (closed || disposed) return
      if (!u) {
        closeCb(1006)
        return
      }
      const r = open(u)
      real = r
      current = r
      r.onOpen(() => {
        openCb()
        for (const m of queue.splice(0)) r.send(m)
      })
      r.onMessage((m) => msgCb(m))
      r.onClose((c) => closeCb(c))
    }
    // Resolve on a microtask even for a known url, so the client has registered
    // its callbacks before an empty url reports a close.
    void Promise.resolve(urlOf).then(bind)

    return {
      send: (m) => (real ? real.send(m) : void queue.push(m)),
      close: (code) => {
        closed = true
        real?.close(code)
      },
      onOpen: (cb) => { openCb = cb },
      onMessage: (cb) => { msgCb = cb },
      onClose: (cb) => { closeCb = cb },
    }
  }

  const makePC = deps.makePC ?? (() => new RTCPeerConnection({ iceServers: [] }))
  let stopWatch: () => void = () => {}

  function forceRebuild() {
    current?.close(LOCAL_NETWORK_CLOSE)
  }

  return {
    options: {
      clock,
      transport: () => lazyTransport(initialPending ? initial : url),
      // A home daemon restart empties its in-memory key store and may move the
      // relay port, so every redial asks for a fresh url first. A throw makes the
      // client count a failed attempt; this hook never ends the loop itself.
      beforeRedial: async () => {
        const r = await deps.connect(deps.host)
        // An empty url (unreachable) keeps the last good one: the proxy's own
        // answer then drives the client's classification.
        if (r.url) url = r.url
      },
      check: async () => {
        try {
          return connectVerdict((await deps.connect(deps.host)).errorCode)
        } catch {
          return 'retry'
        }
      },
      createPeerConnection: () => {
        stopWatch()
        const pc = makePC()
        stopWatch = watchLocalNetwork(pc, {
          clock,
          timeoutMs: deps.localNetworkMs ?? LOCAL_NETWORK_MS,
          onBlocked: () => {
            deps.onLocalNetwork(true)
            forceRebuild()
          },
          onClear: () => deps.onLocalNetwork(false),
        })
        return pc
      },
    },
    forceRebuild,
    dispose() {
      disposed = true
      stopWatch()
      current?.close()
    },
  }
}
