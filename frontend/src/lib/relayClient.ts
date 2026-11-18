import { CLOSE_REASON_SESSION_ENDED, CLOSE_REASON_SHARE_STOPPED, isTerminalClose } from './closeReasons'

// Binary framing constants matching internal/relay/protocol.go
export const MSG_OUTPUT  = 0x01  // server → client: PTY output
export const MSG_RESIZE  = 0x02  // server → client: resize notification
export const MSG_PONG    = 0x04  // server → client: answer to MSG_PING (relay.MsgPong, pinned in tests)
export const MSG_META    = 0x20  // server → client: JSON metadata (relay.MsgMeta)
export const MSG_INPUT   = 0x10  // client → server: keyboard input
export const MSG_RESIZE2 = 0x11  // client → server: resize request
export const MSG_PING    = 0x12  // client → server: keep-alive

// 0x30-0x37 are RETIRED (chat, presence, typing, alias, inject, self). parseServerFrame
// returns {type:'unknown'} for them; never reuse these bytes.

// ServerFrame union type for parsed server messages
export type ServerFrame =
  | { type: 'output'; payload: Uint8Array }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'pong' }
  | { type: 'meta'; payload: Uint8Array }
  | { type: 'unknown' }

/**
 * Encode keyboard input into a binary frame.
 * Format: [MSG_INPUT, ...UTF-8 bytes of text]
 */
export function encodeInputFrame(text: string): Uint8Array {
  const encoded = new TextEncoder().encode(text)
  const frame = new Uint8Array(1 + encoded.length)
  frame[0] = MSG_INPUT
  frame.set(encoded, 1)
  return frame
}

/**
 * Encode a terminal resize request into a 5-byte binary frame.
 * Format: [MSG_RESIZE2, cols_hi, cols_lo, rows_hi, rows_lo]
 */
export function encodeResizeFrame(cols: number, rows: number): Uint8Array {
  return new Uint8Array([
    MSG_RESIZE2,
    (cols >> 8) & 0xff,
    cols & 0xff,
    (rows >> 8) & 0xff,
    rows & 0xff,
  ])
}

/**
 * Parse a server-sent binary frame into a typed union.
 */
export function parseServerFrame(data: Uint8Array): ServerFrame {
  if (data.length === 0) {
    return { type: 'unknown' }
  }

  const typeByte = data[0]

  switch (typeByte) {
    case MSG_OUTPUT:
      return { type: 'output', payload: data.slice(1) }

    case MSG_RESIZE: {
      if (data.length < 5) return { type: 'unknown' }
      const cols = (data[1] << 8) | data[2]
      const rows = (data[3] << 8) | data[4]
      return { type: 'resize', cols, rows }
    }

    case MSG_PONG:
      return { type: 'pong' }

    case MSG_META:
      return { type: 'meta', payload: data.slice(1) }

    default:
      return { type: 'unknown' }
  }
}

/**
 * Reconnect timing (SHAR-07, Phase 5) — the one place for these numbers.
 * Backoff 1, 2, 4, 8, 16 s then 30 s cap, ±20 % jitter, reset only after a
 * connection stayed open 30 s. Ping every 15 s; after the first MsgPong a socket
 * silent for 40 s gets a probe ping and is dead 5 s later. The reconnecting
 * banner shows after 3 s so a brief blip stays silent. A system wake (05-13) acts
 * at most once per wakeMinGapMs; a watchdog tick later than wakeGapMs counts as one.
 * A socket with no host frame within connectMs is dropped and retried (WR-06); it
 * is longer than the daemon proxy's 10 s upstream dial, so that path ends first.
 */
export const RECONNECT = {
  minMs: 1_000,
  maxMs: 30_000,
  jitter: 0.2,
  healthyResetMs: 30_000,
  pingMs: 15_000,
  deadMs: 40_000,
  probeGraceMs: 5_000,
  bannerDelayMs: 3_000,
  wakeGapMs: 5_000,
  wakeMinGapMs: 5_000,
  connectMs: 15_000,
} as const

export type RelayState = 'connecting' | 'open' | 'reconnecting' | 'stopped'

/** Answer of the re-auth hook run before each redial (05-05 wires the real ones). */
export type ReconnectVerdict =
  | { kind: 'ok'; port?: number }
  | { kind: 'retry' }
  | { kind: 'stop'; reason: 'share-stopped' | 'ended' }

