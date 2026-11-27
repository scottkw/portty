import {
  webSocketTransport,
  type CheckVerdict,
  type DesktopClientOptions,
  type DesktopClock,
  type SignalTransport,
} from './desktopClient'

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

const realClock: DesktopClock = {
  now: () => Date.now(),
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (id) => clearTimeout(id as ReturnType<typeof setTimeout>),
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
