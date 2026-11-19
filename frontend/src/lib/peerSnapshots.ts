/**
 * HARD-05: App renders only the active peer tab, so switching tabs unmounts the
 * peer's terminal and closes its socket. Keeping the terminal's content plus the
 * stream offset it reflects lets the returning tab restore itself and ask the host
 * for a delta instead of a full replay. In memory only, never persisted.
 *
 * A stale entry is still correct: a snapshot and its offset describe one stream
 * point, and the host answers a full replay when that point is gone.
 *
 * DEBT-04 W1: keyed by host plus session id, because two Macs' daemons can hand
 * out equal session ids. App drops a tab's entry when the tab is removed; the
 * per-key generation stops the unmount's late flush (which lands after the drop
 * when the closed tab was the active one) from storing it again.
 */
import type { Terminal } from '@xterm/xterm'

export interface PeerSnapshot {
  data: string
  cols: number
  rows: number
  offset: number
}

export const PEER_SNAPSHOT_MAX = 8
/** IN-02: a 10,000-line scrollback serializes to megabytes, so entries alone don't bound memory. */
export const PEER_SNAPSHOT_MAX_BYTES = 8 * 1024 * 1024
/** IN-03: how long a remount waits for its tab's save before dialling; a hung flush must not hold the tab blank. */
export const PEER_SAVE_WAIT_MS = 250
/** W2: xterm may never run the flush callback (e.g. a disposed render service); a leaked terminal holds its whole buffer. */
export const PEER_FLUSH_TIMEOUT_MS = 1000

const snapshots = new Map<string, PeerSnapshot>()
const generations = new Map<string, number>()
const pendingSaves = new Map<string, Promise<void>>()
// JS strings are UTF-16 in memory: two bytes per code unit.
const sizeOf = (snap: PeerSnapshot) => 2 * snap.data.length

/** The host part of a peer tab's key: the name App connects by (MagicDNS when the peer has one). */
export function peerSnapshotHost(tab: { connectHost?: string; peerHost?: string }): string | undefined {
  return tab.connectHost ?? tab.peerHost
}

/** The store key; NUL cannot appear in a host or session id, so no pair fakes another. */
export function peerSnapshotKey(host: string, sessionId: string): string {
  return `${host}\0${sessionId}`
}

/** Read at mount and handed back to putPeerSnapshot; a drop in between makes the put a no-op. */
export function peerSnapshotGeneration(key: string): number {
  return generations.get(key) ?? 0
}

/** The tab is gone: forget its snapshot, and refuse a put from the panel that was showing it. */
export function dropPeerSnapshot(host: string, sessionId: string): void {
  const key = peerSnapshotKey(host, sessionId)
  snapshots.delete(key)
  generations.set(key, peerSnapshotGeneration(key) + 1)
}

/** Return the key's snapshot and forget it (a restore uses it once). */
export function takePeerSnapshot(key: string): PeerSnapshot | undefined {
  const snap = snapshots.get(key)
  snapshots.delete(key)
  return snap
}

/**
 * Store the key's snapshot as the newest; the oldest are dropped until both caps
 * hold. A snapshot over the whole budget is not stored: that tab gets a full replay.
 */
export function putPeerSnapshot(key: string, snap: PeerSnapshot, generation = peerSnapshotGeneration(key)): void {
  if (generation !== peerSnapshotGeneration(key)) return // the tab was closed after this panel mounted
  snapshots.delete(key)
  if (sizeOf(snap) > PEER_SNAPSHOT_MAX_BYTES) return
  snapshots.set(key, snap)
  let total = 0
  for (const s of snapshots.values()) total += sizeOf(s)
  for (const [oldest, s] of snapshots) {
    if (snapshots.size <= PEER_SNAPSHOT_MAX && total <= PEER_SNAPSHOT_MAX_BYTES) break
    snapshots.delete(oldest)
    total -= sizeOf(s)
  }
}

/** A hiding panel's save is in flight for the key; call the result once it has stored (or given up). */
export function beginPeerSave(key: string): () => void {
  let settle!: () => void
  const saved = new Promise<void>((resolve) => { settle = resolve })
  pendingSaves.set(key, saved)
  return () => {
    if (pendingSaves.get(key) === saved) pendingSaves.delete(key)
    settle()
  }
}

/** Null when no save for the key is in flight; else resolves when it settles or after PEER_SAVE_WAIT_MS. */
export function awaitPeerSave(key: string): Promise<void> | null {
  const saved = pendingSaves.get(key)
  if (!saved) return null
  return Promise.race([saved, new Promise<void>((resolve) => setTimeout(resolve, PEER_SAVE_WAIT_MS))])
}

/** The parser hooks watchPeerTerm needs (a subset of xterm's Terminal). */
interface HookableTerm {
  rows: number
  parser: Pick<Terminal['parser'], 'registerCsiHandler' | 'registerEscHandler'>
}

export interface PeerTermWatch {
  /** Feed every output byte handed to term.write, in order. */
  feed(data: Uint8Array): void
  /**
   * What serialize() leaves out, to append to its output; null when the
   * snapshot must not be taken (the delta would land in a torn or unrepresentable
   * state), so the returning tab gets a full replay instead.
   */
  suffix(): string | null
  dispose(): void
}