/** Delay before reconnect attempt `attempt` (0-based); `rand` in [0, 1). */
export function backoffDelayMs(attempt: number, rand: number): number {
  const base = Math.min(RECONNECT.maxMs, RECONNECT.minMs * 2 ** attempt)
  return Math.min(RECONNECT.maxMs, Math.round(base * (1 - RECONNECT.jitter + 2 * RECONNECT.jitter * rand)))
}

// Callbacks for RelayClient events
export interface RelayClientCallbacks {
  onOutput: (data: Uint8Array) => void
  onOpen?: () => void
  /**
   * Phase 175-06 (BUG-02 / #125): fired once when the WebSocket closes,
   * carrying the raw CloseEvent code/reason. Both server write pumps
   * (webserver/server.go, relay/server.go) send a fixed generic reason
   * ("session ended") on hub.Done() — never a raw error string or a
   * user-set session name (T-175-06-01). code/reason are optional so
   * existing () => void-shaped callers keep working unchanged.
   *
   * With opts.reconnect on (SHAR-07) it fires only for a final close
   * (isTerminalClose) or a stop verdict from beforeReconnect; every other
   * close is retried, and a user close() fires nothing.
   */
  onClose?: (code?: number, reason?: string) => void
  /**
   * Phase 157 VIEW-04: called when the server pushes a 0x02 MsgResize frame.
   * Guest viewers (remote || wsURL) use this to honor the host-authority grid.
   * Optional so host-path callers (no remote, no wsURL) can omit it without change.
   */
  onResize?: (cols: number, rows: number) => void
  /** SHAR-07: reconnect-mode state transitions (fired on change only). */
  onStateChange?: (state: RelayState) => void
  /** SHAR-07: the terminal must be cleared before a full replay lands on existing content. */
  onReset?: () => void
}

/**
 * WebSocket client implementing the binary framing protocol from Phase 2.
 * One instance per terminal session. With opts.reconnect it replaces its socket
 * after a drop and resumes the stream (SHAR-07); without it, one socket, as before.
 */
export class RelayClient {
  private ws: WebSocket | null = null // null only while the first construction has failed
  private pingInterval: ReturnType<typeof setInterval> | null = null
  private readonly reconnect?: { beforeReconnect?: () => Promise<ReconnectVerdict> }
  private state: RelayState = 'connecting'
  private closedByUser = false
  private attempt = 0
  private openedAt = 0
  private redialing = false
  private retryTimer: ReturnType<typeof setTimeout> | null = null
  private connectTimer: ReturnType<typeof setTimeout> | null = null
  // Bumped on every dial and abandonment: handlers of an older socket ignore their events.
  private socketGen = 0
  // Liveness (reconnect mode): armed by the first MsgPong so an older host that never
  // answers pings is never declared dead. Wall-clock times; 0 means none.
  private pongSeen = false
  private lastFrameAt = 0
  private probeSentAt = 0
  private watchdog: ReturnType<typeof setInterval> | null = null
  private lastWakeAt = 0
  private lastTickAt = Date.now()
  // Stream position (05-01 contract): offset meta + (payload.length + 1) per later output frame.
  private offset = 0
  private offsetValid = false
  private hasRendered = false
  private awaitingReplayMeta = true

  constructor(
    private port: number,
    private readonly sessionId: string,
    private callbacks: RelayClientCallbacks,
    private readonly opts?: {
      remote?: boolean
      wsURL?: string
      reconnect?: { beforeReconnect?: () => Promise<ReconnectVerdict> }
      /** HARD-05: the stream offset a restored peer-tab snapshot already shows. */
      resume?: number
    },
  ) {
    this.reconnect = opts?.reconnect
    const resume = opts?.resume
    if (resume !== undefined && Number.isSafeInteger(resume) && resume >= 0) {
      // HARD-05: the terminal already shows the stream up to here, so ask for a delta
      // (a full answer resets it via onReset).
      this.offset = resume
      this.offsetValid = true
      this.hasRendered = true
    }
    if (this.reconnect) {
      this.watchdog = setInterval(this.checkLiveness, 1_000)
      window.addEventListener('online', this.onWake)
      window.addEventListener('portty:wake', this.onSystemWake)
      document.addEventListener('visibilitychange', this.onWake)
    }
    this.dial(this.withResume(this.baseURL()))
  }

