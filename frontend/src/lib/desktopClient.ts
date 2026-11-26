import { readWebModeParams } from './webMode'

/**
 * Phase 19-06 — the /desktop/ viewer's connection. One signalling WebSocket
 * (protocol v1, 19-01) plus one RTCPeerConnection that receives the Mac's
 * screen as a video track. View only: no data channel, no input, no key
 * listeners (control is Phase 20).
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

export interface DesktopEvents {
  track: MediaStream
  state: DesktopStateMsg
  size: DesktopSizeMsg
  status: ClientStatus
  wake: WakeReason
}

export interface DesktopClient {
  start(): void
  stop(): void
  wake(reason: WakeReason): void
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
  const clock = opts.clock ?? realClock
  const makeTransport = opts.transport ?? (() => webSocketTransport(defaultDesktopWsUrl(win.location)))
  const makePC = opts.createPeerConnection ?? (() => new RTCPeerConnection({ iceServers: [] }))

  const listeners: { [K in keyof DesktopEvents]: Set<(v: DesktopEvents[K]) => void> } = {
    track: new Set(), state: new Set(), size: new Set(), status: new Set(), wake: new Set(),
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

  let transport: SignalTransport | null = null
  let pc: RTCPeerConnection | null = null

  function connect() {
    const t = makeTransport()
    transport = t
    t.onOpen(() => t.send({ type: 'hello', v: 1, caps: { video: 1 } }))
    t.onMessage((m) => void handle(m as Record<string, unknown>))
    t.onClose(() => {})
  }

  async function handle(m: Record<string, unknown>) {
    switch (m.type) {
      case 'hello': {
        const conn = makePC()
        pc = conn
        conn.ontrack = (e) => {
          emit('track', e.streams[0] ?? new MediaStream([e.track]))
          setStatus({ kind: 'streaming' })
        }
        conn.addTransceiver('video', { direction: 'recvonly' })
        await conn.setLocalDescription(await conn.createOffer())
        if (conn.iceGatheringState !== 'complete') {
          await new Promise<void>((resolve) => {
            conn.onicegatheringstatechange = () => conn.iceGatheringState === 'complete' && resolve()
          })
        }
        transport?.send({ type: 'offer', sdp: conn.localDescription?.sdp ?? '' })
        break
      }
      case 'answer':
        if (typeof m.sdp === 'string') await pc?.setRemoteDescription({ type: 'answer', sdp: m.sdp })
        break
      case 'state':
        if (typeof m.state === 'string' && STATES.has(m.state)) {
          emit('state', { state: m.state as DesktopState, viewers: Number(m.viewers) || 0, locked: m.locked === true })
        }
        break
      case 'size':
        if (Number.isInteger(m.w) && Number.isInteger(m.h)) emit('size', { w: m.w as number, h: m.h as number })
        break
    }
  }

  function startDebug() {
    const hash = win.location.hash === '#debug'
    let flag = false
    try {
      flag = win.localStorage?.getItem('portty:desktop-debug') === '1'
    } catch {
      // storage blocked: the fragment still works
    }
    if (!hash && !flag) return
    // Counts only: no token, no SDP. Read by TestDesktopCSP_E2E and the VM rows.
    const doc = opts.doc ?? document
    let cspViolations = 0
    doc.addEventListener('securitypolicyviolation', () => { cspViolations++ })
    let viewers = 0
    let state = ''
    listeners.state.add((s) => { viewers = s.viewers; state = s.state })
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
        status: status.kind, state, viewers, framesDecoded, frameWidth, frameHeight, bytesReceived, cspViolations,
      }
      doc.title = `Portty Desktop (debug fd=${framesDecoded} fw=${frameWidth} csp=${cspViolations} st=${status.kind})`
      clock.setTimeout(() => void tick(), 1000)
    }
    void tick()
  }

  return {
    start() {
      connect()
      startDebug()
    },
    stop() {
      transport?.close()
      pc?.close()
    },
    wake(reason) {
      emit('wake', reason)
    },
    on(event, cb) {
      const set = listeners[event] as Set<typeof cb>
      set.add(cb)
      return () => set.delete(cb)
    },
  }
}

export async function fetchCheck(loc: Location = window.location): Promise<CheckVerdict> {
  const { capToken } = readWebModeParams(loc)
  try {
    const res = await fetch(`/desktop/check?cap=${encodeURIComponent(capToken ?? '')}`, {
      cache: 'no-store',
      credentials: 'same-origin',
    })
    if (res.status === 204) return 'ok'
    if (res.status === 401) return 'token'
    if (res.status === 410) return 'off'
    if (res.status === 403 && res.headers.has('X-Portty')) return 'other_account'
    return 'retry'
  } catch {
    return 'retry'
  }
}