/**
 * WR-01: SerializeAddon restores the buffer, bracketed paste and mouse tracking,
 * but not cursor visibility (?25), the SGR mouse encodings (?1006, ?1016) or a
 * scroll region, and it cannot capture a sequence a read cut in half. The DEC
 * modes are tracked through xterm's own parser hooks (public API; xterm exposes
 * no getter for them) and re-emitted. A scroll region or an output stream that
 * ends mid-escape or mid-UTF-8 vetoes the snapshot.
 * Create it before the restored snapshot is written, so its modes are tracked too.
 */
export function watchPeerTerm(term: HookableTerm): PeerTermWatch {
  let cursorHidden = false
  let mouseEncoding = 0 // 0 (default), 1006 or 1016
  let inAlt = false
  const region = { normal: false, alt: false } // a non-full DECSTBM per buffer
  const reset = () => { cursorHidden = false; mouseEncoding = 0; inAlt = false; region.normal = region.alt = false }

  const decMode = (set: boolean) => (params: (number | number[])[]) => {
    for (const p of params) {
      if (p === 25) cursorHidden = !set
      else if (p === 1006 || p === 1016) mouseEncoding = set ? p : 0
      else if (p === 47 || p === 1047 || p === 1049) {
        inAlt = set
        if (!set) region.alt = false // xterm clears the alt buffer on leaving it
      }
    }
    return false // xterm still applies the mode
  }
  const disposables = [
    term.parser.registerCsiHandler({ prefix: '?', final: 'h' }, decMode(true)),
    term.parser.registerCsiHandler({ prefix: '?', final: 'l' }, decMode(false)),
    term.parser.registerCsiHandler({ final: 'r' }, (params) => {
      const top = Number(params[0]) || 1
      let bottom = Number(params[1]) || term.rows
      if (bottom > term.rows) bottom = term.rows
      if (bottom > top) region[inAlt ? 'alt' : 'normal'] = top > 1 || bottom < term.rows
      return false
    }),
    term.parser.registerCsiHandler({ intermediates: '!', final: 'p' }, () => { // DECSTR
      cursorHidden = false
      region[inAlt ? 'alt' : 'normal'] = false
      return false
    }),
    term.parser.registerEscHandler({ final: 'c' }, () => { reset(); return false }), // RIS
  ]

  const ground = groundTracker()
  return {
    feed: ground.feed,
    suffix() {
      if (!ground.atGround() || region.normal || region.alt) return null
      return (cursorHidden ? '\x1b[?25l' : '') + (mouseEncoding ? `\x1b[?${mouseEncoding}h` : '')
    },
    dispose() { disposables.forEach((d) => d.dispose()) },
  }
}

/**
 * Reports whether a byte stream ends in the parser's ground state: not inside an
 * ESC, CSI or string (OSC, DCS, SOS, PM, APC) sequence, nor a UTF-8 character.
 * Works on code points, as xterm's parser does, so C1 controls count.
 */
function groundTracker() {
  const GROUND = 0, ESC = 1, CSI = 2, OSC = 3, STR = 4, STR_ESC = 5
  let state = GROUND
  let need = 0 // UTF-8 continuation bytes still to come
  let cp = 0

  const step = (c: number) => {
    if (c === 0x18 || c === 0x1a) { state = GROUND; return } // CAN, SUB abort
    switch (state) {
      case GROUND:
        if (c === 0x1b) state = ESC
        else if (c === 0x9b) state = CSI
        else if (c === 0x9d) state = OSC
        else if (c === 0x90 || c === 0x98 || c === 0x9e || c === 0x9f) state = STR
        return
      case ESC:
        if (c === 0x5b) state = CSI
        else if (c === 0x5d) state = OSC
        else if (c === 0x50 || c === 0x58 || c === 0x5e || c === 0x5f) state = STR
        else if (c >= 0x30 && c <= 0x7e) state = GROUND
        // intermediates (0x20-0x2f), C0 controls and ESC itself keep ESC
        return
      case CSI:
        if (c === 0x1b) state = ESC
        else if (c >= 0x40 && c <= 0x7e) state = GROUND
        return
      case OSC:
      case STR:
        if (c === 0x9c || (state === OSC && c === 0x07)) state = GROUND
        else if (c === 0x1b) state = STR_ESC
        return
      case STR_ESC:
        if (c === 0x5c) { state = GROUND; return }
        state = ESC // the ESC ended the string and starts a new sequence
        step(c)
        return
    }
  }

  return {
    feed(data: Uint8Array) {
      for (const b of data) {
        if (need > 0 && (b & 0xc0) === 0x80) {
          cp = (cp << 6) | (b & 0x3f)
          if (--need === 0) step(cp)
          continue
        }
        need = 0 // a broken sequence decodes to U+FFFD, a printable
        if (b < 0x80) step(b)
        else if ((b & 0xe0) === 0xc0) { need = 1; cp = b & 0x1f }
        else if ((b & 0xf0) === 0xe0) { need = 2; cp = b & 0x0f }
        else if ((b & 0xf8) === 0xf0) { need = 3; cp = b & 0x07 }
      }
    },
    atGround: () => state === GROUND && need === 0,
  }
}