  // SHAR-07 / HARD-05: ask the host for a delta from the counted offset when it is exact.
  private withResume(url: string): string {
    return this.offsetValid ? `${url}${url.includes('?') ? '&' : '?'}resume=${this.offset}` : url
  }

  /**
   * HARD-05: the exact stream offset of everything handed to onOutput, or null when
   * it is not known (no offset meta yet on this socket) or the stream ended for good
   * (stopped: nothing to resume). Saved with a peer-tab snapshot.
   */
  resumePoint(): number | null {
    return this.offsetValid && this.state !== 'stopped' ? this.offset : null
  }

  private baseURL(): string {
    // Phase 155: wsURL override short-circuits BEFORE port is used — port is 0
    // on web-share so it must never appear in the URL (Pitfall 1).
    if (this.opts?.wsURL) return this.opts.wsURL // web-share override (wss://host/...?cap=)
    const path = this.opts?.remote
      ? `/api/relay/remote/${this.sessionId}/ws`   // daemon proxy → peer (cap looked up server-side; T-134-07-01)
      : `/sessions/${this.sessionId}/ws`           // local relay direct
    return `ws://127.0.0.1:${this.port}${path}`
  }

  private dial(url: string): void {
    // offsetValid survives until this socket's replay starts (WR-02): a socket the
    // proxy accepts and then refuses rendered nothing, so the old offset is still exact.
    this.awaitingReplayMeta = true
    this.openedAt = 0
    this.probeSentAt = 0
    const gen = ++this.socketGen
    let ws: WebSocket
    try {
      ws = new WebSocket(url)
    } catch {
      // A malformed URL throws. Never log the URL or the error: both can carry ?cap=.
      console.error('[RelayClient] could not open a WebSocket')
      if (this.reconnect) this.scheduleRetry()
      else setTimeout(() => this.callbacks.onClose?.(1006, ''), 0) // never re-enter the caller
      return
    }
    ws.binaryType = 'arraybuffer'
    this.ws = ws
    if (this.reconnect) {
      // WR-06: a handshake to a sleeping host, or a host that never answers, would
      // otherwise hold this attempt until the browser gives up (minutes).
      this._clearConnect()
      this.connectTimer = setTimeout(() => {
        this.connectTimer = null
        this.socketGen++
        ws.onopen = ws.onmessage = ws.onclose = ws.onerror = null
        ws.close()
        this.scheduleRetry()
      }, RECONNECT.connectMs)
    }

    // Every handler ignores events from a socket that is no longer current.
    ws.onopen = () => {
      if (gen !== this.socketGen) return
      this.lastFrameAt = Date.now()
      if (!this.reconnect) {
        this.openedAt = this.lastFrameAt
        this.callbacks.onOpen?.()
      }
      this._clearPing()
      this.pingInterval = setInterval(() => {
        if (this.ws?.readyState === WebSocket.OPEN) {
          this.ws.send(new Uint8Array([MSG_PING]))
        }
      }, RECONNECT.pingMs)
    }

    ws.onmessage = (event: MessageEvent) => {
      if (gen !== this.socketGen) return
      this.lastFrameAt = Date.now()
      this.probeSentAt = 0
      // WR-03: the host answered (its grid comes at once on join), so a slow replay is
      // never killed by the connect timer. Input opens only after the replay (markOpen).
      if (this.reconnect && this.openedAt === 0) {
        this._clearConnect()
        this.openedAt = this.lastFrameAt
      }
      this.handleFrame(parseServerFrame(new Uint8Array(event.data as ArrayBuffer)))
    }

    ws.onclose = (evt: CloseEvent) => {
      if (gen !== this.socketGen) return
      this._clearPing()
      this._clearConnect()
      if (!this.reconnect) {
        this.callbacks.onClose?.(evt.code, evt.reason)
        return
      }
      if (this.closedByUser) return
      if (isTerminalClose(evt.code, evt.reason)) {
        this.stop(evt.code, evt.reason)
        return
      }
      // A first connect that fails (openedAt 0) is retried like a drop.
      this.resetBackoffIfHealthy()
      this.openedAt = 0 // the watchdog leaves a closed socket alone
      this.scheduleRetry()
    }

    ws.onerror = () => {
      // Error is always followed by close; let onClose handle cleanup
    }
  }

