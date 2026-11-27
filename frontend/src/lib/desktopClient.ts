import { readWebModeParams } from './webMode'
import { MOVE_BUFFER_LIMIT } from './desktopInput'

/**
 * Phase 19-06 — the /desktop/ viewer's connection. One signalling WebSocket
 * (protocol v1, 19-01) plus one RTCPeerConnection that receives the Mac's
 * screen as a video track. Phase 20-06 adds the ordered `input` data channel
 * (offered only when the server hello carries caps input 1), the rights the
 * hello grants and the surface epoch that input must carry. The client never
 * listens to keys or pointers: desktopInput.ts does, over the stage.
 *
 * Pure TypeScript with no React and no Wails imports, so the Phase 21 Wails
 * Desktop tab can reuse it: the transport, the liveness check, the pre-redial
 * hook, the peer-connection factory and the clock are all injectable.
 */

export type DesktopState =
  | 'off'
  | 'needs_permission'
  | 'awaiting_approval'
  | 'display_asleep'
  | 'locked'
  | 'idle'
  | 'streaming'
  | 'capture_paused'

export type WakeReason = 'visibility' | 'pageshow' | 'online' | 'server-restart' | 'external'

export type ClientStatus =
  | { kind: 'connecting' }
  | { kind: 'streaming' }
  | { kind: 'reconnecting' }
  | { kind: 'unreachable' }
  | { kind: 'token_rejected' }
  | { kind: 'desktop_off' }
  | { kind: 'other_account' }

export interface SignalTransport {
  send(msg: object): void
  close(code?: number): void
  onOpen(cb: () => void): void
  onMessage(cb: (msg: unknown) => void): void
  onClose(cb: (code: number) => void): void
}

export type TransportFactory = () => SignalTransport
export type CheckVerdict = 'ok' | 'token' | 'off' | 'other_account' | 'retry'

export interface DesktopClock {
  now(): number
  setTimeout(fn: () => void, ms: number): unknown
  clearTimeout(id: unknown): void
}

export interface DesktopClientOptions {
  transport?: TransportFactory
  check?: () => Promise<CheckVerdict>
  /** Awaited before every redial (never before the first connect); 'stop' ends the loop. */
  beforeRedial?: () => Promise<void | 'stop'>
  createPeerConnection?: () => RTCPeerConnection
  clock?: DesktopClock
  doc?: Document
  win?: Window
}

export interface DesktopStateMsg {
  state: DesktopState
  viewers: number
  locked: boolean
}

export interface DesktopSizeMsg {
  w: number
  h: number
}

/** What the server hello says this viewer may do (20-01 rights map; a missing field is false). */
export interface DesktopRights {
  control: boolean
  clipboard: boolean
  surface: boolean
}

export type SurfaceKind = 'display' | 'window'
export type SurfaceReason = '' | 'window_closed' | 'display_gone'

/** The shared surface and its epoch (20-03). Input must carry `e` or the daemon drops it. */
export interface DesktopSurfaceMsg {
  e: number
  kind: SurfaceKind
  id: number
  name: string
  w: number
  h: number
  reason: SurfaceReason
}

export type ControlInput = 'ok' | 'needs_accessibility'
export type ControlWindow = '' | 'raise_failed' | 'minimized'

/** The daemon's control state, pushed on the input channel (20-02, 20-04). */
export interface DesktopControlMsg {
  input: ControlInput
  window: ControlWindow
}

/** Where the Mac's pointer is after a relative move (20-02); normalised like input. */
export interface DesktopCursorMsg {
  x: number
  y: number
  e: number
}

export interface DesktopEvents {
  track: MediaStream
  state: DesktopStateMsg
  size: DesktopSizeMsg
  status: ClientStatus
  wake: WakeReason
  rights: DesktopRights
  /** The input data channel opened or closed. */
  input: { open: boolean }
  surface: DesktopSurfaceMsg
  control: DesktopControlMsg
  cursor: DesktopCursorMsg
}

export interface DesktopClient {
  start(): void
  stop(): void
  /** Lets a host shell inject its own wake (Phase 21 calls wake('external') from its tab). */
  wake(reason: WakeReason): void
  /** A signalling message (select, clipboard); input goes through sendInput. */
  send(msg: object): void
  /**
   * One message on the input channel. False when the channel is not open, or, for a droppable
   * message (a move), when more than MOVE_BUFFER_LIMIT bytes are already queued. Presses,
   * releases and wheel are never droppable.
   */
  sendInput(msg: object, droppable?: boolean): boolean
  /** The surface epoch last announced by the server, or null until the first surface message. */
  currentEpoch(): number | null
  on<K extends keyof DesktopEvents>(event: K, cb: (v: DesktopEvents[K]) => void): () => void
}

