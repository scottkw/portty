// Binary framing constants matching internal/relay/protocol.go
export const MSG_OUTPUT  = 0x01  // server → client: PTY output
export const MSG_RESIZE  = 0x02  // server → client: resize notification
export const MSG_INPUT   = 0x10  // client → server: keyboard input
export const MSG_RESIZE2 = 0x11  // client → server: resize request
export const MSG_PING    = 0x12  // client → server: keep-alive

// 0x30-0x37 are RETIRED (chat, presence, typing, alias, inject, self). parseServerFrame
// returns {type:'unknown'} for them; never reuse these bytes.

// ServerFrame union type for parsed server messages
export type ServerFrame =
  | { type: 'output'; payload: Uint8Array }
  | { type: 'resize'; cols: number; rows: number }
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

    default:
      return { type: 'unknown' }
  }
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
   */
  onClose?: (code?: number, reason?: string) => void
  /**
   * Phase 157 VIEW-04: called when the server pushes a 0x02 MsgResize frame.
   * Guest viewers (remote || wsURL) use this to honor the host-authority grid.
   * Optional so host-path callers (no remote, no wsURL) can omit it without change.
   */
  onResize?: (cols: number, rows: number) => void
}

/**
 * WebSocket client implementing the binary framing protocol from Phase 2.
 * One instance per terminal session.
 */
export class RelayClient {
  private ws: WebSocket
  private pingInterval: ReturnType<typeof setInterval> | null = null

  constructor(
    port: number,
    sessionId: string,
    private callbacks: RelayClientCallbacks,
    opts?: { remote?: boolean; wsURL?: string },
  ) {
    // Phase 155: wsURL override short-circuits BEFORE port is used — port is 0
    // on web-share so it must never appear in the URL (Pitfall 1).
    let url: string
    if (opts?.wsURL) {
      url = opts.wsURL                          // web-share override (wss://host/...?cap=)
    } else {
      const path = opts?.remote
        ? `/api/relay/remote/${sessionId}/ws`   // daemon proxy → peer (cap looked up server-side; T-134-07-01)
        : `/sessions/${sessionId}/ws`           // local relay direct
      url = `ws://127.0.0.1:${port}${path}`
    }
    this.ws = new WebSocket(url)
    this.ws.binaryType = 'arraybuffer'

    this.ws.onopen = () => {
      callbacks.onOpen?.()
      // Send keep-alive pings every 30 seconds
      this.pingInterval = setInterval(() => {
        if (this.ws.readyState === WebSocket.OPEN) {
          this.ws.send(new Uint8Array([MSG_PING]))
        }
      }, 30_000)
    }

    this.ws.onmessage = (event: MessageEvent) => {
      const frame = parseServerFrame(new Uint8Array(event.data as ArrayBuffer))
      switch (frame.type) {
        case 'output':
          this.callbacks.onOutput(frame.payload)
          break
        case 'resize':
          // VIEW-04 (Phase 157): the server is the host-authority for the PTY grid.
          // Guest viewers (remote || wsURL) pass onResize to honor the server-pushed
          // grid; host-path callers omit it so 0x02 is a no-op for local sessions.
          this.callbacks.onResize?.(frame.cols, frame.rows)
          break
        // 'unknown' frames are silently dropped
      }
    }

    this.ws.onclose = (evt: CloseEvent) => {
      this._clearPing()
      callbacks.onClose?.(evt.code, evt.reason)
    }

    this.ws.onerror = () => {
      // Error is always followed by close; let onClose handle cleanup
    }
  }

  /** Send keyboard input to the PTY. */
  sendInput(text: string): void {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(encodeInputFrame(text))
    }
  }

  /** Notify the PTY of a terminal resize. */
  sendResize(cols: number, rows: number): void {
    if (this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(encodeResizeFrame(cols, rows))
    }
  }

  /** Close the WebSocket connection and stop keep-alive pings. */
  close(): void {
    this._clearPing()
    this.ws.close()
  }

  private _clearPing(): void {
    if (this.pingInterval !== null) {
      clearInterval(this.pingInterval)
      this.pingInterval = null
    }
  }
}
