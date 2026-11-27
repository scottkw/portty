import { readWebModeParams } from './webMode'
import { MOVE_BUFFER_LIMIT, utf8Len } from './desktopInput'

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
  /** Guest mode only (22-07): the connection ended on a fixed reason and is never retried. */
  | { kind: 'guest_final'; reason: GuestCloseReason }

/** Why a guest connection ended for good (22-UI-SPEC Surface 6 close reasons). */
export type GuestCloseReason = 'denied' | 'admit_timeout' | 'revoked' | 'share_ended' | 'token_invalid' | 'replaced'

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
  /** Guest mode (22-07): the share's own socket and check, an offer only after admission. Defaults to defaultGuestMode(location). */
  guest?: boolean
}

export interface DesktopStateMsg {
  state: DesktopState
  viewers: number
  locked: boolean
  /** The daemon's audio capture or encoder failed; the picture is unaffected. Absent when audio is fine. */
  audio?: 'failed'
}

/** What the server hello offers this viewer: sound from the Mac, and the unlock panel (own-account viewers only). */
export interface DesktopCaps {
  audio: boolean
  unlock: boolean
}

/** The daemon's unlock states (23-04 wire contract). The viewer holds no counters: failures, waits and the pause are the daemon's. */
export type UnlockStateName =
  | 'idle' | 'unlocking' | 'retrying' | 'unlocked' | 'failed' | 'rate_limited' | 'paused' | 'refused' | 'unavailable' | 'error'