  private handleFrame(frame: ServerFrame): void {
    switch (frame.type) {
      case 'output':
        // Output before any replay meta on this socket: an older host sent a full
        // replay without saying so, so clear what is already shown.
        if (this.awaitingReplayMeta) {
          if (this.hasRendered) this.callbacks.onReset?.()
          this.awaitingReplayMeta = false
          this.offsetValid = false
          this.markOpen() // an older host: no offset meta will mark the replay's end
        }
        if (this.offsetValid) this.offset += frame.payload.length + 1
        this.hasRendered = true
        this.callbacks.onOutput(frame.payload)
        break
      case 'resize':
        // VIEW-04 (Phase 157): the server is the host-authority for the PTY grid.
        // Guest viewers (remote || wsURL) pass onResize to honor the server-pushed
        // grid; host-path callers omit it so 0x02 is a no-op for local sessions.
        this.callbacks.onResize?.(frame.cols, frame.rows)
        break
      case 'meta':
        this.handleMeta(frame.payload)
        break
      case 'pong':
        this.pongSeen = true
        break
      // 'unknown' frames are dropped
    }
  }

  private resetBackoffIfHealthy(): void {
    if (this.openedAt && Date.now() - this.openedAt >= RECONNECT.healthyResetMs) this.attempt = 0
  }

  private sendProbe(): void {
    if (this.ws?.readyState !== WebSocket.OPEN) return
    this.ws.send(new Uint8Array([MSG_PING]))
    this.probeSentAt = Date.now()
  }

  // Half-open detection. A large Date.now() delta alone (a clock jump after sleep)
  // never kills a socket: silence earns a probe, and only an unanswered probe kills.
  // A tick far later than 1 s means the machine slept (05-13: the browser's wake signal).
  private checkLiveness = (): void => {
    const now = Date.now()
    const gap = now - this.lastTickAt
    this.lastTickAt = now
    if (gap > RECONNECT.wakeGapMs) this.onSystemWake()
    // From the host's first frame, not only from open: a socket can die mid-replay.
    if (this.openedAt === 0 || !this.pongSeen || document.visibilityState !== 'visible') return
    if (this.probeSentAt === 0) {
      if (now - this.lastFrameAt > RECONNECT.deadMs) this.sendProbe()
      return
    }
    if (now - this.probeSentAt >= RECONNECT.probeGraceMs) this.abandon()
  }

  private onWake = (): void => {
    if (document.visibilityState !== 'visible') return
    if (this.state === 'reconnecting' && this.retryTimer !== null) {
      this._clearRetry() // skip the remaining backoff
      void this.redial()
    } else if (this.state === 'open') {
      this.sendProbe()
    }
  }

  // 05-13: this Mac woke or changed networks (the Go detector via App.tsx's
  // 'portty:wake', or a watchdog tick gap). The redial is not gated on visibility, so a hidden window is
  // ready when shown; the probe kill keeps checkLiveness's visibility rule.
  private onSystemWake = (): void => {
    const now = Date.now()
    if (now - this.lastWakeAt < RECONNECT.wakeMinGapMs) return
    this.lastWakeAt = now
    if (this.state === 'reconnecting') {
      if (this.retryTimer !== null) {
        this._clearRetry()
        void this.redial()
      }
      this.attempt = 0 // after redial's synchronous attempt++: the next wait is the first step
    } else if (this.state === 'open') {
      this.attempt = 0
      this.sendProbe()
    }
  }

  // Drop a dead socket without waiting for its close (a half-open socket can sit in
  // CLOSING for a long time) and dial a new one at once.
  private abandon(): void {
    const ws = this.ws! // only reached after a host frame, so a socket exists
    this.socketGen++
    this._clearPing()
    ws.onopen = ws.onmessage = ws.onclose = ws.onerror = null
    ws.close()
    this.resetBackoffIfHealthy()
    this.openedAt = 0
    this.setState('reconnecting')
    void this.redial()
  }

