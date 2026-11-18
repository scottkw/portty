/**
 * HARD-05: App renders only the active peer tab, so switching tabs unmounts the
 * peer's terminal and closes its socket. Keeping the terminal's content plus the
 * stream offset it reflects lets the returning tab restore itself and ask the host
 * for a delta instead of a full replay. In memory only, never persisted.
 *
 * A stale entry is still correct: a snapshot and its offset describe one stream
 * point, and the host answers a full replay when that point is gone. The cap bounds
 * entries a closed tab leaves behind.
 */
export interface PeerSnapshot {
  data: string
  cols: number
  rows: number
  offset: number
}

export const PEER_SNAPSHOT_MAX = 8

const snapshots = new Map<string, PeerSnapshot>()

/** Return the session's snapshot and forget it (a restore uses it once). */
export function takePeerSnapshot(sessionId: string): PeerSnapshot | undefined {
  const snap = snapshots.get(sessionId)
  snapshots.delete(sessionId)
  return snap
}

/** Store the session's snapshot as the newest; the oldest beyond the cap is dropped. */
export function putPeerSnapshot(sessionId: string, snap: PeerSnapshot): void {
  snapshots.delete(sessionId)
  snapshots.set(sessionId, snap)
  for (const oldest of snapshots.keys()) {
    if (snapshots.size <= PEER_SNAPSHOT_MAX) break
    snapshots.delete(oldest)
  }
}