export type UnlockReason = '' | 'caps_lock' | 'unmappable' | 'accessibility'
export interface DesktopUnlockMsg {
  state: UnlockStateName
  failures: number
  /** Rate limit or pause: how long until the daemon accepts another try, in ms. 0 when absent. */
  waitMs: number
  reason: UnlockReason
  /** Unlocked state only: Portty typed the password (false would be the Mac unlocked at its own keyboard). */
  byPortty: boolean
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

/** One entry of the daemon's surface list (20-03). Names are the Mac's own text: render as text, never markup. */
export interface SurfaceDisplayInfo {
  id: number
  name: string
  w: number
  h: number
  main: boolean
}
export interface SurfaceWindowInfo {
  id: number
  app: string
  title: string
  /** `App — Title`, as the daemon formats it. */
  name: string
}
export interface DesktopSurfacesMsg {
  displays: SurfaceDisplayInfo[]
  windows: SurfaceWindowInfo[]
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

/** Admission as the share server reports it to a guest: waiting at Admit, then admitted. */
export interface DesktopAdmissionMsg {
  state: 'waiting' | 'admitted'
}

/** Who holds the 2 s input lease, from this guest's side. `name` is set only for `other`, and is the holder's sanitized display name. */
export type LeaseState = 'you' | 'other' | 'owner' | 'physical' | 'free'
export interface DesktopLeaseMsg {
  state: LeaseState
  name: string
}

export interface DesktopEvents {
  track: MediaStream
  state: DesktopStateMsg
  size: DesktopSizeMsg
  status: ClientStatus
  wake: WakeReason
  rights: DesktopRights
  /** The server hello's capabilities, once per connection. */
  caps: DesktopCaps
  /** The input data channel opened or closed. */
  input: { open: boolean }
  /** Own-account viewers only: the daemon's unlock progress (23-04). */
  unlock: DesktopUnlockMsg
  surface: DesktopSurfaceMsg
  /** A good reply to a surfaces request. */
  surfaces: DesktopSurfacesMsg
  /** The daemon refused or failed a select (`code` is its reason, '' when not a string). */
  selectFailed: { code: string }
  control: DesktopControlMsg
  cursor: DesktopCursorMsg
  /** Guest mode: the admission state of this connection. */
  admission: DesktopAdmissionMsg
  /** Guest mode: the input lease. */
  lease: DesktopLeaseMsg
}

/** What the daemon answered for a clipboard transfer (20-01 clip_result outcomes). */
export type ClipOutcome = 'ok' | 'concealed' | 'empty' | 'too_large' | 'waiting_approval' | 'failed' | 'forbidden' | 'busy'
export interface ClipResult {
  outcome: ClipOutcome
  /** The Mac's text, only with outcome ok on a copy. Held in memory for the caller; never logged. */
  text?: string
}

/** Clipboard limits (20-CONTEXT [Limits]): a chunk's text, an encoded frame (under the 32768-byte read limit), the whole text, the wait. */
export const CLIP_CHUNK_BYTES = 16384
export const CLIP_FRAME_MAX = 30000
export const CLIP_MAX_BYTES = 1048576
export const CLIP_TIMEOUT_MS = 20000
// A sane bound on `total` from the wire: 1 MiB in the smallest chunks a JSON-escaped frame allows is about 210.
const CLIP_MAX_CHUNKS = 1024
// What a clip_put frame carries besides its text (type, xid, seq, total, len), rounded up.
const CLIP_FRAME_OVERHEAD = 256
const CLIP_OUTCOMES: ReadonlySet<string> = new Set<ClipOutcome>([
  'ok', 'concealed', 'empty', 'too_large', 'waiting_approval', 'failed', 'forbidden', 'busy',
])

const utf8Encoder = new TextEncoder()
/** UTF-8 length of a string as the daemon counts it. */
export const clipByteLength = (s: string): number => utf8Encoder.encode(s).length

/** Bytes this code point takes inside a JSON string as JSON.stringify writes it. */
function jsonBytes(cp: number): number {
  if (cp === 0x22 || cp === 0x5c) return 2
  if (cp < 0x20) return cp === 8 || cp === 9 || cp === 10 || cp === 12 || cp === 13 ? 2 : 6
  if (cp >= 0xd800 && cp <= 0xdfff) return 6 // a lone surrogate is written as \udXXX
  return utf8Len(cp)
}

/**
 * Cuts text for clip_put: at code-point boundaries, at most CLIP_CHUNK_BYTES of UTF-8 per chunk,
 * and few enough JSON-escaped bytes that each encoded frame stays at most CLIP_FRAME_MAX.
 */
export function chunkClipText(s: string): string[] {
  const escBudget = CLIP_FRAME_MAX - CLIP_FRAME_OVERHEAD
  const out: string[] = []
  let cur = ''
  let raw = 0
  let esc = 0
  for (const ch of s) {
    const cp = ch.codePointAt(0)!
    const r = utf8Len(cp)
    const e = jsonBytes(cp)
    if (cur !== '' && (raw + r > CLIP_CHUNK_BYTES || esc + e > escBudget)) {
      out.push(cur)
      cur = ''
      raw = 0
      esc = 0
    }
    cur += ch
    raw += r
    esc += e
  }
  if (cur !== '') out.push(cur)
  return out
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
  /**
   * The unlock request: `bytes` (type byte 0x01, then UTF-8) as one binary message on the input channel,
   * never the signalling socket. False when the channel is not open. The caller zeroes `bytes` afterwards.
   */
  sendUnlock(bytes: Uint8Array): boolean
  /** The surface epoch last announced by the server, or null until the first surface message. */
  currentEpoch(): number | null
  /** Asks for the displays and windows. Resolves on the next reply; rejects on an error reply, a dropped connection or after 5 s. */
  requestSurfaces(): Promise<DesktopSurfacesMsg>
  /** Picks a surface; the answer is a `surface` event for every viewer, or `selectFailed` for this one. Display id 0 is the main display. */
  selectSurface(kind: SurfaceKind, id: number): void
  /**
   * Asks the Mac for its clipboard text (clip_get) and collects the chunks in order. Never rejects:
   * a gap, a duplicate, a length mismatch, a dropped connection or CLIP_TIMEOUT_MS is outcome failed;
   * a transfer already in flight is busy. Call it only inside the user's own click.
   */
  copyFromMac(): Promise<ClipResult>
  /** Sends text to the Mac's clipboard as clip_put chunks. Empty is `empty` and over CLIP_MAX_BYTES is `too_large`, with nothing sent. Never rejects. */
  pasteToMac(text: string): Promise<{ outcome: ClipOutcome }>
  on<K extends keyof DesktopEvents>(event: K, cb: (v: DesktopEvents[K]) => void): () => void
}

export const RECONNECT = {
  delaysMs: [1000, 2000, 4000, 8000, 10000],
  unreachableAfterMs: 30000,
  pingTimeoutMs: 3000,
  helloTimeoutMs: 10000,
  gatherTimeoutMs: 3000,
  iceDisconnectedMs: 2000,
  surfacesTimeoutMs: 5000,
}

/** wss://<same host>/desktop/ws?cap=<token from the page URL>. */
export function defaultDesktopWsUrl(loc: Location = window.location): string {
  const { capToken } = readWebModeParams(loc)
  return `wss://${loc.host}/desktop/ws?cap=${encodeURIComponent(capToken ?? '')}`
}

/** True on the guest page: the path starts with /desktop/guest/ (the share server's route, 22-01). */
export function defaultGuestMode(loc: Location = window.location): boolean {
  // typeof: an injected window in a test may carry no pathname.
  return typeof loc.pathname === 'string' && loc.pathname.startsWith('/desktop/guest/')
}

/** wss://<same host>/desktop/guest/ws?cap=<token from the page URL>. */
export function defaultGuestWsUrl(loc: Location = window.location): string {
  const { capToken } = readWebModeParams(loc)
  return `wss://${loc.host}/desktop/guest/ws?cap=${encodeURIComponent(capToken ?? '')}`
}

/**
 * Asks the Mac for stereo Opus: adds stereo=1 to the Opus fmtp line of the offer (or a new fmtp line when
 * there is none). Browsers offer mono by default, and the Mac's music would come out as one channel.
 * Every other line is left untouched.
 */
export function withOpusStereo(sdp: string): string {
  const pt = /^a=rtpmap:(\d+) opus\/48000\/2\r?$/m.exec(sdp)?.[1]
  if (!pt) return sdp
  const fmtp = new RegExp(`^(a=fmtp:${pt} )(.*?)(\\r?)$`, 'm')
  if (fmtp.test(sdp)) {
    return sdp.replace(fmtp, (_, head: string, params: string, cr: string) => {
      const kept = params.split(';').filter((p) => p !== '' && !p.startsWith('stereo='))
      return `${head}${[...kept, 'stereo=1'].join(';')}${cr}`
    })
  }
  return sdp.replace(new RegExp(`^(a=rtpmap:${pt} opus/48000/2)(\\r?)$`, 'm'), (_, line: string, cr: string) => `${line}${cr}\na=fmtp:${pt} stereo=1${cr}`)
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

// Fixed close codes of the share server (22-04); the page tells the states apart by code, never by text.
const GUEST_CLOSE: Readonly<Record<number, GuestCloseReason>> = {
  4401: 'token_invalid',
  4410: 'share_ended',
  4430: 'denied',
  4431: 'admit_timeout',
  4432: 'revoked',
  4433: 'share_ended',
  // Final, not retried: a retry would knock again and replace the other tab, which would retry in turn.
  4434: 'replaced',
}

const LEASE_STATES: ReadonlySet<string> = new Set<LeaseState>(['you', 'other', 'owner', 'physical', 'free'])

/** Belt and braces on a name from the wire: no control or format characters, one space between words, at most 128 characters. */
function cleanName(v: unknown): string {
  if (typeof v !== 'string') return ''
  return [...v.replace(/[\p{Cc}\p{Cf}]/gu, ' ').replace(/\s+/g, ' ').trim()].slice(0, 128).join('')
}

function parseRights(v: unknown): DesktopRights {
  const r = (isObj(v) ? v : {}) as Record<string, unknown>
  return { control: r.control === true, clipboard: r.clipboard === true, surface: r.surface === true }
}

const UNLOCK_STATES: ReadonlySet<string> = new Set<UnlockStateName>([
  'idle', 'unlocking', 'retrying', 'unlocked', 'failed', 'rate_limited', 'paused', 'refused', 'unavailable', 'error',
])
const UNLOCK_REASONS: ReadonlySet<string> = new Set(['caps_lock', 'unmappable', 'accessibility'])
const UNLOCK_MAX_WAIT_MS = 3600000

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
  const guest = opts.guest ?? defaultGuestMode(win.location)
  const makeTransport =
    opts.transport ?? (() => webSocketTransport(guest ? defaultGuestWsUrl(win.location) : defaultDesktopWsUrl(win.location)))
  const makePC = opts.createPeerConnection ?? (() => new RTCPeerConnection({ iceServers: [] }))
  const check = opts.check ?? (() => (guest ? fetchGuestCheck(win.location) : fetchCheck(win.location)))

  const listeners: { [K in keyof DesktopEvents]: Set<(v: DesktopEvents[K]) => void> } = {
    track: new Set(), state: new Set(), size: new Set(), status: new Set(), wake: new Set(),
    rights: new Set(), caps: new Set(), input: new Set(), unlock: new Set(), surface: new Set(), surfaces: new Set(), selectFailed: new Set(),
    control: new Set(), cursor: new Set(), admission: new Set(), lease: new Set(),
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
  // Guest mode: what the hello offered, kept until admission lets the offer start (once per connection).
  let helloInput = false
  let offered = false
  let hasPicture = false
  // Per peer connection: a picture needs both the track and ICE connected. ontrack fires
  // on setRemoteDescription, long before (or without) any media path.
  let gotTrack = false
  // The one stream this connection's tracks play from (video and audio share it).
  let stream: MediaStream | null = null
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

  // One clipboard transfer at a time (the daemon allows one per viewer and answers busy otherwise).
  interface ClipXfer {
    xid: string
    kind: 'get' | 'put'
    resolve: (r: ClipResult) => void
    timer: unknown
    // Assembly of a copy: the next expected seq, the announced total and len, bytes so far, the pieces.
    next: number
    total: number
    len: number
    bytes: number
    parts: string[]
  }
  let clipXfer: ClipXfer | null = null
  let xidN = 0
  function endClip(outcome: ClipOutcome, text?: string) {
    const x = clipXfer
    if (!x) return
    clipXfer = null
    clock.clearTimeout(x.timer)
    x.resolve(text === undefined ? { outcome } : { outcome, text })
  }
  function startClip(kind: 'get' | 'put', go: (xid: string, t: SignalTransport) => void): Promise<ClipResult> {
    if (clipXfer) return Promise.resolve({ outcome: 'busy' })
    const t = transport
    if (!t || !hadHello) return Promise.resolve({ outcome: 'failed' })
    return new Promise<ClipResult>((resolve) => {
      const xid = `x${++xidN}`
      const x: ClipXfer = {
        xid, kind, resolve, next: 0, total: 0, len: 0, bytes: 0, parts: [],
        timer: clock.setTimeout(() => endClip('failed'), CLIP_TIMEOUT_MS),
      }
      clipXfer = x
      try {
        go(xid, t)
      } catch {
        endClip('failed')
      }
    })
  }

  interface ListWait {
    resolve: (m: DesktopSurfacesMsg) => void
    reject: (e: Error) => void
    timer: unknown
  }
  let listWaits: ListWait[] = []
  /** Ends every waiting requestSurfaces: with the list, or with the reason it will not come. */
  function settleLists(result: DesktopSurfacesMsg | Error) {
    const waits = listWaits
    listWaits = []
    for (const w of waits) {
      clock.clearTimeout(w.timer)
      if (result instanceof Error) w.reject(result)
      else w.resolve(result)
    }
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
    settleLists(new Error('not connected'))
    endClip('failed')
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

  function guestFinal(reason: GuestCloseReason) {
    shutdown()
    setStatus({ kind: 'guest_final', reason })
  }

  function connect() {
    teardown()
    const g = ++gen
    hadHello = false
    helloInput = false
    offered = false
    let t: SignalTransport
    try {
      t = makeTransport()
    } catch {
      retry()
      return
    }
    transport = t
    t.onOpen(() => {
      if (g === gen) t.send({ type: 'hello', v: 1, caps: { video: 1, audio: 1, input: 1 } })
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
    if (guest && GUEST_CLOSE[code]) return guestFinal(GUEST_CLOSE[code])
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
    // A guest's check answers 401, 410 or 403 for a link that is gone: one card for all of them.
    if (guest && (verdict === 'token' || verdict === 'off' || verdict === 'other_account')) return guestFinal('token_invalid')
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
    // Audio and video share one stream id, so the browser hands both the same stream and one <video> plays both.
    // A peer that sent no stream id gets one stream built here instead.
    const given = e.streams[0]
    const s = given ?? stream ?? new MediaStream()
    if (!given) s.addTrack(e.track)
    stream = s
    // The picture waits for the video track; audio alone is never a picture.
    if (e.track.kind === 'audio') return
    gotTrack = true
    emit('track', s)
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

  /** The peer connection and the offer: from the hello, or, for a guest, from admission. */
  async function beginOffer(g: number) {
    offered = true
    stream = null
    const conn = makePC()
    pc = conn
    conn.ontrack = (e) => {
      if (g === gen) onTrack(e)
    }
    conn.onconnectionstatechange = () => onIceState(g, conn.connectionState)
    // Before addTransceiver and createOffer, so the offer carries an application m-line.
    if (helloInput) openInputChannel(g, conn)
    conn.addTransceiver('video', { direction: 'recvonly' })
    conn.addTransceiver('audio', { direction: 'recvonly' })
    const offer = await conn.createOffer()
    await conn.setLocalDescription({ type: 'offer', sdp: withOpusStereo(offer.sdp ?? '') })
    await gathered(conn)
    if (g !== gen) return
    transport?.send({ type: 'offer', sdp: conn.localDescription?.sdp ?? '' })
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
          emit('rights', parseRights(m.rights))
          const caps = m.caps as Record<string, unknown> | undefined
          helloInput = caps?.input === 1
          emit('caps', { audio: caps?.audio === 1, unlock: caps?.unlock === 1 })
          // A guest sends no offer until the share server says admitted.
          if (!guest) await beginOffer(g)
          break
        }
        case 'admission': {
          if (!guest) break
          if (m.state === 'waiting') {
            // Waiting at Admit with the socket up is not an unreachable Mac, and not a lost picture.
            unreachableTimer = cancel(unreachableTimer)
            if (status.kind === 'reconnecting' || status.kind === 'unreachable') setStatus({ kind: 'connecting' })
            emit('admission', { state: 'waiting' })
          } else if (m.state === 'admitted' && !offered) {
            emit('admission', { state: 'admitted' })
            armUnreachable()
            await beginOffer(g)
          }
          break
        }
        case 'rights':
          if (guest) emit('rights', parseRights(m.rights))
          break
        case 'lease':
          if (guest && typeof m.state === 'string' && LEASE_STATES.has(m.state)) {
            emit('lease', { state: m.state as LeaseState, name: m.state === 'other' ? cleanName(m.name) : '' })
          }
          break
        case 'answer':
          if (typeof m.sdp === 'string' && pc) await pc.setRemoteDescription({ type: 'answer', sdp: m.sdp })
          break
        case 'state':
          // Only an enum value and integers are taken from the wire; anything else is dropped.
          if (typeof m.state === 'string' && STATES.has(m.state)) {
            lastState = m.state
            lastViewers = Number.isInteger(m.viewers) ? (m.viewers as number) : 0
            emit('state', {
              state: m.state as DesktopState,
              viewers: lastViewers,
              locked: m.locked === true,
              ...(m.audio === 'failed' ? { audio: 'failed' as const } : {}),
            })
          }
          break
        case 'unlock':
          // Only an enum value and bounded integers are taken from the wire.
          if (typeof m.state === 'string' && UNLOCK_STATES.has(m.state)) {
            const wait = m.wait_ms
            emit('unlock', {
              state: m.state as UnlockStateName,
              failures: Number.isInteger(m.failures) && (m.failures as number) >= 0 ? (m.failures as number) : 0,
              waitMs: Number.isFinite(wait) && (wait as number) > 0 ? Math.min(wait as number, UNLOCK_MAX_WAIT_MS) : 0,
              reason: typeof m.reason === 'string' && UNLOCK_REASONS.has(m.reason) ? (m.reason as UnlockReason) : '',
              byPortty: m.by_portty === true,
            })
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
        case 'surfaces': {
          const list = parseSurfaces(m)
          if (list === null) {
            settleLists(new Error(m.error === 'forbidden' ? 'forbidden' : 'list_failed'))
            break
          }
          emit('surfaces', list)
          settleLists(list)
          break
        }
        case 'select_failed':
          emit('selectFailed', { code: typeof m.code === 'string' ? m.code : '' })
          break
        case 'clip_data': {
          const x = clipXfer
          if (!x || x.kind !== 'get' || m.xid !== x.xid) break
          const { seq, total, len, text } = m
          if (!Number.isInteger(seq) || !Number.isInteger(total) || !Number.isInteger(len) || typeof text !== 'string') {
            endClip('failed')
            break
          }
          if ((total as number) < 1 || (total as number) > CLIP_MAX_CHUNKS || (len as number) < 1 || (len as number) > CLIP_MAX_BYTES) {
            endClip('failed')
            break
          }
          // Every chunk must agree with the first on total and len, and arrive as the next seq.
          if (x.next === 0) {
            x.total = total as number
            x.len = len as number
          } else if (total !== x.total || len !== x.len) {
            endClip('failed')
            break
          }
          const n = clipByteLength(text)
          if (seq !== x.next || (seq as number) >= x.total || n > CLIP_CHUNK_BYTES || x.bytes + n > x.len) {
            endClip('failed')
            break
          }
          x.parts.push(text)
          x.next++
          x.bytes += n
          break
        }
        case 'clip_result': {
          const x = clipXfer
          if (!x || m.xid !== x.xid) break
          const outcome: ClipOutcome = typeof m.outcome === 'string' && CLIP_OUTCOMES.has(m.outcome) ? (m.outcome as ClipOutcome) : 'failed'
          if (x.kind === 'get' && outcome === 'ok') {
            // ok means every chunk came, in order, and add up to the announced length.
            if (x.next === 0 || x.next !== x.total || x.bytes !== x.len) endClip('failed')
            else endClip('ok', x.parts.join(''))
            break
          }
          endClip(outcome)
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
    sendUnlock(bytes) {
      const ch = inputChannel
      if (!ch || ch.readyState !== 'open') return false
      ch.send(bytes as Uint8Array<ArrayBuffer>)
      return true
    },
    currentEpoch: () => epoch,
    requestSurfaces() {
      return new Promise<DesktopSurfacesMsg>((resolve, reject) => {
        if (!transport || !hadHello) {
          reject(new Error('not connected'))
          return
        }
        const wait: ListWait = {
          resolve,
          reject,
          timer: clock.setTimeout(() => {
            listWaits = listWaits.filter((w) => w !== wait)
            reject(new Error('timeout'))
          }, RECONNECT.surfacesTimeoutMs),
        }
        listWaits.push(wait)
        transport.send({ type: 'surfaces' })
      })
    },
    selectSurface(kind, id) {
      transport?.send({ type: 'select', kind, id })
    },
    copyFromMac() {
      return startClip('get', (xid, t) => t.send({ type: 'clip_get', xid }))
    },
    pasteToMac(text) {
      const len = clipByteLength(text)
      if (len === 0) return Promise.resolve({ outcome: 'empty' })
      if (len > CLIP_MAX_BYTES) return Promise.resolve({ outcome: 'too_large' })
      const chunks = chunkClipText(text)
      return startClip('put', (xid, t) => {
        chunks.forEach((c, seq) => t.send({ type: 'clip_put', xid, seq, total: chunks.length, len, text: c }))
      })
    },
    on(event, cb) {
      const set = listeners[event] as Set<typeof cb>
      set.add(cb)
      return () => set.delete(cb)
    },
  }
}

const isObj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null

/** A surfaces reply: only typed fields are kept (T-20-35). Null for an error reply or a malformed list. */
function parseSurfaces(m: Record<string, unknown>): DesktopSurfacesMsg | null {
  if (m.error !== undefined || !Array.isArray(m.displays) || !Array.isArray(m.windows)) return null
  const displays: SurfaceDisplayInfo[] = []
  for (const d of m.displays) {
    if (!isObj(d) || !Number.isInteger(d.id) || typeof d.name !== 'string') continue
    if (!Number.isInteger(d.w) || !Number.isInteger(d.h)) continue
    displays.push({ id: d.id as number, name: d.name, w: d.w as number, h: d.h as number, main: d.main === true })
  }
  const windows: SurfaceWindowInfo[] = []
  for (const w of m.windows) {
    if (!isObj(w) || !Number.isInteger(w.id) || typeof w.name !== 'string') continue
    windows.push({
      id: w.id as number,
      app: typeof w.app === 'string' ? w.app : '',
      title: typeof w.title === 'string' ? w.title : '',
      name: w.name,
    })
  }
  return { displays, windows }
}

/** The guest page's check: 401 (bad token), 410 (share ended) and 403 (refused) all mean the link is gone. */
export async function fetchGuestCheck(loc: Location = window.location): Promise<CheckVerdict> {
  const { capToken } = readWebModeParams(loc)
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), 5000)
  try {
    const res = await fetch(`/desktop/guest/check?cap=${encodeURIComponent(capToken ?? '')}`, {
      cache: 'no-store',
      credentials: 'same-origin',
      signal: ctrl.signal,
    })
    if (res.status === 204) return 'ok'
    if (res.status === 401 || res.status === 410 || res.status === 403) return 'token'
    return 'retry'
  } catch {
    return 'retry'
  } finally {
    clearTimeout(timer)
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