  private handleMeta(payload: Uint8Array): void {
    let m: unknown
    try {
      m = JSON.parse(new TextDecoder().decode(payload))
    } catch {
      this.offsetValid = false // a bad frame forces an explicit full replay next time, never a partial
      return
    }
    if (typeof m !== 'object' || m === null) {
      this.offsetValid = false
      return
    }
    const { replay, offset } = m as { replay?: unknown; offset?: unknown }
    if (typeof replay === 'string') {
      if (replay === 'full' && this.hasRendered) this.callbacks.onReset?.()
      this.awaitingReplayMeta = false
      this.offsetValid = false // trusted again only after this socket's offset meta
    }
    if (offset !== undefined) {
      if (typeof offset === 'number' && Number.isSafeInteger(offset) && offset >= 0) {
        this.offset = offset
        this.offsetValid = true
      } else {
        this.offsetValid = false
      }
      // WR-01 it3: the replay is written, so the host reads input from here on.
      this.markOpen()
    }
  }

  // Reconnect mode: open (input accepted) once the host's catch-up is written. Typing
  // at a half-drawn replay would be queued by the host and land late (T-05-14).
  private markOpen(): void {
    if (!this.reconnect || this.state === 'open') return
    this.setState('open')
    this.callbacks.onOpen?.()
  }

  private setState(state: RelayState): void {
    if (this.state === state) return
    this.state = state
    this.callbacks.onStateChange?.(state)
  }

  private scheduleRetry(): void {
    this.setState('reconnecting')
    this._clearRetry()
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null
      void this.redial()
    }, backoffDelayMs(this.attempt, Math.random()))
  }

  private async redial(): Promise<void> {
    if (this.redialing) return
    this.redialing = true
    this.attempt++
    let verdict: ReconnectVerdict
    try {
      verdict = (await this.reconnect?.beforeReconnect?.()) ?? { kind: 'ok' }
    } catch {
      verdict = { kind: 'retry' }
    }
    this.redialing = false
    if (this.closedByUser || this.state !== 'reconnecting') return
    if (verdict.kind === 'stop') {
      this.stop(1000, verdict.reason === 'share-stopped' ? CLOSE_REASON_SHARE_STOPPED : CLOSE_REASON_SESSION_ENDED)
      return
    }
    if (verdict.kind === 'retry') {
      this.scheduleRetry()
      return
    }
    if (verdict.port) this.port = verdict.port // a wsURL ignores the port
    this.dial(this.withResume(this.baseURL()))
  }

  private stop(code: number, reason: string): void {
    this.teardown()
    this.setState('stopped')
    this.callbacks.onClose?.(code, reason)
  }

  private teardown(): void {
    this._clearPing()
    this._clearRetry()
    this._clearConnect()
    if (this.watchdog !== null) {
      clearInterval(this.watchdog)
      this.watchdog = null
      window.removeEventListener('online', this.onWake)
      window.removeEventListener('portty:wake', this.onSystemWake)
      document.removeEventListener('visibilitychange', this.onWake)
    }
  }

  /** Send keyboard input to the PTY. Dropped (never queued) unless the connection is open. */
  sendInput(text: string): void {
    if (this.reconnect && this.state !== 'open') return // T-05-14, WR-03
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(encodeInputFrame(text))
    }
  }

  /** Notify the PTY of a terminal resize. */
  sendResize(cols: number, rows: number): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(encodeResizeFrame(cols, rows))
    }
  }

  /** Close the WebSocket connection and stop keep-alive pings (and any reconnecting). */
  close(): void {
    this.closedByUser = true
    if (!this.reconnect) {
      this._clearPing()
      this.ws?.close()
      return
    }
    this.teardown()
    this.socketGen++
    const ws = this.ws
    if (!ws) return
    ws.onopen = ws.onmessage = ws.onclose = ws.onerror = null
    ws.close()
  }

  private _clearPing(): void {
    if (this.pingInterval !== null) {
      clearInterval(this.pingInterval)
      this.pingInterval = null
    }
  }

  private _clearConnect(): void {
    if (this.connectTimer !== null) {
      clearTimeout(this.connectTimer)
      this.connectTimer = null
    }
  }

  private _clearRetry(): void {
    if (this.retryTimer !== null) {
      clearTimeout(this.retryTimer)
      this.retryTimer = null
    }
  }
}
