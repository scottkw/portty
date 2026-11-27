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

/** The last state a peer's desktop reported that leaves its sidebar row muted (ready and streaming mute nothing). */
export type DesktopRowState = 'needs_permission' | 'awaiting_approval' | 'display_asleep' | 'locked' | 'capture_paused'

/** UI-SPEC Surface 1 muted lines: the words are the browser viewer's, so the wording matches everywhere. */
export const DESKTOP_ROW_LINES: Record<DesktopRowState, string> = {
  needs_permission: 'Needs permission',
  awaiting_approval: 'Waiting for approval at the Mac',
  display_asleep: 'Display asleep',
  locked: 'Locked',
  capture_paused: 'Desktop paused',
}

/** The unique key a Desktop tab and the connect binding use: MagicDNS name, else hostname (HostNames can repeat). */
export function desktopConnectHost(peer: RemotePeerSessions): string {
  return peer.dnsName || peer.hostname
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

/** Palette size: style.css declares --color-peer-0 … --color-peer-4 in both token blocks. */
export const PEER_COLOR_COUNT = 5

/**
 * One stable colour per Mac: FNV-1a of the lower-cased full hostname, so a Mac
 * keeps its --color-peer-N across launches. Callers pass the full hostname
 * (PeerGroup.key, Tab.peerHost), so the sidebar and the tabs agree.
 */
export function peerColorIndex(hostname: string): number {
  let h = 0x811c9dc5
  for (const ch of hostname.toLowerCase()) h = Math.imul(h ^ ch.charCodeAt(0), 0x01000193)
  return (h >>> 0) % PEER_COLOR_COUNT
}

export function peerView(
  tailscaleOff: boolean,
  loaded: boolean,
  peers: RemotePeerSessions[],
  reconnectingHost: string | null = null,
): PeerView {
  if (tailscaleOff) return { kind: 'off' }
  if (!loaded) return { kind: 'loading' }
  // Discovery lists only peers that answer its probe, so a Mac whose open tab is
  // reconnecting would drop out; keep it, unreachable, so its row can show amber.
  const listed =
    reconnectingHost && !peers.some((p) => p.hostname === reconnectingHost)
      ? [...peers, { hostname: reconnectingHost, reachable: false, desktop: false, sessions: [] }]
      : peers
  const groups = listed
    // Every probed peer is kept: an unreachable one shows the can't-reach line, a reachable one
    // with nothing shared shows "Nothing shared". Quick 261116-c9f reverses the UI-SPEC "Hidden"
    // row at the user's request.
    .map((p) => ({ key: p.hostname, host: shortHost(p.hostname), reachable: p.reachable, peer: p }))
    .sort((a, b) => a.host.localeCompare(b.host) || a.key.localeCompare(b.key))
  return groups.length === 0 ? { kind: 'none' } : { kind: 'groups', groups }
}