export const RECONNECT = {
  delaysMs: [1000, 2000, 4000, 8000, 10000],
  unreachableAfterMs: 30000,
  pingTimeoutMs: 3000,
  helloTimeoutMs: 10000,
  gatherTimeoutMs: 3000,
  iceDisconnectedMs: 2000,
}

/** wss://<same host>/desktop/ws?cap=<token from the page URL>. */
export function defaultDesktopWsUrl(loc: Location = window.location): string {
  const { capToken } = readWebModeParams(loc)
  return `wss://${loc.host}/desktop/ws?cap=${encodeURIComponent(capToken ?? '')}`
}

export function webSocketTransport(url: string): SignalTransport {
  const ws = new WebSocket(url)
  return {
    send: (msg) => ws.send(JSON.stringify(msg)),
    close: (code) => ws.close(code ?? 1000),
    onOpen: (cb) => ws.addEventListener('open', () => cb()),
    onMessage: (cb) =>
      ws.addEventListener('message', (e) => {
        if (typeof e.data !== 'string') return
        try {
          cb(JSON.parse(e.data))
        } catch {
          // Not JSON: protocol v1 is JSON text frames only. Drop it.
        }
      }),
    onClose: (cb) => ws.addEventListener('close', (e) => cb(e.code)),
  }
}

const STATES: ReadonlySet<string> = new Set<DesktopState>([
  'off', 'needs_permission', 'awaiting_approval', 'display_asleep', 'locked', 'idle', 'streaming', 'capture_paused',
])

const realClock: DesktopClock = {
  now: () => Date.now(),
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (id) => clearTimeout(id as ReturnType<typeof setTimeout>),
}

