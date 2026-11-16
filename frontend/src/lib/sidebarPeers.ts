// Portty 02-04 (NAV-02): pure derivation of the sidebar's peer area from the
// Tailscale state and the remote-peer poll (UI-SPEC "Peer group states").
import type { RemotePeerSessions } from './remoteSession'

export interface PeerGroup {
  /** Full hostname: unique per peer, used as the React key. */
  key: string
  /** Short hostname (first DNS label), shown on the peer's host row. */
  host: string
  reachable: boolean
  peer: RemotePeerSessions
}

export type PeerView =
  | { kind: 'off' }
  | { kind: 'loading' }
  | { kind: 'none' }
  | { kind: 'groups'; groups: PeerGroup[] }

/** First DNS label: "office-air.tail1234.ts.net" -> "office-air". */
export function shortHost(hostname: string): string {
  return hostname.split('.')[0]
}

export function peerView(tailscaleOff: boolean, loaded: boolean, peers: RemotePeerSessions[]): PeerView {
  if (tailscaleOff) return { kind: 'off' }
  if (!loaded) return { kind: 'loading' }
  const groups = peers
    // Every probed peer is kept: an unreachable one shows the can't-reach line, a reachable one
    // with nothing shared shows "Nothing shared". Quick 261116-c9f reverses the UI-SPEC "Hidden"
    // row at the user's request.
    .map((p) => ({ key: p.hostname, host: shortHost(p.hostname), reachable: p.reachable, peer: p }))
    .sort((a, b) => a.host.localeCompare(b.host) || a.key.localeCompare(b.key))
  return groups.length === 0 ? { kind: 'none' } : { kind: 'groups', groups }
}
