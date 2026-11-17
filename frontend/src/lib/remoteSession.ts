// Phase 122-03 Task 1 — remoteSession.ts
//
// Pure-function helpers translating the RemoteSessionsPanel session model into
// the URL/base-URL shape consumed by App.tsx for the remote-session open
// path. All functions are stateless and side-effect-free; tests live in
// frontend/src/lib/__tests__/remoteSession.test.ts.

// Phase 138 Task 1 — type relocation: RemoteSession and RemotePeerSessions
// previously lived in RemoteSessionsPanel.tsx. They are defined here so all
// lib importers are decoupled from the panel file (which is deleted in Plan 04).

export interface RemoteSession {
  id: string
  name: string
  status: string
  url: string
}

export interface RemotePeerSessions {
  hostname: string
  /** MagicDNS FQDN: the unambiguous key for ConnectPeerSession (HostNames can repeat). */
  dnsName?: string
  /** Phase 130 — true when the peer responded to the metadata probe; false when unreachable. */
  reachable: boolean
  sessions: RemoteSession[]
}

export interface RemoteSessionWithHost extends RemoteSession {
  hostname: string
}

/**
 * Find a remote session by id across all peers. Returns the session enriched
 * with its peer hostname, or undefined if no peer has the id.
 */
export function findRemoteSession(
  sessionId: string,
  remotePeers: RemotePeerSessions[],
): RemoteSessionWithHost | undefined {
  for (const peer of remotePeers) {
    const match = peer.sessions.find((s) => s.id === sessionId)
    if (match) return { ...match, hostname: peer.hostname }
  }
  return undefined
}

/** True iff the session id is in any peer's remote sessions list. */
export function isRemoteSessionId(
  sessionId: string,
  remotePeers: RemotePeerSessions[],
): boolean {
  return findRemoteSession(sessionId, remotePeers) !== undefined
}