export function createDesktopClient(opts: DesktopClientOptions = {}): DesktopClient {
  const win = opts.win ?? window
  const doc = opts.doc ?? document
  const clock = opts.clock ?? realClock
  const makeTransport = opts.transport ?? (() => webSocketTransport(defaultDesktopWsUrl(win.location)))
  const makePC = opts.createPeerConnection ?? (() => new RTCPeerConnection({ iceServers: [] }))
  const check = opts.check ?? (() => fetchCheck(win.location))

  const listeners: { [K in keyof DesktopEvents]: Set<(v: DesktopEvents[K]) => void> } = {
    track: new Set(), state: new Set(), size: new Set(), status: new Set(), wake: new Set(),
    rights: new Set(), input: new Set(), surface: new Set(), control: new Set(), cursor: new Set(),
  }
  function emit<K extends keyof DesktopEvents>(event: K, v: DesktopEvents[K]) {
    for (const cb of [...listeners[event]]) cb(v)
  }

  let status: ClientStatus = { kind: 'connecting' }
  function setStatus(s: ClientStatus) {
    if (s.kind === status.kind) return
    status = s
    emit('status', s)
  }

  let running = false
  // Every connection attempt takes the next generation; callbacks from a
  // superseded socket or peer connection compare theirs and drop out
  // (relayClient's stale-socket rule).
  let gen = 0
  let transport: SignalTransport | null = null
  let pc: RTCPeerConnection | null = null
  let hadHello = false
  let hasPicture = false
  // Per peer connection: a picture needs both the track and ICE connected. ontrack fires
  // on setRemoteDescription, long before (or without) any media path.
  let gotTrack = false
  let iceUp = false
  let inputChannel: RTCDataChannel | null = null
  let epoch: number | null = null
  let attempt = 0
  let pingN = 0
  let pendingPing: number | null = null
  let redialInFlight = false
  let redialTimer: unknown = null
  let helloTimer: unknown = null
  let iceTimer: unknown = null
  let pingTimer: unknown = null
  let unreachableTimer: unknown = null
  let debugTimer: unknown = null
  let lastViewers = 0
  let lastState = ''

  const cancel = (id: unknown): null => {
    if (id !== null) clock.clearTimeout(id)
    return null
  }

  /** Close the current socket and peer connection and their timers. Never redials. */
  function teardown() {
    helloTimer = cancel(helloTimer)
    iceTimer = cancel(iceTimer)
    pingTimer = cancel(pingTimer)
    pendingPing = null
    gotTrack = false
    iceUp = false
    epoch = null
    // The viewer must release what it holds when its channel goes, so say so now: a closed
    // peer connection reports its channel only after the generation has moved on.
    if (inputChannel) {
      inputChannel = null
      emit('input', { open: false })
    }
    const t = transport
    const p = pc
    transport = null
    pc = null
    try { t?.close() } catch { /* already closed */ }
    try { p?.close() } catch { /* already closed */ }
  }

  function armUnreachable() {
    if (unreachableTimer !== null || status.kind === 'unreachable') return
    unreachableTimer = clock.setTimeout(() => {
      unreachableTimer = null
      if (running && !hasPicture) setStatus({ kind: 'unreachable' })
    }, RECONNECT.unreachableAfterMs)
  }

  /** The picture is gone (or never came): Reconnecting, and the 30 s clock to unreachable runs. */
  function markLost() {
    hasPicture = false
    armUnreachable()
    if (status.kind !== 'unreachable') setStatus({ kind: 'reconnecting' })
  }

  function retry() {
    if (!running) return
    markLost()
    const delay = RECONNECT.delaysMs[Math.min(attempt, RECONNECT.delaysMs.length - 1)]
    attempt++
    redialTimer = cancel(redialTimer)
    redialTimer = clock.setTimeout(() => {
      redialTimer = null
      void redial()
    }, delay)
  }

  async function redial() {
    if (!running || redialInFlight) return
    redialInFlight = true
    try {
      if (opts.beforeRedial) {
        let verdict: void | 'stop'
        try {
          verdict = await opts.beforeRedial()
        } catch {
          // A re-auth that could not answer is a failed attempt, not the end.
          if (running) retry()
          return
        }
        if (!running) return
        if (verdict === 'stop') {
          shutdown()
          return
        }
      }
      connect()
    } finally {
      redialInFlight = false
    }
  }

  function redialNow() {
    redialTimer = cancel(redialTimer)
    void redial()
  }

  /** A rebuild from scratch: socket and peer connection both new. */
  function failConnection() {
    teardown()
    retry()
  }

  /** The end of the loop on a fixed code: no retry, no wake listeners. */
  function final(kind: 'token_rejected' | 'desktop_off' | 'other_account') {
    shutdown()
    setStatus({ kind })
  }

  function connect() {
    teardown()
    const g = ++gen
    hadHello = false
    let t: SignalTransport
    try {
      t = makeTransport()
    } catch {
      retry()
      return
    }
    transport = t
    t.onOpen(() => {
      if (g === gen) t.send({ type: 'hello', v: 1, caps: { video: 1, input: 1 } })
    })
    t.onMessage((m) => {
      if (g === gen) void handle(g, m)
    })
    t.onClose((code) => {
      if (g === gen) onClose(code)
    })
    helloTimer = clock.setTimeout(() => {
      helloTimer = null
      if (g === gen && !hadHello) failConnection()
    }, RECONNECT.helloTimeoutMs)
  }

  function onClose(code: number) {
    const hello = hadHello
    teardown()
    if (code === 4401) return final('token_rejected')
    if (code === 4410) return final('desktop_off')
    if (code === 1012) {
      emit('wake', 'server-restart')
      attempt = 0
      markLost()
      redialNow()
      return
    }
    if (hello) return retry()
    void classify()
  }

  /** A failed handshake hides its HTTP status from the page, so ask the check endpoint. */
  async function classify() {
    const g = gen
    let verdict: CheckVerdict
    try {
      verdict = await check()
    } catch {
      verdict = 'retry'
    }
    if (g !== gen || !running) return
    if (verdict === 'token') return final('token_rejected')
    if (verdict === 'off') return final('desktop_off')
    if (verdict === 'other_account') return final('other_account')
    retry()
  }

  async function gathered(conn: RTCPeerConnection) {
    if (conn.iceGatheringState === 'complete') return
    await new Promise<void>((resolve) => {
      const id = clock.setTimeout(resolve, RECONNECT.gatherTimeoutMs)
      conn.onicegatheringstatechange = () => {
        if (conn.iceGatheringState !== 'complete') return
        clock.clearTimeout(id)
        resolve()
      }
    })
  }

  function onTrack(e: RTCTrackEvent) {
    gotTrack = true
    emit('track', e.streams[0] ?? new MediaStream([e.track]))
    maybePicture()
  }

  /** The picture is here only once the track is in and ICE has connected (never on ontrack alone). */
  function maybePicture() {
    if (!gotTrack || !iceUp) return
    hasPicture = true
    attempt = 0
    unreachableTimer = cancel(unreachableTimer)
    setStatus({ kind: 'streaming' })
  }

  function onIceState(g: number, s: RTCPeerConnectionState) {
    if (g !== gen) return
    if (s === 'connected') {
      iceTimer = cancel(iceTimer)
      iceUp = true
      maybePicture()
    } else if (s === 'failed') failConnection()
    else if (s === 'disconnected' && iceTimer === null) {
      iceTimer = clock.setTimeout(() => {
        iceTimer = null
        if (g === gen) failConnection()
      }, RECONNECT.iceDisconnectedMs)
    }
  }

  function openInputChannel(g: number, conn: RTCPeerConnection) {
    const ch = conn.createDataChannel('input', { ordered: true })
    ch.onopen = () => {
      if (g !== gen) return
      inputChannel = ch
      emit('input', { open: true })
    }
    ch.onclose = () => {
      if (g !== gen || inputChannel !== ch) return
      inputChannel = null
      emit('input', { open: false })
    }
    ch.onmessage = (ev) => {
      if (g === gen) onInputMessage(ev.data)
    }
  }

  /** Server messages on the input channel: only known enum values and finite numbers are taken (T-20-33). */
  function onInputMessage(data: unknown) {
    if (typeof data !== 'string') return
    let m: Record<string, unknown>
    try {
      m = JSON.parse(data)
    } catch {
      return
    }
    if (typeof m !== 'object' || m === null) return
    if (m.t === 'control') {
      if (m.input !== 'ok' && m.input !== 'needs_accessibility') return
      const win = m.window === 'raise_failed' || m.window === 'minimized' ? m.window : ''
      emit('control', { input: m.input, window: win })
    } else if (m.t === 'cursor') {
      if (Number.isFinite(m.x) && Number.isFinite(m.y) && Number.isInteger(m.e)) {
        emit('cursor', { x: m.x as number, y: m.y as number, e: m.e as number })
      }
    }
  }

  async function handle(g: number, raw: unknown) {
    if (typeof raw !== 'object' || raw === null) return
    const m = raw as Record<string, unknown>
    try {
      switch (m.type) {
        case 'hello': {
          if (hadHello) return
          hadHello = true
          helloTimer = cancel(helloTimer)
          const conn = makePC()
          pc = conn
          conn.ontrack = (e) => {
            if (g === gen) onTrack(e)
          }
          conn.onconnectionstatechange = () => onIceState(g, conn.connectionState)
          const rights = (m.rights ?? {}) as Record<string, unknown>
          emit('rights', { control: rights.control === true, clipboard: rights.clipboard === true, surface: rights.surface === true })
          // Before addTransceiver and createOffer, so the offer carries an application m-line.
          if ((m.caps as Record<string, unknown> | undefined)?.input === 1) openInputChannel(g, conn)
          conn.addTransceiver('video', { direction: 'recvonly' })
          await conn.setLocalDescription(await conn.createOffer())
          await gathered(conn)
          if (g !== gen) return
          transport?.send({ type: 'offer', sdp: conn.localDescription?.sdp ?? '' })
          break
        }
        case 'answer':
          if (typeof m.sdp === 'string' && pc) await pc.setRemoteDescription({ type: 'answer', sdp: m.sdp })
          break
        case 'state':
          // Only an enum value and integers are taken from the wire; anything else is dropped.
          if (typeof m.state === 'string' && STATES.has(m.state)) {
            lastState = m.state
            lastViewers = Number.isInteger(m.viewers) ? (m.viewers as number) : 0
            emit('state', { state: m.state as DesktopState, viewers: lastViewers, locked: m.locked === true })
          }
          break
        case 'surface': {
          const kind = m.kind
          if (!Number.isInteger(m.e) || (m.e as number) < 1 || (kind !== 'display' && kind !== 'window')) break
          if (!Number.isInteger(m.id) || !Number.isInteger(m.w) || !Number.isInteger(m.h)) break
          epoch = m.e as number
          emit('surface', {
            e: epoch,
            kind,
            id: m.id as number,
            name: typeof m.name === 'string' ? m.name : '',
            w: m.w as number,
            h: m.h as number,
            reason: m.reason === 'window_closed' || m.reason === 'display_gone' ? m.reason : '',
          })
          break
        }
        case 'size':
          if (Number.isInteger(m.w) && Number.isInteger(m.h)) emit('size', { w: m.w as number, h: m.h as number })
          break
        case 'pong':
          if (m.n === pendingPing) {
            pendingPing = null
            pingTimer = cancel(pingTimer)
          }
          break
        case 'error':
          // version, bad_message or offer_failed: rebuild and try again.
          failConnection()
          break
      }
    } catch {
      if (g === gen) failConnection()
    }
  }

  function onWake(reason: WakeReason) {
    emit('wake', reason)
    if (!running) return
    if (status.kind === 'streaming' && transport) {
      // A dead iOS socket can still read OPEN, so ask and wait for the answer.
      if (pendingPing !== null) return
      const g = gen
      const n = ++pingN
      pendingPing = n
      transport.send({ type: 'ping', n })
      pingTimer = clock.setTimeout(() => {
        pingTimer = null
        if (g !== gen || pendingPing !== n) return
        teardown()
        attempt = 0
        markLost()
        redialNow()
      }, RECONNECT.pingTimeoutMs)
      return
    }
    // The first attempt is still in flight: a pageshow at load must not kill it.
    if (status.kind === 'connecting' && transport) return
    attempt = 0
    redialNow()
  }

  const onVisibility = () => {
    if (doc.visibilityState === 'visible') onWake('visibility')
  }
  const onPageShow = () => onWake('pageshow')
  const onOnline = () => onWake('online')

  function shutdown() {
    running = false
    teardown()
    redialTimer = cancel(redialTimer)
    unreachableTimer = cancel(unreachableTimer)
    debugTimer = cancel(debugTimer)
    doc.removeEventListener('visibilitychange', onVisibility)
    win.removeEventListener('pageshow', onPageShow)
    win.removeEventListener('online', onOnline)
  }

  function startDebug() {
    let flag = false
    try {
      flag = win.localStorage?.getItem('portty:desktop-debug') === '1'
    } catch {
      // storage blocked: the fragment still works
    }
    if (win.location.hash !== '#debug' && !flag) return
    // Counts only: no token, no SDP. Read by TestDesktopCSP_E2E and the VM rows.
    let cspViolations = 0
    doc.addEventListener('securitypolicyviolation', () => { cspViolations++ })
    const tick = async () => {
      let framesDecoded = 0, frameWidth = 0, frameHeight = 0, bytesReceived = 0
      if (pc) {
        const report = await pc.getStats()
        report.forEach((s) => {
          if (s.type === 'inbound-rtp' && s.kind === 'video') {
            framesDecoded = s.framesDecoded ?? 0
            frameWidth = s.frameWidth ?? 0
            frameHeight = s.frameHeight ?? 0
            bytesReceived = s.bytesReceived ?? 0
          }
        })
      }
      ;(win as unknown as { __desktopStats: object }).__desktopStats = {
        status: status.kind, state: lastState, viewers: lastViewers,
        framesDecoded, frameWidth, frameHeight, bytesReceived, cspViolations,
      }
      doc.title = `Portty Desktop (debug fd=${framesDecoded} fw=${frameWidth} csp=${cspViolations} st=${status.kind})`
      if (running) debugTimer = clock.setTimeout(() => void tick(), 1000)
    }
    void tick()
  }

  return {
    start() {
      if (running) return
      running = true
      doc.addEventListener('visibilitychange', onVisibility)
      win.addEventListener('pageshow', onPageShow)
      win.addEventListener('online', onOnline)
      armUnreachable()
      connect()
      startDebug()
    },
    stop: shutdown,
    wake: onWake,
    send(msg) {
      transport?.send(msg)
    },
    sendInput(msg, droppable = false) {
      const ch = inputChannel
      if (!ch || ch.readyState !== 'open') return false
      if (droppable && ch.bufferedAmount > MOVE_BUFFER_LIMIT) return false
      ch.send(JSON.stringify(msg))
      return true
    },
    currentEpoch: () => epoch,
    on(event, cb) {
      const set = listeners[event] as Set<typeof cb>
      set.add(cb)
      return () => set.delete(cb)
    },
  }
}

export async function fetchCheck(loc: Location = window.location): Promise<CheckVerdict> {
  const { capToken } = readWebModeParams(loc)
  // A hung check would stall the classification (and so the redial) forever.
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), 5000)
  try {
    const res = await fetch(`/desktop/check?cap=${encodeURIComponent(capToken ?? '')}`, {
      cache: 'no-store',
      credentials: 'same-origin',
      signal: ctrl.signal,
    })
    if (res.status === 204) return 'ok'
    if (res.status === 401) return 'token'
    if (res.status === 410) return 'off'
    if (res.status === 403 && res.headers.has('X-Portty')) return 'other_account'
    return 'retry'
  } catch {
    return 'retry'
  } finally {
    clearTimeout(timer)
  }
}
